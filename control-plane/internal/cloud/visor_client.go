// Package cloud provides cloud agent provisioning on Kubernetes and multi-cloud VMs.
// The VisorClient bridges to Hanzo Visor (CasVisor) for managing VMs across
// AWS EC2, DigitalOcean, GCP, Azure, Proxmox, and other providers.
// Visor provides remote access (RDP/VNC/SSH) via Apache Guacamole tunnels.
package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hanzoai/playground/control-plane/internal/config"
	"github.com/hanzoai/playground/control-plane/internal/logger"
)

// DesktopOS represents the target operating system for a desktop VM.
type DesktopOS string

const (
	OSLinux    DesktopOS = "linux"
	OSMacOS    DesktopOS = "macos"
	OSWindows  DesktopOS = "windows"
	OSTerminal DesktopOS = "terminal" // Terminal-only mode: no desktop, just xterm/ttyd
)

// VisorMachine represents a VM managed by Visor.
type VisorMachine struct {
	Owner          string `json:"owner"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Provider       string `json:"provider"`       // Visor provider name
	OS             string `json:"os"`             // "Linux", "macOS", "Windows"
	Region         string `json:"region"`
	State          string `json:"state"`          // "Running", "Stopped", "Pending"
	PublicIP       string `json:"publicIp"`
	PrivateIP      string `json:"privateIp"`
	RemoteProtocol string `json:"remoteProtocol"` // "RDP", "SSH", "VNC"
	RemotePort     int    `json:"remotePort"`
	RemoteUsername string `json:"remoteUsername"`
	CreatedTime    string `json:"createdTime"`
}

// VisorSession represents a remote access session via Guacamole.
type VisorSession struct {
	Owner        string `json:"owner"`
	Name         string `json:"name"`
	Protocol     string `json:"protocol"` // "rdp", "ssh", "vnc"
	ConnectionID string `json:"connectionId"`
	Status       string `json:"status"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

// VisorProvider represents a cloud provider configured in Visor.
type VisorProvider struct {
	Owner    string `json:"owner"`
	Name     string `json:"name"`
	Category string `json:"category"` // "Public Cloud", "Private Cloud"
	Type     string `json:"type"`     // "AWS", "DigitalOcean", "GCP", "Azure", "Proxmox"
	Region   string `json:"region"`
	State    string `json:"state"` // "Active", "Inactive"
}

// VMProvisionRequest describes a VM to provision via a cloud provider.
type VMProvisionRequest struct {
	NodeID       string            `json:"node_id"`
	DisplayName  string            `json:"display_name"`
	OS           DesktopOS         `json:"os"`                       // "linux", "macos", "windows"
	Provider     string            `json:"provider"`                 // Visor provider name (e.g. "aws-us-east-1")
	Region       string            `json:"region,omitempty"`
	InstanceType string            `json:"instance_type,omitempty"`  // e.g. "t3.medium", "mac2.metal"
	Owner        string            `json:"owner,omitempty"`
	Org          string            `json:"org,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`           // env vars passed to cloud-init (prefix "env:")
	SSHKeyIDs    []string          `json:"ssh_key_ids,omitempty"`    // Provider SSH key IDs
}

// VMProvisionResult describes the outcome of VM provisioning.
type VMProvisionResult struct {
	NodeID         string    `json:"node_id"`
	MachineName    string    `json:"machine_name"`
	OS             DesktopOS `json:"os"`
	Provider       string    `json:"provider"`
	State          string    `json:"state"`
	RemoteProtocol string    `json:"remote_protocol"` // "rdp", "ssh", "vnc"
	RemoteURL      string    `json:"remote_url"`      // Visor tunnel URL
	CreatedAt      time.Time `json:"created_at"`
}

// VisorClient talks to the Visor API for multi-cloud VM management.
type VisorClient struct {
	config config.VisorConfig
	client *http.Client
}

// NewVisorClient creates a new Visor API client.
func NewVisorClient(cfg config.VisorConfig) *VisorClient {
	return &VisorClient{
		config: cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// ListProviders returns all cloud providers configured in Visor.
func (vc *VisorClient) ListProviders(ctx context.Context, owner string) ([]VisorProvider, error) {
	url := fmt.Sprintf("%s/api/get-providers?owner=%s%s", vc.config.Endpoint, owner, vc.authQuery())

	resp, err := vc.doGet(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("visor list providers: %w", err)
	}

	var result struct {
		Data []VisorProvider `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		// Try direct array decode
		var providers []VisorProvider
		if err2 := json.Unmarshal(resp, &providers); err2 != nil {
			return nil, fmt.Errorf("visor decode providers: %w", err)
		}
		return providers, nil
	}
	return result.Data, nil
}

// ListMachines returns all VMs from a specific owner, syncing from cloud providers.
func (vc *VisorClient) ListMachines(ctx context.Context, owner string) ([]VisorMachine, error) {
	url := fmt.Sprintf("%s/api/get-machines?owner=%s%s", vc.config.Endpoint, owner, vc.authQuery())

	resp, err := vc.doGet(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("visor list machines: %w", err)
	}

	var result struct {
		Data []VisorMachine `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		var machines []VisorMachine
		if err2 := json.Unmarshal(resp, &machines); err2 != nil {
			return nil, fmt.Errorf("visor decode machines: %w", err)
		}
		return machines, nil
	}
	return result.Data, nil
}

// GetMachine returns details of a specific VM.
func (vc *VisorClient) GetMachine(ctx context.Context, owner, name string) (*VisorMachine, error) {
	url := fmt.Sprintf("%s/api/get-machine?id=%s/%s%s", vc.config.Endpoint, owner, name, vc.authQuery())

	resp, err := vc.doGet(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("visor get machine: %w", err)
	}

	var machine VisorMachine
	if err := json.Unmarshal(resp, &machine); err != nil {
		return nil, fmt.Errorf("visor decode machine: %w", err)
	}
	return &machine, nil
}

// UpdateMachineState starts or stops a VM.
func (vc *VisorClient) UpdateMachineState(ctx context.Context, owner, name, state string) error {
	machine := map[string]interface{}{
		"owner": owner,
		"name":  name,
		"state": state,
	}

	url := fmt.Sprintf("%s/api/update-machine%s", vc.config.Endpoint, vc.authQueryFirst())
	_, err := vc.doPost(ctx, url, machine)
	return err
}

// CreateTunnel creates a Guacamole remote access tunnel to a VM.
// Returns the session ID for WebSocket connection.
func (vc *VisorClient) CreateTunnel(ctx context.Context, owner, machineName, mode string) (*VisorSession, error) {
	assetID := fmt.Sprintf("%s/%s", owner, machineName)
	url := fmt.Sprintf("%s/api/add-asset-tunnel?assetId=%s&mode=%s%s",
		vc.config.Endpoint, assetID, mode, vc.authQuery())

	resp, err := vc.doPost(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("visor create tunnel: %w", err)
	}

	var session VisorSession
	if err := json.Unmarshal(resp, &session); err != nil {
		return nil, fmt.Errorf("visor decode session: %w", err)
	}
	return &session, nil
}

// CreateMachine launches a new cloud VM via Visor's cloud provider integration.
// Visor delegates to the provider's API (e.g. AWS EC2 RunInstances) and registers
// the resulting machine in its database.
func (vc *VisorClient) CreateMachine(ctx context.Context, req *VMProvisionRequest) (*VisorMachine, error) {
	owner := req.Owner
	if owner == "" {
		owner = req.Org
	}
	if owner == "" {
		owner = "hanzo"
	}

	// Build the launch spec that Visor's /api/launch-machine expects
	spec := map[string]interface{}{
		"name":         req.NodeID,
		"displayName":  req.DisplayName,
		"instanceType": req.InstanceType,
		"os":           string(req.OS),
	}
	if req.Region != "" {
		spec["region"] = req.Region
	}
	if len(req.Tags) > 0 {
		spec["tags"] = req.Tags
	}
	if len(req.SSHKeyIDs) > 0 {
		spec["sshKeyIds"] = req.SSHKeyIDs
	}

	provider := req.Provider
	if provider == "" {
		provider = "aws" // default provider
	}

	url := fmt.Sprintf("%s/api/launch-machine?owner=%s&provider=%s%s",
		vc.config.Endpoint, owner, provider, vc.authQuery())

	resp, err := vc.doPost(ctx, url, spec)
	if err != nil {
		return nil, fmt.Errorf("visor create machine: %w", err)
	}

	// Visor wraps response in {data: ..., data2: ...} or returns object directly
	var result struct {
		Data *VisorMachine `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil || result.Data == nil {
		var machine VisorMachine
		if err2 := json.Unmarshal(resp, &machine); err2 != nil {
			return nil, fmt.Errorf("visor decode created machine: %w (raw: %s)", err, string(resp))
		}
		return &machine, nil
	}
	return result.Data, nil
}

// DeleteMachine terminates and removes a VM from Visor.
func (vc *VisorClient) DeleteMachine(ctx context.Context, owner, name string) error {
	machine := map[string]interface{}{
		"owner": owner,
		"name":  name,
	}

	deleteURL := fmt.Sprintf("%s/api/delete-machine%s", vc.config.Endpoint, vc.authQueryFirst())
	_, err := vc.doPost(ctx, deleteURL, machine)
	if err != nil {
		return fmt.Errorf("visor delete machine: %w", err)
	}
	return nil
}

// ProtocolForOS returns the default remote protocol for an OS.
func ProtocolForOS(os DesktopOS) string {
	switch os {
	case OSWindows:
		return "rdp"
	case OSMacOS:
		return "vnc"
	case OSLinux:
		return "ssh"
	default:
		return "ssh"
	}
}

// DefaultInstanceType returns a reasonable default instance type for each OS/provider combo.
func DefaultInstanceType(os DesktopOS, providerType string) string {
	switch providerType {
	case "AWS":
		switch os {
		case OSMacOS:
			return "mac2.metal" // Apple Silicon Mac, minimum 24h dedicated host
		case OSWindows:
			return "t3.medium"
		default:
			return "t3.medium"
		}
	case "DigitalOcean":
		return "s-2vcpu-4gb" // DOKS droplet
	case "GCP":
		return "e2-medium"
	default:
		return "t3.medium"
	}
}

// authQuery returns the IAM auth query parameters for Visor API calls.
// Returns "&clientId=...&clientSecret=..." — append to URLs that already have a "?".
func (vc *VisorClient) authQuery() string {
	if vc.config.ClientID == "" {
		return ""
	}
	return fmt.Sprintf("&clientId=%s&clientSecret=%s", vc.config.ClientID, vc.config.ClientSecret)
}

// authQueryFirst returns auth as the first query parameter (starts with "?").
// Use for URLs that have no existing query string.
func (vc *VisorClient) authQueryFirst() string {
	if vc.config.ClientID == "" {
		return ""
	}
	return fmt.Sprintf("?clientId=%s&clientSecret=%s", vc.config.ClientID, vc.config.ClientSecret)
}

func (vc *VisorClient) doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := vc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("visor API %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (vc *VisorClient) doPost(ctx context.Context, url string, payload interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bodyReader)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := vc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("visor API %d: %s", resp.StatusCode, string(body))
	}

	// Visor (Beego) returns HTTP 200 with {"status":"error","msg":"..."} for application errors
	var envelope struct {
		Status string `json:"status"`
		Msg    string `json:"msg"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Status == "error" {
		return nil, fmt.Errorf("visor API error: %s", envelope.Msg)
	}

	logger.Logger.Debug().
		Str("url", url).
		Int("status", resp.StatusCode).
		Msg("visor API call")

	return body, nil
}
