package kms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// SecretsClient manages org-scoped encrypted secrets via the Hanzo KMS API
// (Infisical-compatible v3 secret endpoints).
type SecretsClient struct {
	baseURL     string
	token       string
	environment string
	httpClient  *http.Client
}

// NewSecretsClient creates a secrets client for the given KMS endpoint.
// environment defaults to "prod" if empty.
func NewSecretsClient(baseURL, token, environment string) *SecretsClient {
	if environment == "" {
		environment = "prod"
	}
	return &SecretsClient{
		baseURL:     baseURL,
		token:       token,
		environment: environment,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// StoreSecret creates or updates an encrypted secret in the given org's project.
// orgID maps to the KMS workspace/project scope.
func (s *SecretsClient) StoreSecret(ctx context.Context, orgID, key string, value []byte) error {
	reqURL := fmt.Sprintf("%s/api/v3/secrets/raw/%s", s.baseURL, url.PathEscape(key))

	body, err := json.Marshal(map[string]interface{}{
		"workspaceId":   orgID,
		"environment":   s.environment,
		"secretPath":    "/",
		"secretValue":   string(value),
		"secretComment": "",
		"type":          "shared",
	})
	if err != nil {
		return fmt.Errorf("kms secrets: marshal store request: %w", err)
	}

	resp, err := s.doJSON(ctx, http.MethodPost, reqURL, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 200 = updated existing, 201 = created new
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return s.readError(resp, "store", key)
	}
	return nil
}

// GetSecret retrieves a secret value by key, scoped to the given org.
// Returns a nil slice and nil error when the key does not exist.
func (s *SecretsClient) GetSecret(ctx context.Context, orgID, key string) ([]byte, error) {
	reqURL := fmt.Sprintf("%s/api/v3/secrets/raw/%s", s.baseURL, url.PathEscape(key))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("kms secrets: create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("workspaceId", orgID)
	q.Set("environment", s.environment)
	q.Set("secretPath", "/")
	req.URL.RawQuery = q.Encode()

	s.setAuth(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kms secrets: request for %q: %w", key, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, s.readError(resp, "get", key)
	}

	var result struct {
		Secret struct {
			SecretValue string `json:"secretValue"`
		} `json:"secret"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("kms secrets: decode response for %q: %w", key, err)
	}
	return []byte(result.Secret.SecretValue), nil
}

// DeleteSecret removes a secret by key, scoped to the given org.
func (s *SecretsClient) DeleteSecret(ctx context.Context, orgID, key string) error {
	reqURL := fmt.Sprintf("%s/api/v3/secrets/raw/%s", s.baseURL, url.PathEscape(key))

	body, err := json.Marshal(map[string]string{
		"workspaceId": orgID,
		"environment": s.environment,
		"secretPath":  "/",
		"type":        "shared",
	})
	if err != nil {
		return fmt.Errorf("kms secrets: marshal delete request: %w", err)
	}

	resp, err := s.doJSON(ctx, http.MethodDelete, reqURL, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return s.readError(resp, "delete", key)
	}
	return nil
}

// ListSecrets returns metadata for all secrets in the given org scope.
func (s *SecretsClient) ListSecrets(ctx context.Context, orgID string) ([]SecretMetadata, error) {
	reqURL := fmt.Sprintf("%s/api/v3/secrets/raw", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("kms secrets: create list request: %w", err)
	}

	q := req.URL.Query()
	q.Set("workspaceId", orgID)
	q.Set("environment", s.environment)
	q.Set("secretPath", "/")
	req.URL.RawQuery = q.Encode()

	s.setAuth(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kms secrets: list request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, s.readError(resp, "list", "")
	}

	var result struct {
		Secrets []struct {
			SecretKey string    `json:"secretKey"`
			Version   int       `json:"version"`
			CreatedAt time.Time `json:"createdAt"`
			UpdatedAt time.Time `json:"updatedAt"`
		} `json:"secrets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("kms secrets: decode list response: %w", err)
	}

	metadata := make([]SecretMetadata, len(result.Secrets))
	for i, sec := range result.Secrets {
		metadata[i] = SecretMetadata{
			Key:       sec.SecretKey,
			Version:   sec.Version,
			CreatedAt: sec.CreatedAt,
			UpdatedAt: sec.UpdatedAt,
		}
	}
	return metadata, nil
}

// setAuth adds the bearer token to the request.
func (s *SecretsClient) setAuth(req *http.Request) {
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
}

// doJSON executes an HTTP request with JSON body, authorization, and content-type headers.
func (s *SecretsClient) doJSON(ctx context.Context, method, reqURL string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, reader)
	if err != nil {
		return nil, fmt.Errorf("kms secrets: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	s.setAuth(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kms secrets: request to %s: %w", reqURL, err)
	}
	return resp, nil
}

// readError extracts an error from a non-success HTTP response.
func (s *SecretsClient) readError(resp *http.Response, op, key string) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var errResp struct {
		Error string `json:"error"`
	}
	msg := string(body)
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
		msg = errResp.Error
	}
	if key != "" {
		return fmt.Errorf("kms secrets: %s %q: %d %s", op, key, resp.StatusCode, msg)
	}
	return fmt.Errorf("kms secrets: %s: %d %s", op, resp.StatusCode, msg)
}
