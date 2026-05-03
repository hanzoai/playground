package zap

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

// SidecarOpts configures how a sidecar process is spawned.
type SidecarOpts struct {
	// BinaryPath is the path to the dev binary.
	// If empty, "hanzo-dev" is looked up in PATH.
	BinaryPath string

	// SpaceID associates this sidecar with a playground space.
	SpaceID string

	// BotID is the unique identifier for the bot this sidecar serves.
	BotID string

	// Cwd is the working directory for the sidecar process.
	Cwd string

	// Model is the model slug to use (e.g. "o4-mini").
	Model string

	// ApprovalPolicy controls command approval behavior.
	ApprovalPolicy AskForApproval

	// Sandbox controls execution restrictions.
	Sandbox SandboxPolicy

	// Env is additional environment variables for the process.
	Env []string

	// ClientName identifies this client to the sidecar.
	ClientName string

	// ClientVersion identifies this client version.
	ClientVersion string
}

// Sidecar manages the lifecycle of a dev sidecar process.
type Sidecar struct {
	mu   sync.Mutex
	cmd  *exec.Cmd
	port int

	client *Client
	opts   SidecarOpts

	cancel context.CancelFunc
}

// SpawnSidecar starts a dev sidecar process and connects to it.
func SpawnSidecar(ctx context.Context, opts SidecarOpts) (*Sidecar, error) {
	binary := opts.BinaryPath
	if binary == "" {
		binary = "hanzo-dev"
	}

	// Find a free port for the sidecar WebSocket server.
	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("zap: find free port: %w", err)
	}

	procCtx, cancel := context.WithCancel(ctx)

	args := []string{
		"--port", fmt.Sprintf("%d", port),
	}

	cmd := exec.CommandContext(procCtx, binary, args...)
	cmd.Dir = opts.Cwd
	cmd.Stdout = os.Stderr // Log sidecar output to stderr.
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), opts.Env...)

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("zap: start sidecar %s: %w", binary, err)
	}

	wsURL := fmt.Sprintf("ws://127.0.0.1:%d", port)
	client := NewClient(wsURL)

	s := &Sidecar{
		cmd:    cmd,
		port:   port,
		client: client,
		opts:   opts,
		cancel: cancel,
	}

	// Wait for the sidecar to start accepting connections.
	if err := s.waitReady(ctx); err != nil {
		_ = s.Stop(ctx)
		return nil, err
	}

	// Connect and initialize.
	if err := client.Connect(ctx); err != nil {
		_ = s.Stop(ctx)
		return nil, fmt.Errorf("zap: connect to sidecar: %w", err)
	}

	clientName := opts.ClientName
	if clientName == "" {
		clientName = "playground"
	}
	clientVersion := opts.ClientVersion
	if clientVersion == "" {
		clientVersion = "0.1.0"
	}

	_, err = client.Initialize(InitializeParams{
		ClientInfo: ClientInfo{
			Name:    clientName,
			Version: clientVersion,
		},
	})
	if err != nil {
		_ = s.Stop(ctx)
		return nil, fmt.Errorf("zap: initialize sidecar: %w", err)
	}

	return s, nil
}

// Client returns the JSON-RPC client connected to this sidecar.
func (s *Sidecar) Client() *Client {
	return s.client
}

// Port returns the port the sidecar is listening on.
func (s *Sidecar) Port() int {
	return s.port
}

// Opts returns the options this sidecar was spawned with.
func (s *Sidecar) Opts() SidecarOpts {
	return s.opts
}

// Stop gracefully shuts down the sidecar process and closes the client.
func (s *Sidecar) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error

	if s.client != nil {
		if err := s.client.Close(); err != nil && err != ErrClosed {
			errs = append(errs, err)
		}
	}

	if s.cancel != nil {
		s.cancel()
	}

	if s.cmd != nil && s.cmd.Process != nil {
		// Give the process a moment to exit gracefully.
		done := make(chan error, 1)
		go func() { done <- s.cmd.Wait() }()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = s.cmd.Process.Kill()
			<-done
		case <-ctx.Done():
			_ = s.cmd.Process.Kill()
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// waitReady polls the sidecar port until it accepts TCP connections.
func (s *Sidecar) waitReady(ctx context.Context) error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	deadline := time.Now().Add(15 * time.Second)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("zap: sidecar did not become ready on port %d within 15s", s.port)
}

// freePort asks the OS for an available TCP port.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}
