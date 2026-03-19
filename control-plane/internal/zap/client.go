package zap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var (
	ErrNotConnected = errors.New("zap: client not connected")
	ErrClosed       = errors.New("zap: client closed")
)

// Client is a WebSocket JSON-RPC client for communicating with a dev sidecar.
type Client struct {
	wsURL string

	mu   sync.Mutex
	conn *websocket.Conn

	nextID atomic.Int64

	// events is the channel consumers read server notifications from.
	events chan EventMsg

	// pending tracks in-flight requests waiting for a response.
	pendingMu sync.Mutex
	pending   map[string]chan *JSONRPCMessage

	done chan struct{}
}

// NewClient creates a new client targeting the given WebSocket URL.
func NewClient(wsURL string) *Client {
	return &Client{
		wsURL:   wsURL,
		events:  make(chan EventMsg, 256),
		pending: make(map[string]chan *JSONRPCMessage),
		done:    make(chan struct{}),
	}
}

// Connect establishes the WebSocket connection to the sidecar.
func (c *Client) Connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	header := http.Header{}
	conn, _, err := dialer.DialContext(ctx, c.wsURL, header)
	if err != nil {
		return fmt.Errorf("zap: dial %s: %w", c.wsURL, err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	go c.readLoop()
	return nil
}

// Initialize sends the initialize request and waits for the response.
func (c *Client) Initialize(params InitializeParams) (*InitializeResponse, error) {
	raw, err := c.request("initialize", params)
	if err != nil {
		return nil, fmt.Errorf("zap: initialize: %w", err)
	}

	var resp InitializeResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("zap: unmarshal initialize response: %w", err)
	}

	// Send the "initialized" notification per protocol.
	if err := c.notify("initialized", nil); err != nil {
		return nil, fmt.Errorf("zap: initialized notification: %w", err)
	}

	return &resp, nil
}

// SendSubmission sends a Submission (op) to the sidecar via a JSON-RPC
// notification. The sidecar protocol uses notifications for submissions
// rather than request/response because events flow back asynchronously.
func (c *Client) SendSubmission(sub Submission) error {
	return c.notify("submission", sub)
}

// Events returns a read-only channel of server events/notifications.
func (c *Client) Events() <-chan EventMsg {
	return c.events
}

// Close gracefully shuts down the client connection.
func (c *Client) Close() error {
	select {
	case <-c.done:
		return ErrClosed
	default:
	}
	close(c.done)

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn != nil {
		// Send close message to peer.
		_ = conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		return conn.Close()
	}
	return nil
}

// request sends a JSON-RPC request and waits for the response.
func (c *Client) request(method string, params interface{}) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	idStr := fmt.Sprintf("%d", id)

	var rawParams *json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		raw := json.RawMessage(b)
		rawParams = &raw
	}

	msg := struct {
		ID     int64            `json:"id"`
		Method string           `json:"method"`
		Params *json.RawMessage `json:"params,omitempty"`
	}{
		ID:     id,
		Method: method,
		Params: rawParams,
	}

	respCh := make(chan *JSONRPCMessage, 1)
	c.pendingMu.Lock()
	c.pending[idStr] = respCh
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, idStr)
		c.pendingMu.Unlock()
	}()

	if err := c.writeJSON(msg); err != nil {
		return nil, err
	}

	select {
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, fmt.Errorf("zap: rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		if resp.Result != nil {
			return *resp.Result, nil
		}
		return json.RawMessage("{}"), nil
	case <-c.done:
		return nil, ErrClosed
	}
}

// notify sends a JSON-RPC notification (no response expected).
func (c *Client) notify(method string, params interface{}) error {
	var rawParams *json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		raw := json.RawMessage(b)
		rawParams = &raw
	}

	msg := struct {
		Method string           `json:"method"`
		Params *json.RawMessage `json:"params,omitempty"`
	}{
		Method: method,
		Params: rawParams,
	}

	return c.writeJSON(msg)
}

func (c *Client) writeJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return ErrNotConnected
	}
	return c.conn.WriteJSON(v)
}

func (c *Client) readLoop() {
	defer close(c.events)

	for {
		select {
		case <-c.done:
			return
		default:
		}

		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()

		if conn == nil {
			return
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			select {
			case <-c.done:
			default:
				// Connection lost unexpectedly.
			}
			return
		}

		var msg JSONRPCMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		// Route responses to pending request channels.
		if msg.ID != nil && (msg.Result != nil || msg.Error != nil) {
			var idStr string
			if err := json.Unmarshal(*msg.ID, &idStr); err != nil {
				// Try as integer.
				var idInt int64
				if err := json.Unmarshal(*msg.ID, &idInt); err == nil {
					idStr = fmt.Sprintf("%d", idInt)
				}
			}

			c.pendingMu.Lock()
			ch, ok := c.pending[idStr]
			c.pendingMu.Unlock()

			if ok {
				ch <- &msg
				continue
			}
		}

		// Server notifications carry events in params.
		if msg.Method != "" && msg.Params != nil {
			var evt EventMsg
			if err := json.Unmarshal(*msg.Params, &evt); err == nil && evt.Type != "" {
				select {
				case c.events <- evt:
				default:
					// Drop if consumer is not keeping up.
				}
				continue
			}

			// Some notifications encode the event differently (e.g.
			// flat params without a "type" field). Wrap with the method
			// as type so consumers can still identify them.
			evt = EventMsg{Type: msg.Method, Raw: *msg.Params}
			select {
			case c.events <- evt:
			default:
			}
		}
	}
}
