package kms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client communicates with the Lux KMS MPC daemon HTTP API.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewClient creates an MPC client pointing at the given base URL.
// The token is sent as a Bearer authorization header on every request.
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Keygen triggers distributed key generation via the MPC daemon.
// vaultID identifies the vault that will hold the new wallet.
func (c *Client) Keygen(ctx context.Context, vaultID, name, keyType, protocol string) (*Wallet, error) {
	url := fmt.Sprintf("%s/api/v1/vaults/%s/wallets", c.BaseURL, vaultID)

	body, err := json.Marshal(KeygenRequest{
		Name:     name,
		KeyType:  keyType,
		Protocol: protocol,
	})
	if err != nil {
		return nil, fmt.Errorf("kms: marshal keygen request: %w", err)
	}

	resp, err := c.do(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, readError(resp)
	}

	var wallet Wallet
	if err := json.NewDecoder(resp.Body).Decode(&wallet); err != nil {
		return nil, fmt.Errorf("kms: decode keygen response: %w", err)
	}
	return &wallet, nil
}

// Sign triggers threshold signing via the MPC daemon.
func (c *Client) Sign(ctx context.Context, walletID, keyType string, message []byte) (*SignResult, error) {
	url := fmt.Sprintf("%s/api/v1/transactions", c.BaseURL)

	body, err := json.Marshal(map[string]interface{}{
		"wallet_id": walletID,
		"key_type":  keyType,
		"payload":   message,
		"type":      "sign",
	})
	if err != nil {
		return nil, fmt.Errorf("kms: marshal sign request: %w", err)
	}

	resp, err := c.do(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, readError(resp)
	}

	var result SignResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("kms: decode sign response: %w", err)
	}
	return &result, nil
}

// Reshare triggers key resharing to change threshold or participants.
func (c *Client) Reshare(ctx context.Context, walletID string, newThreshold int, newParticipants []string) error {
	url := fmt.Sprintf("%s/api/v1/wallets/%s/reshare", c.BaseURL, walletID)

	body, err := json.Marshal(ReshareRequest{
		NewThreshold:    newThreshold,
		NewParticipants: newParticipants,
	})
	if err != nil {
		return fmt.Errorf("kms: marshal reshare request: %w", err)
	}

	resp, err := c.do(ctx, http.MethodPost, url, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return readError(resp)
	}
	return nil
}

// GetWallet retrieves wallet metadata from the MPC daemon.
func (c *Client) GetWallet(ctx context.Context, walletID string) (*Wallet, error) {
	url := fmt.Sprintf("%s/api/v1/wallets/%s", c.BaseURL, walletID)

	resp, err := c.do(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, readError(resp)
	}

	var wallet Wallet
	if err := json.NewDecoder(resp.Body).Decode(&wallet); err != nil {
		return nil, fmt.Errorf("kms: decode wallet response: %w", err)
	}
	return &wallet, nil
}

// Status returns the MPC cluster status.
func (c *Client) Status(ctx context.Context) (*ClusterStatus, error) {
	url := fmt.Sprintf("%s/api/v1/status", c.BaseURL)

	resp, err := c.do(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, readError(resp)
	}

	var status ClusterStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("kms: decode status response: %w", err)
	}
	return &status, nil
}

// do executes an HTTP request with authorization and content-type headers.
func (c *Client) do(ctx context.Context, method, url string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("kms: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kms: request to %s: %w", url, err)
	}
	return resp, nil
}

// readError extracts an APIError from a non-success HTTP response.
func readError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var errResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
		return &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
	}
	return &APIError{StatusCode: resp.StatusCode, Message: string(body)}
}
