// Package cloud provides cloud agent provisioning on Kubernetes and multi-cloud VMs.
// It creates and manages agent pods in the org's DOKS cluster for Linux,
// and delegates to Visor for Mac/Windows VM provisioning across AWS EC2,
// DigitalOcean, GCP, Azure, Proxmox, and other cloud providers.
package cloud

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	OS           string   `json:"os,omitempty"`            // "linux" (default), "macos", "windows"
	Provider     string   `json:"provider,omitempty"`      // Visor provider name for Mac/Windows VMs
	InstanceType string   `json:"instance_type,omitempty"` // Cloud instance type (e.g. "mac2.metal", "t3.medium")
	SSHKeyIDs    []string `json:"ssh_key_ids,omitempty"`   // Provider SSH key IDs to inject
}

// ProvisionResult describes the outcome of provisioning.
type ProvisionResult struct {
	NodeID    string    `json:"node_id"`
	PodName   string    `json:"pod_name"`
	Namespace string    `json:"namespace"`
	NodeType  NodeType  `json:"node_type"`
	Status    string    `json:"status"` // "provisioning", "running", "failed"
	Endpoint  string    `json:"endpoint,omitempty"` // Internal service URL
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
	// Billing: hold placed at provision time, settled on deprovision.
	HoldID        string    `json:"hold_id,omitempty"`
	ProvisionedAt time.Time `json:"provisioned_at"`
	// CentsPerHour is the billing rate used for hold/settle calculations.
	CentsPerHour int `json:"cents_per_hour,omitempty"`
	// BillingUserID is the Commerce user ID (org/name format) for billing.
	BillingUserID string `json:"billing_user_id,omitempty"`
	// BearerToken is the IAM token of the launching user, kept for settle calls.
	BearerToken string `json:"-"`
}

// Provisioner manages cloud agent lifecycle on Kubernetes and multi-cloud VMs.
// Linux bots: K8s pods with operative sidecar (cheap, containerized).
// Mac/Windows bots: Real VMs via Visor (AWS EC2, DO, GCP, etc.) with RDP/VNC access.
//
// Node state is persisted to BoltDB so tracked agents survive pod restarts.
// Without persistence, a restart loses all tracked agents, creating orphaned
// compute and billing leaks.
type Provisioner struct {
	config config.CloudConfig
	k8s    KubernetesClient
	visor  *VisorClient // nil if Visor not configured
	mu     sync.RWMutex
	store  *NodeStore
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
	// CreateSecret creates or updates a K8s Secret with the given data.
	CreateSecret(ctx context.Context, namespace, name string, data map[string]string, labels map[string]string) error
	// DeleteSecret removes a K8s Secret.
	DeleteSecret(ctx context.Context, namespace, name string) error
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
	// SecretRef is the name of a K8s Secret whose keys are injected as env vars
	// via envFrom. Sensitive values (API keys, tokens) go here instead of Env.
	SecretRef string
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

// NewProvisioner creates a new cloud agent provisioner backed by BoltDB.
// dataDir is the directory where the nodes.db file will be stored; it defaults
// to the value of PLAYGROUND_DATA_DIR (or /data if unset).
func NewProvisioner(cfg config.CloudConfig, k8sClient KubernetesClient, dataDir string) (*Provisioner, error) {
	if dataDir == "" {
		dataDir = os.Getenv("PLAYGROUND_DATA_DIR")
	}
	if dataDir == "" {
		dataDir = "/data"
	}

	store, err := NewNodeStore(filepath.Join(dataDir, "nodes.db"))
	if err != nil {
		return nil, fmt.Errorf("open node store: %w", err)
	}

	p := &Provisioner{
		config: cfg,
		k8s:    k8sClient,
		store:  store,
	}
	// Initialize Visor client for Mac/Windows VM provisioning
	if cfg.Visor.Enabled && cfg.Visor.Endpoint != "" {
		p.visor = NewVisorClient(cfg.Visor)
		logger.Logger.Info().
			Str("endpoint", cfg.Visor.Endpoint).
			Msg("visor multi-cloud VM provisioner enabled")
	}

	logger.Logger.Info().
		Str("db_path", filepath.Join(dataDir, "nodes.db")).
		Msg("node store opened for crash recovery")

	return p, nil
}

// Close releases the underlying BoltDB resources. Call on shutdown.
func (p *Provisioner) Close() error {
	return p.store.Close()
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
	// Route DigitalOcean droplet requests to Visor VM provisioning.
	if req.Provider == "digitalocean" {
		return p.provisionDroplet(ctx, req)
	}
	// Default: Linux K8s pod (with or without operative desktop sidecar)
	return p.provisionK8sPod(ctx, req)
}

// provisionK8sPod creates a Linux bot as a K8s pod with operative sidecar.
func (p *Provisioner) provisionK8sPod(ctx context.Context, req *ProvisionRequest) (*ProvisionResult, error) {
	nodeID := req.NodeID
	if nodeID == "" {
		nodeID = fmt.Sprintf("cloud-%s", uuid.New().String()[:8])
	}
	podName := sanitizePodName(fmt.Sprintf("agent-%s", nodeID))
	namespace := p.config.Kubernetes.Namespace
	image := p.config.Kubernetes.BotImage
	if req.Image != "" {
		image = req.Image
	}

	cpu := p.config.Kubernetes.DefaultCPU
	memory := p.config.Kubernetes.DefaultMemory
	if req.OS == "terminal" {
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

	if req.Org != "" {
		count := p.countByOrg(req.Org)
		if count >= p.config.Kubernetes.MaxAgentsPerOrg {
			return nil, fmt.Errorf("organization %q has reached the maximum of %d cloud agents", req.Org, p.config.Kubernetes.MaxAgentsPerOrg)
		}
	}

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

	env := map[string]string{
		"AGENT_NODE_ID":         nodeID,
		"PLAYGROUND_SERVER":     fmt.Sprintf("http://hanzo-playground.%s.svc:8080", namespace),
		"AGENT_NODE_TYPE":       string(NodeTypeCloud),
		"HANZO_PLAYGROUND_MODE": "production",
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
	if p.config.Kubernetes.CloudAPIEndpoint != "" {
		env["HANZO_API_BASE"] = p.config.Kubernetes.CloudAPIEndpoint
		env["OPENAI_API_BASE"] = p.config.Kubernetes.CloudAPIEndpoint
	}
	apiKey := req.UserAPIKey
	if apiKey == "" {
		apiKey = p.config.Kubernetes.CloudAPIKey
	}
	if apiKey != "" {
		env["HANZO_API_KEY"] = apiKey
		env["OPENAI_API_KEY"] = apiKey
	}
	if req.UserAPIKey != "" {
		logger.Logger.Info().
			Str("node_id", nodeID).
			Str("owner", req.Owner).
			Str("api_key", RedactKey(req.UserAPIKey)).
			Msg("using per-user API key for billing")
	}

	env["BOT_CLOUD_NODE"] = "true"
	if p.config.Kubernetes.GatewayURL != "" {
		env["BOT_NODE_GATEWAY_URL"] = p.config.Kubernetes.GatewayURL
	}
	gatewayToken := p.config.Kubernetes.GatewayToken
	if gatewayToken != "" {
		env["BOT_GATEWAY_TOKEN"] = gatewayToken
	} else if apiKey != "" {
		env["BOT_GATEWAY_TOKEN"] = apiKey
	} else {
		env["BOT_GATEWAY_TOKEN"] = nodeID
	}
	env["NODE_OPTIONS"] = nodeOptionsForMemory(p.config.Kubernetes.LimitMemory)

	terminalOnly := req.OS == "terminal"
	if terminalOnly {
		env["AGENT_MODE"] = "terminal"
		env["TTYD_URL"] = "http://localhost:7681"
	} else if p.config.Kubernetes.OperativeEnabled {
		env["OPERATIVE_URL"] = "http://localhost:8501"
		env["OPERATIVE_VNC_URL"] = "http://localhost:6080"
	}

	for k, v := range req.Env {
		env[k] = v
	}

	var sidecars []SidecarSpec
	if terminalOnly {
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
		operativeEnv := map[string]string{
			"DISPLAY":      ":1",
			"DISPLAY_NUM":  "1",
			"RESOLUTION":   "1920x1080x24",
			"WIDTH":        "1920",
			"HEIGHT":       "1080",
			"API_PROVIDER": "hanzo",
		}
		if apiKey != "" {
			env["ANTHROPIC_API_KEY"] = apiKey
		}
		if p.config.Kubernetes.CloudAPIEndpoint != "" {
			operativeEnv["HANZO_API_BASE"] = p.config.Kubernetes.CloudAPIEndpoint
		}
		sidecars = append(sidecars, SidecarSpec{
			Name:     "operative",
			Image:    p.config.Kubernetes.OperativeImage,
			Env:      operativeEnv,
			Ports:    []int32{8080, 6080, 5900, 8501},
			CPU:      "200m",
			Memory:   "512Mi",
			LimitCPU: "1000m",
			LimitMem: "2Gi",
		})
	}

	safeEnv, secretData := splitSensitiveEnv(env)
	secretName := agentSecretName(nodeID)

	if len(secretData) > 0 {
		secretLabels := map[string]string{
			"app.kubernetes.io/managed-by": "playground-provisioner",
			"playground.hanzo.ai/node-id":  nodeID,
		}
		if err := p.k8s.CreateSecret(ctx, namespace, secretName, secretData, secretLabels); err != nil {
			return nil, fmt.Errorf("failed to create agent secret: %w", err)
		}
	}

	spec := &PodSpec{
		Name:            podName,
		Namespace:       namespace,
		Image:           image,
		ImagePullSecret: p.config.Kubernetes.ImagePullSecret,
		ServiceAccount:  p.config.Kubernetes.ServiceAccount,
		Env:             safeEnv,
		Labels:          labels,
		Annotations: map[string]string{
			"playground.hanzo.ai/provisioned-at": time.Now().UTC().Format(time.RFC3339),
		},
		CPU:             cpu,
		Memory:          memory,
		LimitCPU:        p.config.Kubernetes.LimitCPU,
		LimitMemory:     p.config.Kubernetes.LimitMemory,
		Args:            nodeArgs(p.config.Kubernetes.GatewayURL, nodeID),
		ControlPlaneURL: fmt.Sprintf("http://hanzo-playground.%s.svc:8080", namespace),
		Sidecars:        sidecars,
		NodeSelector:    p.config.Kubernetes.NodeSelector,
	}
	if len(secretData) > 0 {
		spec.SecretRef = secretName
	}

	logger.Logger.Info().
		Str("node_id", nodeID).
		Str("pod_name", podName).
		Str("namespace", namespace).
		Str("image", image).
		Str("org", req.Org).
		Str("secret", secretName).
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
		Endpoint:       p.config.Kubernetes.GatewayURL,
		Owner:          req.Owner,
		Org:            req.Org,
		Labels:         labels,
		CreatedAt:      time.Now(),
		LastSeen:       time.Now(),
	}

	if err := p.store.Put(node); err != nil {
		return nil, fmt.Errorf("persist node %s: %w", nodeID, err)
	}

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

	machines, err := p.visor.ListMachines(ctx, org)
	if err != nil {
		logger.Logger.Warn().Err(err).Msg("visor list machines failed, continuing with registration")
	}

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
		logger.Logger.Info().
			Str("os", string(os)).
			Str("provider", req.Provider).
			Msg("no running VM found — launching via visor")

		instanceType := req.InstanceType
		if instanceType == "" {
			providerType := "AWS"
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

	if err := p.store.Put(node); err != nil {
		return nil, fmt.Errorf("persist node %s: %w", nodeID, err)
	}

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

// provisionDroplet creates a DigitalOcean droplet with @hanzo/bot pre-installed.
func (p *Provisioner) provisionDroplet(ctx context.Context, req *ProvisionRequest) (*ProvisionResult, error) {
	if p.visor == nil {
		return nil, fmt.Errorf("visor not configured — DigitalOcean droplet provisioning requires Visor (set HANZO_AGENTS_VISOR_ENABLED=true)")
	}

	nodeID := req.NodeID
	if nodeID == "" {
		nodeID = fmt.Sprintf("hz-%s", uuid.New().String()[:8])
	}

	org := req.Org
	if org == "" {
		org = "hanzo"
	}

	if req.Org != "" {
		count := p.countByOrg(req.Org)
		if count >= p.config.Kubernetes.MaxAgentsPerOrg {
			return nil, fmt.Errorf("organization %q has reached the maximum of %d cloud agents", req.Org, p.config.Kubernetes.MaxAgentsPerOrg)
		}
	}

	instanceType := req.InstanceType
	if instanceType == "" {
		instanceType = "s-2vcpu-4gb"
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = fmt.Sprintf("agent-%s", nodeID)
	}

	logger.Logger.Info().
		Str("node_id", nodeID).
		Str("instance_type", instanceType).
		Str("provider", "digitalocean").
		Str("org", org).
		Msg("provisioning DO droplet via visor")

	// Pass non-sensitive env vars as tags for cloud-init. Droplets run outside
	// K8s, so use the public gateway URL (wss://gw.hanzo.bot) rather than the
	// internal K8s service URL.
	//
	// SECURITY: API keys and tokens are NOT placed in env: tags because DO tags
	// are readable via the DO API. Instead, they are passed via user-data
	// (write-only) using the "secret:" prefix, which Visor's cloud-init reads
	// only from the droplet's user-data metadata (not from API-visible tags).
	gatewayURL := p.config.Kubernetes.GatewayURL
	if strings.HasPrefix(gatewayURL, "ws://") && strings.Contains(gatewayURL, ".svc") {
		gatewayURL = "wss://gw.hanzo.bot"
	}
	tags := map[string]string{
		"env:BOT_NODE_GATEWAY_URL": gatewayURL,
		"env:AGENT_NODE_ID":        nodeID,
	}
	if p.config.Kubernetes.GatewayToken != "" {
		tags["secret:BOT_GATEWAY_TOKEN"] = p.config.Kubernetes.GatewayToken
	}
	if req.UserAPIKey != "" {
		tags["secret:HANZO_API_KEY"] = req.UserAPIKey
		logger.Logger.Info().
			Str("node_id", nodeID).
			Str("api_key", RedactKey(req.UserAPIKey)).
			Msg("passing user API key via secure user-data for droplet")
	}

	vmReq := &VMProvisionRequest{
		NodeID:       nodeID,
		DisplayName:  displayName,
		OS:           OSLinux,
		Provider:     "DigitalOcean",
		InstanceType: instanceType,
		Owner:        req.Owner,
		Org:          org,
		Tags:         tags,
		SSHKeyIDs:    req.SSHKeyIDs,
	}

	created, err := p.visor.CreateMachine(ctx, vmReq)
	if err != nil {
		return nil, fmt.Errorf("visor create droplet: %w", err)
	}

	machineName := created.Name
	remoteURL := fmt.Sprintf("%s/api/get-asset-tunnel?assetId=%s/%s",
		p.config.Visor.Endpoint, org, machineName)

	node := &CloudNode{
		NodeID:         nodeID,
		PodName:        machineName,
		Namespace:      org,
		NodeType:       NodeTypeCloud,
		Status:         created.State,
		Image:          fmt.Sprintf("droplet:%s", instanceType),
		OS:             "linux",
		RemoteProtocol: "ssh",
		RemoteURL:      remoteURL,
		Endpoint:       p.config.Kubernetes.GatewayURL,
		Owner:          req.Owner,
		Org:            req.Org,
		Labels: map[string]string{
			"playground.hanzo.ai/node-id":  nodeID,
			"playground.hanzo.ai/type":     "droplet",
			"playground.hanzo.ai/provider": "digitalocean",
			"playground.hanzo.ai/size":     instanceType,
		},
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
	}

	if err := p.store.Put(node); err != nil {
		return nil, fmt.Errorf("persist node %s: %w", nodeID, err)
	}

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
	node, err := p.store.Get(nodeID)
	if err != nil {
		return fmt.Errorf("read node %s from store: %w", nodeID, err)
	}
	if node == nil {
		return fmt.Errorf("cloud node %q not found", nodeID)
	}

	logger.Logger.Info().
		Str("node_id", nodeID).
		Str("pod_name", node.PodName).
		Str("os", node.OS).
		Msg("deprovisioning cloud agent")

	os := DesktopOS(node.OS)
	isVisorManaged := os == OSMacOS || os == OSWindows || node.Labels["playground.hanzo.ai/type"] == "droplet"
	if isVisorManaged {
		if p.visor == nil {
			return fmt.Errorf("visor not configured — cannot deprovision node %q", nodeID)
		}
		owner := node.Org
		if owner == "" {
			owner = "hanzo"
		}
		if err := p.visor.DeleteMachine(ctx, owner, node.PodName); err != nil {
			return fmt.Errorf("failed to delete machine via visor: %w", err)
		}
	} else {
		// Delete the K8s Secret holding API keys before deleting the pod.
		// Best-effort: the secret may not exist if the node was provisioned
		// before the secret-based approach was introduced.
		secretName := agentSecretName(nodeID)
		if err := p.k8s.DeleteSecret(ctx, node.Namespace, secretName); err != nil {
			logger.Logger.Warn().Err(err).
				Str("secret", secretName).
				Msg("failed to delete agent secret (may not exist)")
		}
		if err := p.k8s.DeleteAgentPod(ctx, node.Namespace, node.PodName, p.config.Kubernetes.GracefulShutdown); err != nil {
			return fmt.Errorf("failed to delete agent pod: %w", err)
		}
	}

	if err := p.store.Delete(nodeID); err != nil {
		return fmt.Errorf("remove node %s from store: %w", nodeID, err)
	}

	return nil
}

// GetNode returns info about a provisioned cloud node.
func (p *Provisioner) GetNode(ctx context.Context, nodeID string) (*CloudNode, error) {
	node, err := p.store.Get(nodeID)
	if err != nil {
		return nil, fmt.Errorf("read node %s from store: %w", nodeID, err)
	}
	if node == nil {
		return nil, fmt.Errorf("cloud node %q not found", nodeID)
	}

	// Refresh status from K8s
	podStatus, err := p.k8s.GetNodePod(ctx, node.Namespace, node.PodName)
	if err != nil {
		return node, nil // Return cached info on K8s error
	}

	node.Status = podStatus.Phase
	node.LastSeen = time.Now()
	if putErr := p.store.Put(node); putErr != nil {
		logger.Logger.Warn().Err(putErr).Str("node_id", nodeID).Msg("failed to persist refreshed node status")
	}

	return node, nil
}

// ListNodes returns all provisioned cloud nodes, optionally filtered by org.
// It merges persisted state from BoltDB with live K8s pod status.
func (p *Provisioner) ListNodes(ctx context.Context, org string) ([]*CloudNode, error) {
	selector := "app.kubernetes.io/managed-by=playground-provisioner"
	if org != "" {
		selector += fmt.Sprintf(",playground.hanzo.ai/org=%s", org)
	}

	stored, err := p.store.List()
	if err != nil {
		return nil, fmt.Errorf("list nodes from store: %w", err)
	}

	storeIndex := make(map[string]*CloudNode, len(stored))
	for _, n := range stored {
		storeIndex[n.NodeID] = n
	}

	pods, err := p.k8s.ListAgentPods(ctx, p.config.Kubernetes.Namespace, selector)
	if err != nil {
		var result []*CloudNode
		for _, n := range stored {
			if org == "" || n.Org == org {
				result = append(result, n)
			}
		}
		return result, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	seen := make(map[string]bool)
	var result []*CloudNode
	for _, ps := range pods {
		nodeID := ""
		if strings.HasPrefix(ps.Name, "agent-") {
			nodeID = strings.TrimPrefix(ps.Name, "agent-")
		}
		seen[nodeID] = true
		if existing, ok := storeIndex[nodeID]; ok {
			existing.Status = ps.Phase
			existing.LastSeen = time.Now()
			_ = p.store.Put(existing)
			result = append(result, existing)
		} else {
			node := &CloudNode{
				NodeID:    nodeID,
				PodName:   ps.Name,
				Namespace: ps.Namespace,
				NodeType:  NodeTypeCloud,
				Status:    ps.Phase,
				LastSeen:  time.Now(),
			}
			_ = p.store.Put(node)
			result = append(result, node)
		}
	}

	for _, n := range stored {
		if seen[n.NodeID] {
			continue
		}
		if org != "" && n.Org != org {
			continue
		}
		result = append(result, n)
	}

	return result, nil
}

// GetLogs returns recent logs for a cloud agent pod.
func (p *Provisioner) GetLogs(ctx context.Context, nodeID string, tailLines int64) (string, error) {
	node, err := p.store.Get(nodeID)
	if err != nil {
		return "", fmt.Errorf("read node %s from store: %w", nodeID, err)
	}
	if node == nil {
		return "", fmt.Errorf("cloud node %q not found", nodeID)
	}

	return p.k8s.GetPodLogs(ctx, node.Namespace, node.PodName, tailLines)
}

// Sync refreshes the node list from Kubernetes and rehydrates
// non-K8s nodes (DO droplets, VMs) from Visor.
func (p *Provisioner) Sync(ctx context.Context) error {
	nodes, err := p.ListNodes(ctx, "")
	if err != nil {
		return err
	}

	SyncActiveAgentCount(len(nodes))

	if p.visor != nil {
		if err := p.rehydrateFromVisor(ctx); err != nil {
			logger.Logger.Warn().Err(err).Msg("failed to rehydrate nodes from visor")
		}
		all, listErr := p.store.List()
		if listErr == nil {
			SyncActiveAgentCount(len(all))
		}
	}
	return nil
}

// rehydrateFromVisor loads machines from Visor and adds any that are missing
// from the store. This recovers DO droplets after a restart.
func (p *Provisioner) rehydrateFromVisor(ctx context.Context) error {
	machines, err := p.visor.ListMachines(ctx, "hanzo")
	if err != nil {
		return err
	}

	existing, err := p.store.List()
	if err != nil {
		return fmt.Errorf("list nodes for visor rehydration: %w", err)
	}
	existingByPod := make(map[string]bool, len(existing))
	for _, n := range existing {
		existingByPod[n.PodName] = true
	}

	added := 0
	for _, m := range machines {
		if existingByPod[m.Name] {
			continue
		}

		nodeID := m.Name

		node := &CloudNode{
			NodeID:         nodeID,
			PodName:        m.Name,
			Namespace:      "hanzo",
			NodeType:       NodeTypeCloud,
			Status:         m.State,
			Image:          fmt.Sprintf("droplet:%s", m.Provider),
			OS:             strings.ToLower(m.OS),
			RemoteProtocol: strings.ToLower(m.RemoteProtocol),
			Labels: map[string]string{
				"playground.hanzo.ai/node-id":  nodeID,
				"playground.hanzo.ai/type":     "droplet",
				"playground.hanzo.ai/provider": strings.ToLower(m.Provider),
			},
			LastSeen: time.Now(),
		}
		if m.CreatedTime != "" {
			if t, parseErr := time.Parse("2006-01-02T15:04:05-07:00", m.CreatedTime); parseErr == nil {
				node.CreatedAt = t
			} else if t, parseErr := time.Parse("2006-01-02T15:04:05Z", m.CreatedTime); parseErr == nil {
				node.CreatedAt = t
			}
		}
		if putErr := p.store.Put(node); putErr != nil {
			logger.Logger.Warn().Err(putErr).Str("node_id", nodeID).Msg("failed to persist rehydrated visor node")
			continue
		}
		added++
	}

	if added > 0 {
		logger.Logger.Info().Int("count", added).Msg("rehydrated cloud nodes from visor")
	}
	return nil
}

// countByOrg returns the number of cloud nodes for an organization.
func (p *Provisioner) countByOrg(org string) int {
	nodes, err := p.store.List()
	if err != nil {
		logger.Logger.Warn().Err(err).Msg("failed to list nodes for org count")
		return 0
	}
	count := 0
	for _, n := range nodes {
		if n.Org == org {
			count++
		}
	}
	return count
}

// RunningBillingNodes returns all provisioned nodes that have an active billing
// hold. Used by the metering adapter to report usage for running compute.
func (p *Provisioner) RunningBillingNodes() []*CloudNode {
	nodes, err := p.store.List()
	if err != nil {
		logger.Logger.Warn().Err(err).Msg("failed to list nodes for billing")
		return nil
	}

	var result []*CloudNode
	for _, n := range nodes {
		if n.HoldID == "" || n.CentsPerHour <= 0 || n.BillingUserID == "" {
			continue
		}
		result = append(result, n)
	}
	return result
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
		limitMB = 1024
	}
	heapMB := limitMB - 128
	if heapMB < 256 {
		heapMB = 256
	}
	return fmt.Sprintf("--max-old-space-size=%d", heapMB)
}

// nodeArgs builds the container args to run the bot in node mode.
func nodeArgs(gatewayURL, nodeID string) []string {
	if gatewayURL == "" {
		return []string{
			"node", "hanzo-bot.mjs", "gateway",
			"--allow-unconfigured", "--bind", "lan",
		}
	}
	return []string{
		"node", "hanzo-bot.mjs", "node", "run",
		"--name", nodeID,
	}
}

// agentSecretName returns the K8s Secret name for an agent's sensitive env vars.
func agentSecretName(nodeID string) string {
	return sanitizePodName(fmt.Sprintf("agent-keys-%s", nodeID))
}

// RedactKey returns a redacted version of an API key, showing only the first 6
// characters followed by "...". Safe for logging without leaking secrets.
func RedactKey(key string) string {
	if len(key) <= 6 {
		return "***"
	}
	return key[:6] + "..."
}

// sensitiveEnvKeys lists env var names that contain API keys or tokens and must
// be stored in a K8s Secret rather than inline in the pod spec.
var sensitiveEnvKeys = map[string]bool{
	"HANZO_API_KEY":     true,
	"OPENAI_API_KEY":    true,
	"ANTHROPIC_API_KEY": true,
	"BOT_GATEWAY_TOKEN": true,
}

// splitSensitiveEnv separates env vars into non-sensitive (safe for pod spec)
// and sensitive (must go into a K8s Secret). Keys listed in sensitiveEnvKeys
// are classified as sensitive.
func splitSensitiveEnv(env map[string]string) (safe map[string]string, secret map[string]string) {
	safe = make(map[string]string, len(env))
	secret = make(map[string]string)
	for k, v := range env {
		if sensitiveEnvKeys[k] {
			secret[k] = v
		} else {
			safe[k] = v
		}
	}
	return safe, secret
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
