// Package cloud — Visor client for multi-cloud VM provisioning.
// Visor manages VMs across AWS EC2, DigitalOcean, GCP, Azure, Proxmox, etc.
// and provides remote access (RDP/VNC/SSH) via Guacamole tunnels.
package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hanzoai/playground/control-plane/internal/config"
)

// DesktopOS represents the operating system for a VM.
type DesktopOS string

const (
	OSLinux   DesktopOS = "linux"
	OSMacOS   DesktopOS = "macos"
	OSWindows DesktopOS = "windows"
)

// ProtocolForOS returns the remote access protocol for a given OS.
func ProtocolForOS(os DesktopOS) string {
	switch os {
	case OSMacOS:
		return "vnc"
	case OSWindows:
		return "rdp"
	default:
		return "ssh"
	}
}

// VisorMachine represents a VM managed by Visor.
type VisorMachine struct {
	Name     string `json:"name"`
	OS       string `json:"os"`
	State    string `json:"state"`
	PublicIP string `json:"public_ip"`
	Provider string `json:"provider"`
}

// VisorClient communicates with the Visor API for VM provisioning.
type VisorClient struct {
	endpoint   string
	clientID   string
	clientSecret string
	httpClient *http.Client
}

// NewVisorClient creates a new Visor API client.
func NewVisorClient(cfg config.VisorConfig) *VisorClient {
	return &VisorClient{
		endpoint:     strings.TrimRight(cfg.Endpoint, "/"),
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ListMachines returns VMs for the given organization.
func (c *VisorClient) ListMachines(ctx context.Context, org string) ([]VisorMachine, error) {
	url := fmt.Sprintf("%s/api/orgs/%s/machines", c.endpoint, org)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("visor: create request: %w", err)
	}

	if c.clientID != "" {
		req.Header.Set("X-Client-ID", c.clientID)
	}
	if c.clientSecret != "" {
		req.Header.Set("Authorization", "Bearer "+c.clientSecret)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("visor: list machines: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("visor: list machines returned %d", resp.StatusCode)
	}

	var machines []VisorMachine
	if err := json.NewDecoder(resp.Body).Decode(&machines); err != nil {
		return nil, fmt.Errorf("visor: decode response: %w", err)
	}

	return machines, nil
}
