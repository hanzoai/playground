package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client talks to the Commerce API for balance lookups, holds, and usage recording.
type Client struct {
	baseURL      string
	serviceToken string
	httpClient   *http.Client
}

// BalanceResult represents the response from the Commerce balance endpoint.
type BalanceResult struct {
	User      string  `json:"user"`
	Balance   float64 `json:"balance"`
	Currency  string  `json:"currency"`
	Available float64 `json:"available"`
	Holds     float64 `json:"holds,omitempty"`
}

// Hold represents a billing hold (fund reservation) in Commerce.
type Hold struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	AmountCents int       `json:"amount_cents"`
	Status      string    `json:"status"` // "pending", "settled", "released"
	CreatedAt   time.Time `json:"created_at"`
}

// NewClient creates a billing client using COMMERCE_API_URL env
// or defaulting to https://commerce.hanzo.ai.
func NewClient() *Client {
	base := os.Getenv("COMMERCE_API_URL")
	if base == "" {
		base = "https://commerce.hanzo.ai"
	}
	base = strings.TrimRight(base, "/")
	return &Client{
		baseURL:      base,
		serviceToken: os.Getenv("COMMERCE_SERVICE_TOKEN"),
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// setAuthHeaders sets Authorization and X-Hanzo-Org headers using the same
// precedence logic as GetBalance: prefer service token, fall back to user token.
func (c *Client) setAuthHeaders(req *http.Request, userID, token string) {
	authToken := c.serviceToken
	if authToken == "" {
		authToken = token
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	if c.serviceToken != "" {
		if parts := strings.SplitN(userID, "/", 2); len(parts) == 2 {
			req.Header.Set("X-Hanzo-Org", parts[0])
		}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
}

// GetBalance returns the user's available balance in cents.
func (c *Client) GetBalance(ctx context.Context, userID, token string) (*BalanceResult, error) {
	// Normalize user ID to lowercase — Commerce stores all IDs lowercase.
	userID = strings.ToLower(userID)
	u := fmt.Sprintf("%s/api/v1/billing/balance?user=%s&currency=usd",
		c.baseURL, url.QueryEscape(userID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("billing: create request: %w", err)
	}
	// Prefer service token for inter-service calls; fall back to user token.
	authToken := c.serviceToken
	if authToken == "" {
		authToken = token
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	// Set org header for service-token auth (org derived from user ID "org/name").
	if c.serviceToken != "" {
		if parts := strings.SplitN(userID, "/", 2); len(parts) == 2 {
			req.Header.Set("X-Hanzo-Org", parts[0])
		}
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("billing: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("billing: unexpected status %d", resp.StatusCode)
	}

	var result BalanceResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("billing: decode response: %w", err)
	}
	return &result, nil
}

// CreateHold reserves funds on a user's account. The hold prevents the reserved
// amount from being spent elsewhere until it is settled or released.
// Commerce endpoint: POST /api/v1/billing/holds
func (c *Client) CreateHold(ctx context.Context, userID, token string, amountCents int, description string) (*Hold, error) {
	userID = strings.ToLower(userID)

	body, err := json.Marshal(map[string]interface{}{
		"user_id":      userID,
		"amount_cents": amountCents,
		"currency":     "usd",
		"description":  description,
	})
	if err != nil {
		return nil, fmt.Errorf("billing: marshal hold request: %w", err)
	}

	u := fmt.Sprintf("%s/api/v1/billing/holds", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("billing: create hold request: %w", err)
	}
	c.setAuthHeaders(req, userID, token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("billing: hold request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("billing: read hold response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("billing: create hold returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var hold Hold
	if err := json.Unmarshal(respBody, &hold); err != nil {
		return nil, fmt.Errorf("billing: decode hold response: %w", err)
	}
	return &hold, nil
}

// SettleHold finalizes a hold with the actual usage amount. If actualCents is less
// than the held amount, the difference is released back to the user's balance.
// Commerce endpoint: POST /api/v1/billing/holds/{holdID}/settle
func (c *Client) SettleHold(ctx context.Context, holdID, token string, actualCents int) error {
	body, err := json.Marshal(map[string]interface{}{
		"actual_cents": actualCents,
	})
	if err != nil {
		return fmt.Errorf("billing: marshal settle request: %w", err)
	}

	u := fmt.Sprintf("%s/api/v1/billing/holds/%s/settle", c.baseURL, url.PathEscape(holdID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("billing: create settle request: %w", err)
	}
	c.setAuthHeaders(req, "", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("billing: settle request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("billing: settle hold returned status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// RecordUsage posts a usage event to Commerce for metering and invoicing.
// Commerce endpoint: POST /api/v1/billing/usage
func (c *Client) RecordUsage(ctx context.Context, userID, token string, centsUsed int, metadata map[string]string) error {
	userID = strings.ToLower(userID)

	payload := map[string]interface{}{
		"user_id":    userID,
		"cents_used": centsUsed,
		"currency":   "usd",
	}
	if len(metadata) > 0 {
		payload["metadata"] = metadata
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("billing: marshal usage request: %w", err)
	}

	u := fmt.Sprintf("%s/api/v1/billing/usage", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("billing: create usage request: %w", err)
	}
	c.setAuthHeaders(req, userID, token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("billing: usage request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("billing: record usage returned status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
