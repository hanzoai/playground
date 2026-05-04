package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// NewNodesCommand groups node management subcommands.
func NewNodesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "Manage hanzo nodes",
	}

	cmd.AddCommand(newRegisterServerlessCommand())
	return cmd
}

type registerServerlessOptions struct {
	invocationURL string
	serverURL     string
	token         string
	timeout       time.Duration
	jsonOutput    bool
}

// envFallback returns the value of the primary env var, falling back to the
// secondary env var when the primary is empty.
func envFallback(primary, fallback string) string {
	if v := os.Getenv(primary); v != "" {
		return v
	}
	return os.Getenv(fallback)
}

func newRegisterServerlessCommand() *cobra.Command {
	opts := &registerServerlessOptions{
		serverURL: envFallback("PLAYGROUND_SERVER", "AGENTS_SERVER"),
		token:     envFallback("PLAYGROUND_TOKEN", "AGENTS_TOKEN"),
		timeout:   15 * time.Second,
	}

	cmd := &cobra.Command{
		Use:   "register-serverless --url <invocation-url>",
		Short: "Register a serverless bot by its invocation URL (Lambda/Cloud Functions/Cloud Run)",
		Long:  "Registers a serverless bot with the control plane by calling its /discover endpoint and storing the invocation URL for on-demand execution.",
		RunE: func(_ *cobra.Command, _ []string) error {
			if opts.invocationURL == "" {
				return fmt.Errorf("--url is required")
			}

			server := strings.TrimSpace(opts.serverURL)
			if server == "" {
				server = "http://localhost:8080"
			}
			server = strings.TrimSuffix(server, "/")

			payload := map[string]string{
				"invocation_url": opts.invocationURL,
			}
			body, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("encode payload: %w", err)
			}

			client := &http.Client{Timeout: opts.timeout}
			req, err := http.NewRequest(http.MethodPost, server+"/api/v1/nodes/register-serverless", bytes.NewReader(body))
			if err != nil {
				return fmt.Errorf("build request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")
			if opts.token != "" {
				req.Header.Set("Authorization", "Bearer "+opts.token)
			}

			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("request failed: %w", err)
			}
			defer resp.Body.Close()

			var parsed map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if resp.StatusCode >= 300 {
				return fmt.Errorf("registration failed (%d): %v", resp.StatusCode, parsed)
			}

			if opts.jsonOutput {
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(parsed)
			}

			nodeID := ""
			if node, ok := parsed["node"].(map[string]any); ok {
				if id, ok := node["id"].(string); ok {
					nodeID = id
				}
			}

			if nodeID != "" {
				fmt.Printf("Registered serverless bot: %s\n", nodeID)
			} else {
				fmt.Println("Registered serverless bot")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.invocationURL, "url", "", "Invocation URL for the serverless function (required)")
	cmd.Flags().StringVar(&opts.serverURL, "server", opts.serverURL, "Control plane URL (default: http://localhost:8080 or $PLAYGROUND_SERVER)")
	cmd.Flags().StringVar(&opts.token, "token", opts.token, "Bearer token for the control plane (default: $PLAYGROUND_TOKEN)")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", opts.timeout, "HTTP timeout for discovery/registration")
	cmd.Flags().BoolVar(&opts.jsonOutput, "json", false, "Print raw JSON response")

	return cmd
}
