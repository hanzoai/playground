// Package cloud provides cloud agent provisioning on Kubernetes and multi-cloud VMs.
// It creates and manages agent pods in the org's DOKS cluster for Linux,
// and delegates to Visor for Mac/Windows VM provisioning across AWS EC2,
// DigitalOcean, GCP, Azure, Proxmox, and other cloud providers.
package cloud

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hanzoai/playground/control-plane/internal/config"
	"github.com/hanzoai/playground/control-plane/internal/logger"
)

// NodeType distinguishes between local bots and full cloud nodes.
type NodeType string

const (
	// NodeTypeLocal is a lightweight bot running on the user's machine.
	// It connects to the gateway, user approves perms locally.
	NodeTypeLocal NodeType = "local"

	// NodeTypeCloud is a full agent node running in the DOKS cluster.
	// It has its own compute, terminal, desktop, files — like a full blockchain node.
	NodeTypeCloud NodeType = "cloud"
)

// ProvisionRequest describes a cloud agent to create.
type ProvisionRequest struct {
	NodeID      string            `json:"node_id"`
	DisplayName string            `json:"display_name"`
	Model       string            `json:"model"`
	Image       string            `json:"image,omitempty"`  // Override default agent image
	Workspace   string            `json:"workspace,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	CPU         string            `json:"cpu,omitempty"`
	Memory      string            `json:"memory,omitempty"`
	Owner       string            `json:"owner,omitempty"` // IAM user sub
	Org         string            `json:"org,omitempty"`   // Organization
	// Per-user billing: IAM token of the launching user.
	// Injected as HANZO_API_KEY so usage is billed to the user, not a shared service key.
	UserAPIKey string `json:"-"` // Never from JSON; set by handler
	// Multi-OS desktop support
	OS           string `json:"os,omitempty"`            // "linux" (default), "macos", "windows"
	Provider     string `json:"provider,omitempty"`      // Visor provider name for Mac/Windows VMs
	InstanceType string `json:"instance_type,omitempty"` // Cloud instance type (e.g. "mac2.metal", "t3.medium")
}

// ProvisionResult describes the outcome of provisioning.
type ProvisionResult struct {
	NodeID    string   `json:"node_id"`
	PodName   string   `json:"pod_name"`
	Namespace string   `json:"namespace"`
	NodeType  NodeType `json:"node_type"`
	Status    string   `json:"status"` // "provisioning", "running", "failed"
	Endpoint  string   `json:"endpoint,omitempty"` // Internal service URL
	CreatedAt time.Time `json:"created_at"`
}

// CloudNode represents a provisioned cloud agent.
type CloudNode struct {
	NodeID         string            `json:"node_id"`
	PodName        string            `json:"pod_name"`
	Namespace      string            `json:"namespace"`
	NodeType       NodeType          `json:"node_type"`
	Status         string            `json:"status"`
	Image          string            `json:"image"`
	Endpoint       string            `json:"endpoint"`
	Owner          string            `json:"owner"`
	Org            string            `json:"org"`
	OS             string            `json:"os"`              // "linux", "macos", "windows"
	RemoteProtocol string            `json:"remote_protocol"` // "vnc", "rdp", "ssh"
	RemoteURL      string            `json:"remote_url"`      // Visor tunnel URL for Mac/Windows
	Labels         map[string]string `json:"labels"`
	CreatedAt      time.Time         `json:"created_at"`
	LastSeen       time.Time         `json:"last_seen"`
}

// Provisioner manages cloud agent lifecycle on Kubernetes and multi-cloud VMs.
// Linux bots: K8s pods with operative sidecar (cheap, containerized).
// Mac/Windows bots: Real VMs via Visor (AWS EC2, DO, GCP, etc.) with RDP/VNC access.
type Provisioner struct {
	config    config.CloudConfig
	k8s       KubernetesClient
	visor     *VisorClient // nil if Visor not configured
	mu        sync.RWMutex
	nodes     map[string]*CloudNode // nodeID -> node
}

// KubernetesClient is the interface for K8s operations.
// This allows testing with mocks and swapping implementations.
type KubernetesClient interface {
	// CreateAgentPod creates a new agent pod in the cluster.
	CreateAgentPod(ctx context.Context, req *PodSpec) (*PodStatus, error)
	// DeleteAgentPod removes an agent pod.
	DeleteAgentPod(ctx context.Context, namespace, podName string, gracePeriod time.Duration) error
	// GetNodePod returns the current status of an agent pod.
	GetNodePod(ctx context.Context, namespace, podName string) (*PodStatus, error)
	// ListAgentPods lists all agent pods matching label selector.
	ListAgentPods(ctx context.Context, namespace, labelSelector string) ([]*PodStatus, error)
	// GetPodLogs returns recent logs for a pod.
	GetPodLogs(ctx context.Context, namespace, podName string, tailLines int64) (string, error)
}

// SidecarSpec describes an additional container to run alongside the main agent.
type SidecarSpec struct {
	Name     string
	Image    string
	Env      map[string]string
	Ports    []int32
	CPU      string
	Memory   string
	LimitCPU string
	LimitMem string
}

// PodSpec describes the desired state for an agent pod.
type PodSpec struct {
	Name            string
	Namespace       string
	Image           string
	ImagePullSecret string
	ServiceAccount  string
	Env             map[string]string
	Labels          map[string]string
	Annotations     map[string]string
	CPU             string
	Memory          string
	LimitCPU        string
	LimitMemory     string
	Args            []string
	ControlPlaneURL string // URL for agent to connect back
	Sidecars        []SidecarSpec
	NodeSelector    map[string]string
}

// PodStatus represents the current state of a pod.
type PodStatus struct {
	Name      string
	Namespace string
	Phase     string // Pending, Running, Succeeded, Failed, Unknown
	Ready     bool
	IP        string
	StartTime *time.Time
	Message   string
}

// NewProvisioner creates a new cloud agent provisioner.
func NewProvisioner(cfg config.CloudConfig, k8sClient KubernetesClient) *Provisioner {
	p := &Provisioner{
		config: cfg,
		k8s:    k8sClient,
		nodes:     make(map[string]*CloudNode),
	}
	// Initialize Visor client for Mac/Windows VM provisioning
	if cfg.Visor.Enabled && cfg.Visor.Endpoint != "" {
		p.visor = NewVisorClient(cfg.Visor)
		logger.Logger.Info().
			Str("endpoint", cfg.Visor.Endpoint).
			Msg("visor multi-cloud VM provisioner enabled")
	}
	return p
}

// Provision creates a new cloud agent. Routes based on OS:
//   - linux (default): K8s pod with operative sidecar
//   - macos/windows: Visor VM (AWS EC2, DO, GCP, etc.) — charged per day
//
// Local mode: user runs bot on their own machine and connects to the Space.
// Cloud mode: we provision compute for them.
func (p *Provisioner) Provision(ctx context.Context, req *ProvisionRequest) (*ProvisionResult, error) {
	if !p.config.Enabled || !p.config.Kubernetes.Enabled {
		return nil, fmt.Errorf("cloud provisioning is not enabled")
	}

	// Route Mac/Windows to Visor VM provisioning
	os := DesktopOS(req.OS)
	if os == OSMacOS || os == OSWindows {
		return p.provisionVM(ctx, req)
	}
	// Default: Linux K8s pod (with or without operative desktop sidecar)
	return p.provisionK8sPod(ctx, req)
}

// provisionK8sPod creates a Linux bot as a K8s pod with operative sidecar.
func (p *Provisioner) provisionK8sPod(ctx context.Context, req *ProvisionRequest) (*ProvisionResult, error) {

	// Generate node ID if not provided
	nodeID := req.NodeID
	if nodeID == "" {
		nodeID = fmt.Sprintf("cloud-%s", uuid.New().String()[:8])
	}

	// Sanitize pod name (K8s DNS-1123 label)
	podName := sanitizePodName(fmt.Sprintf("agent-%s", nodeID))

	namespace := p.config.Kubernetes.Namespace
	image := p.config.Kubernetes.BotImage
	if req.Image != "" {
		image = req.Image
	}

	// Resource defaults — scale based on mode
	cpu := p.config.Kubernetes.DefaultCPU
	memory := p.config.Kubernetes.DefaultMemory
	if req.OS == "terminal" {
		// Terminal-only: no X11/VNC but still needs Node.js heap for bot agent
		if cpu == p.config.Kubernetes.DefaultCPU {
			cpu = "200m"
		}
		if memory == p.config.Kubernetes.DefaultMemory {
			memory = "512Mi"
		}
	}
	if req.CPU != "" {
		cpu = req.CPU
	}
	if req.Memory != "" {
		memory = req.Memory
	}

	// Check org limits
	if req.Org != "" {
		count := p.countByOrg(req.Org)
		if count >= p.config.Kubernetes.MaxAgentsPerOrg {
			return nil, fmt.Errorf("organization %q has reached the maximum of %d cloud agents", req.Org, p.config.Kubernetes.MaxAgentsPerOrg)
		}
	}

	// Build labels
	labels := map[string]string{
		"app.kubernetes.io/name":       "playground-agent",
		"app.kubernetes.io/part-of":    "hanzo-playground",
		"app.kubernetes.io/managed-by": "playground-provisioner",
		"playground.hanzo.ai/node-id":  nodeID,
		"playground.hanzo.ai/type":     string(NodeTypeCloud),
	}
	if req.Org != "" {
		labels["playground.hanzo.ai/org"] = req.Org
	}
	if req.Owner != "" {
		labels["playground.hanzo.ai/owner"] = sanitizeLabel(req.Owner)
	}
	for k, v := range req.Labels {
		labels[k] = v
	}

	// Build env vars
	env := map[string]string{
		"AGENT_NODE_ID":           nodeID,
		"PLAYGROUND_SERVER":           fmt.Sprintf("http://hanzo-playground.%s.svc:8080", namespace),
		"AGENT_NODE_TYPE":             string(NodeTypeCloud),
		"HANZO_PLAYGROUND_MODE":       "production",
		"HANZO_PLAYGROUND_CLOUD_NODE": "true",
	}
	if req.Model != "" {
		env["AGENT_MODEL"] = req.Model
	}
	if req.Workspace != "" {
		env["AGENT_WORKSPACE"] = req.Workspace
	}
	if req.DisplayName != "" {
		env["AGENT_DISPLAY_NAME"] = req.DisplayName
	}
	// Inject Hanzo API env vars for bot LLM calls.
	// Default: api.hanzo.ai; overridable via CLOUD_API_ENDPOINT env var.
	if p.config.Kubernetes.CloudAPIEndpoint != "" {
		env["HANZO_API_BASE"] = p.config.Kubernetes.CloudAPIEndpoint
		env["OPENAI_API_BASE"] = p.config.Kubernetes.CloudAPIEndpoint // backward compat
	}
	// Per-user billing: use the launching user's API key so usage is
	// tracked and billed to their account. Fall back to shared service key.
	apiKey := req.UserAPIKey
	if apiKey == "" {
		apiKey = p.config.Kubernetes.CloudAPIKey
	}
	if apiKey != "" {
		env["HANZO_API_KEY"] = apiKey
		env["OPENAI_API_KEY"] = apiKey // backward compat
	}
	if req.UserAPIKey != "" {
		logger.Logger.Info().
			Str("node_id", nodeID).
			Str("owner", req.Owner).
			Msg("using per-user API key for billing")
	}

	// The bot gateway auto-detects IAM mode when HANZO_PLAYGROUND_CLOUD_NODE=true,
	// but it reads IAM config from its config file (gateway.auth.iam), not env vars.
	// Override to token auth (with key) or none (without key). Cloud pods are only
	// accessible within the K8s cluster — the playground handles real auth upstream.
	if apiKey != "" {
		env["BOT_GATEWAY_AUTH_MODE"] = "token"
		env["BOT_GATEWAY_TOKEN"] = apiKey
	} else {
		env["BOT_GATEWAY_AUTH_MODE"] = "none"
	}

	// Set NODE_OPTIONS to scale V8 heap based on container memory limit.
	// The bot image (Node.js) defaults to ~512MB heap which OOMs under tight limits.
	// Reserve ~128MB for OS/native overhead and give the rest to V8.
	env["NODE_OPTIONS"] = nodeOptionsForMemory(p.config.Kubernetes.LimitMemory)

	// Terminal-only mode: skip operative desktop, use lightweight ttyd shell access.
	// This is for "xterm + Claude Code" — no desktop environment needed.
	terminalOnly := req.OS == "terminal"

	if terminalOnly {
		env["AGENT_MODE"] = "terminal"
		env["TTYD_URL"] = "http://localhost:7681"
	} else if p.config.Kubernetes.OperativeEnabled {
		// Wire operative desktop URL if sidecar is enabled
		env["OPERATIVE_URL"] = "http://localhost:8501"
		env["OPERATIVE_VNC_URL"] = "http://localhost:6080"
	}

	// User-provided env vars override everything
	for k, v := range req.Env {
		env[k] = v
	}

	// Build sidecar containers
	var sidecars []SidecarSpec
	if terminalOnly {
		// Terminal-only: lightweight ttyd for web-based terminal access
		sidecars = append(sidecars, SidecarSpec{
			Name:  "ttyd",
			Image: "tsl0922/ttyd:1.7.7-alpine",
			Env: map[string]string{
				"TERM": "xterm-256color",
			},
			Ports:    []int32{7681},
			CPU:      "100m",
			Memory:   "128Mi",
			LimitCPU: "500m",
			LimitMem: "512Mi",
		})
	} else if p.config.Kubernetes.OperativeEnabled {
		sidecars = append(sidecars, SidecarSpec{
			Name:  "operative",
			Image: p.config.Kubernetes.OperativeImage,
			Env: map[string]string{
				"DISPLAY":     ":1",
				"DISPLAY_NUM": "1",
				"RESOLUTION":  "1920x1080x24",
				"WIDTH":       "1920",
				"HEIGHT":      "1080",
			},
			Ports:    []int32{8080, 6080, 5900, 8501},
			CPU:      "200m",
			Memory:   "512Mi",
			LimitCPU: "1000m",
			LimitMem: "2Gi",
		})
	}

	spec := &PodSpec{
		Name:            podName,
		Namespace:       namespace,
		Image:           image,
		ImagePullSecret: p.config.Kubernetes.ImagePullSecret,
		ServiceAccount:  p.config.Kubernetes.ServiceAccount,
		Env:             env,
		Labels:          labels,
		Annotations: map[string]string{
			"playground.hanzo.ai/provisioned-at": time.Now().UTC().Format(time.RFC3339),
		},
		CPU:             cpu,
		Memory:          memory,
		LimitCPU:        p.config.Kubernetes.LimitCPU,
		LimitMemory:     p.config.Kubernetes.LimitMemory,
		Args:            []string{}, // Agent image entrypoint handles startup
		ControlPlaneURL: fmt.Sprintf("http://hanzo-playground.%s.svc:8080", namespace),
		Sidecars:        sidecars,
		NodeSelector:    p.config.Kubernetes.NodeSelector,
	}

	logger.Logger.Info().
		Str("node_id", nodeID).
		Str("pod_name", podName).
		Str("namespace", namespace).
		Str("image", image).
		Str("org", req.Org).
		Msg("provisioning cloud agent")

	podStatus, err := p.k8s.CreateAgentPod(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent pod: %w", err)
	}

	nodeOS := "linux"
	nodeProtocol := "vnc"
	if terminalOnly {
		nodeOS = "terminal"
		nodeProtocol = "ssh"
	}

	node := &CloudNode{
		NodeID:         nodeID,
		PodName:        podName,
		Namespace:      namespace,
		NodeType:       NodeTypeCloud,
		Status:         podStatus.Phase,
		Image:          image,
		OS:             nodeOS,
		RemoteProtocol: nodeProtocol,
		Endpoint:       fmt.Sprintf("http://%s.%s.svc:8001", podName, namespace),
		Owner:          req.Owner,
		Org:            req.Org,
		Labels:         labels,
		CreatedAt:      time.Now(),
		LastSeen:       time.Now(),
	}

	p.mu.Lock()
	p.nodes[nodeID] = node
	p.mu.Unlock()

	return &ProvisionResult{
		NodeID:    nodeID,
		PodName:   podName,
		Namespace: namespace,
		NodeType:  NodeTypeCloud,
		Status:    podStatus.Phase,
		Endpoint:  node.Endpoint,
		CreatedAt: node.CreatedAt,
	}, nil
}

// provisionVM creates a Mac or Windows VM via Visor and registers the bot.
// Mac: minimum 1-day billing (Apple licensing on dedicated hardware).
// Windows: RDP-based, charged per day.
// Users can also connect their own Mac/Windows machines as local nodes.
func (p *Provisioner) provisionVM(ctx context.Context, req *ProvisionRequest) (*ProvisionResult, error) {
	if p.visor == nil {
		return nil, fmt.Errorf("visor not configured — Mac/Windows VMs require Visor integration (set HANZO_AGENTS_VISOR_ENABLED=true)")
	}

	nodeID := req.NodeID
	if nodeID == "" {
		nodeID = fmt.Sprintf("vm-%s", uuid.New().String()[:8])
	}

	os := DesktopOS(req.OS)
	protocol := ProtocolForOS(os)
	org := req.Org
	if org == "" {
		org = "hanzo"
	}

	// Check org limits
	if req.Org != "" {
		count := p.countByOrg(req.Org)
		if count >= p.config.Kubernetes.MaxAgentsPerOrg {
			return nil, fmt.Errorf("organization %q has reached the maximum of %d cloud agents", req.Org, p.config.Kubernetes.MaxAgentsPerOrg)
		}
	}

	logger.Logger.Info().
		Str("node_id", nodeID).
		Str("os", string(os)).
		Str("provider", req.Provider).
		Str("protocol", protocol).
		Str("org", req.Org).
		Msg("provisioning VM via visor")

	// List available machines from Visor, or return info for provisioning
	machines, err := p.visor.ListMachines(ctx, org)
	if err != nil {
		logger.Logger.Warn().Err(err).Msg("visor list machines failed, continuing with registration")
	}

	// Look for an available machine matching the OS
	var matchedMachine *VisorMachine
	for i := range machines {
		m := &machines[i]
		if strings.EqualFold(m.OS, string(os)) && m.State == "Running" {
			matchedMachine = m
			break
		}
	}

	remoteURL := ""
	machineName := sanitizePodName(fmt.Sprintf("vm-%s", nodeID))

	if matchedMachine != nil {
		machineName = matchedMachine.Name
		remoteURL = fmt.Sprintf("%s/api/get-asset-tunnel?assetId=%s/%s",
			p.config.Visor.Endpoint, org, machineName)
		logger.Logger.Info().
			Str("machine", machineName).
			Str("ip", matchedMachine.PublicIP).
			Msg("matched existing visor machine")
	} else {
		// No running VM found — launch one via Visor's cloud provider
		logger.Logger.Info().
			Str("os", string(os)).
			Str("provider", req.Provider).
			Msg("no running VM found — launching via visor")

		instanceType := req.InstanceType
		if instanceType == "" {
			providerType := "AWS" // default
			if req.Provider != "" {
				providerType = req.Provider
			}
			instanceType = DefaultInstanceType(os, providerType)
		}

		vmReq := &VMProvisionRequest{
			NodeID:       nodeID,
			DisplayName:  req.DisplayName,
			OS:           os,
			Provider:     req.Provider,
			Region:       "",
			InstanceType: instanceType,
			Owner:        req.Owner,
			Org:          org,
		}
		if vmReq.DisplayName == "" {
			vmReq.DisplayName = fmt.Sprintf("agent-%s-%s", os, nodeID)
		}

		created, err := p.visor.CreateMachine(ctx, vmReq)
		if err != nil {
			logger.Logger.Error().Err(err).
				Str("os", string(os)).
				Msg("failed to launch VM via visor — returning pending status")
		} else {
			machineName = created.Name
			remoteURL = fmt.Sprintf("%s/api/get-asset-tunnel?assetId=%s/%s",
				p.config.Visor.Endpoint, org, machineName)
			logger.Logger.Info().
				Str("machine", machineName).
				Str("state", created.State).
				Msg("launched VM via visor")
		}
	}

	node := &CloudNode{
		NodeID:         nodeID,
		PodName:        machineName,
		Namespace:      org,
		NodeType:       NodeTypeCloud,
		Status:         "Provisioning",
		Image:          fmt.Sprintf("vm:%s", os),
		OS:             string(os),
		RemoteProtocol: protocol,
		RemoteURL:      remoteURL,
		Endpoint:       remoteURL,
		Owner:          req.Owner,
		Org:            req.Org,
		Labels: map[string]string{
			"playground.hanzo.ai/node-id": nodeID,
			"playground.hanzo.ai/type":    "vm",
			"playground.hanzo.ai/os":      string(os),
		},
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
	}

	if matchedMachine != nil {
		node.Status = matchedMachine.State
	}

	p.mu.Lock()
	p.nodes[nodeID] = node
	p.mu.Unlock()

	return &ProvisionResult{
		NodeID:    nodeID,
		PodName:   machineName,
		Namespace: org,
		NodeType:  NodeTypeCloud,
		Status:    node.Status,
		Endpoint:  remoteURL,
		CreatedAt: node.CreatedAt,
	}, nil
}

// Deprovision removes a cloud agent from the cluster or terminates a VM.
func (p *Provisioner) Deprovision(ctx context.Context, nodeID string) error {
	p.mu.RLock()
	node, ok := p.nodes[nodeID]
	p.mu.RUnlock()

	if !ok {
		return fmt.Errorf("cloud node %q not found", nodeID)
	}

	logger.Logger.Info().
		Str("node_id", nodeID).
		Str("pod_name", node.PodName).
		Str("os", node.OS).
		Msg("deprovisioning cloud agent")

	// Route VM nodes (macOS/Windows) to Visor for teardown
	os := DesktopOS(node.OS)
	if os == OSMacOS || os == OSWindows {
		if p.visor == nil {
			return fmt.Errorf("visor not configured — cannot deprovision VM node %q", nodeID)
		}
		owner := node.Org
		if owner == "" {
			owner = "hanzo"
		}
		if err := p.visor.DeleteMachine(ctx, owner, node.PodName); err != nil {
			return fmt.Errorf("failed to delete VM via visor: %w", err)
		}
	} else {
		// Linux/terminal pods: delete via K8s API
		if err := p.k8s.DeleteAgentPod(ctx, node.Namespace, node.PodName, p.config.Kubernetes.GracefulShutdown); err != nil {
			return fmt.Errorf("failed to delete agent pod: %w", err)
		}
	}

	p.mu.Lock()
	delete(p.nodes, nodeID)
	p.mu.Unlock()

	return nil
}

// GetNode returns info about a provisioned cloud node.
func (p *Provisioner) GetNode(ctx context.Context, nodeID string) (*CloudNode, error) {
	p.mu.RLock()
	node, ok := p.nodes[nodeID]
	p.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("cloud node %q not found", nodeID)
	}

	// Refresh status from K8s
	podStatus, err := p.k8s.GetNodePod(ctx, node.Namespace, node.PodName)
	if err != nil {
		return node, nil // Return cached info on K8s error
	}

	p.mu.Lock()
	node.Status = podStatus.Phase
	node.LastSeen = time.Now()
	p.mu.Unlock()

	return node, nil
}

// ListNodes returns all provisioned cloud nodes, optionally filtered by org.
func (p *Provisioner) ListNodes(ctx context.Context, org string) ([]*CloudNode, error) {
	selector := "app.kubernetes.io/managed-by=playground-provisioner"
	if org != "" {
		selector += fmt.Sprintf(",playground.hanzo.ai/org=%s", org)
	}

	pods, err := p.k8s.ListAgentPods(ctx, p.config.Kubernetes.Namespace, selector)
	if err != nil {
		// Fall back to in-memory list
		p.mu.RLock()
		defer p.mu.RUnlock()
		var result []*CloudNode
		for _, n := range p.nodes {
			if org == "" || n.Org == org {
				result = append(result, n)
			}
		}
		return result, nil
	}

	// Sync in-memory state with K8s
	p.mu.Lock()
	defer p.mu.Unlock()

	var result []*CloudNode
	for _, ps := range pods {
		nodeID := ""
		// Extract node ID from pod name
		if strings.HasPrefix(ps.Name, "agent-") {
			nodeID = strings.TrimPrefix(ps.Name, "agent-")
		}
		if existing, ok := p.nodes[nodeID]; ok {
			existing.Status = ps.Phase
			existing.LastSeen = time.Now()
			result = append(result, existing)
		} else {
			// Pod exists in K8s but not in memory — rehydrate
			node := &CloudNode{
				NodeID:    nodeID,
				PodName:   ps.Name,
				Namespace: ps.Namespace,
				NodeType:  NodeTypeCloud,
				Status:    ps.Phase,
				LastSeen:  time.Now(),
			}
			p.nodes[nodeID] = node
			result = append(result, node)
		}
	}

	return result, nil
}

// GetLogs returns recent logs for a cloud agent pod.
func (p *Provisioner) GetLogs(ctx context.Context, nodeID string, tailLines int64) (string, error) {
	p.mu.RLock()
	node, ok := p.nodes[nodeID]
	p.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("cloud node %q not found", nodeID)
	}

	return p.k8s.GetPodLogs(ctx, node.Namespace, node.PodName, tailLines)
}

// Sync refreshes the in-memory node list from Kubernetes.
func (p *Provisioner) Sync(ctx context.Context) error {
	_, err := p.ListNodes(ctx, "")
	return err
}

// countByOrg returns the number of cloud nodes for an organization.
func (p *Provisioner) countByOrg(org string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	count := 0
	for _, n := range p.nodes {
		if n.Org == org {
			count++
		}
	}
	return count
}

// sanitizePodName ensures the name is a valid K8s DNS-1123 label.
func sanitizePodName(name string) string {
	name = strings.ToLower(name)
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, name)
	name = strings.Trim(name, "-")
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

// nodeOptionsForMemory returns a NODE_OPTIONS string with --max-old-space-size
// set appropriately for the given K8s memory limit (e.g. "4Gi", "2Gi", "512Mi").
// Reserves 128MB for OS and native heap, gives the rest to V8.
func nodeOptionsForMemory(memLimit string) string {
	limitMB := 0
	memLimit = strings.TrimSpace(memLimit)
	if strings.HasSuffix(memLimit, "Gi") {
		s := strings.TrimSuffix(memLimit, "Gi")
		if n, err := strconv.Atoi(s); err == nil {
			limitMB = n * 1024
		}
	} else if strings.HasSuffix(memLimit, "Mi") {
		s := strings.TrimSuffix(memLimit, "Mi")
		if n, err := strconv.Atoi(s); err == nil {
			limitMB = n
		}
	}
	if limitMB <= 256 {
		limitMB = 1024 // fallback: 1GB
	}
	heapMB := limitMB - 128 // reserve for OS + native
	if heapMB < 256 {
		heapMB = 256
	}
	return fmt.Sprintf("--max-old-space-size=%d", heapMB)
}

// sanitizeLabel ensures a value is safe for K8s labels.
func sanitizeLabel(val string) string {
	val = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, val)
	if len(val) > 63 {
		val = val[:63]
	}
	return val
}
