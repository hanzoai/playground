package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client talks to the Commerce API for balance lookups.
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
