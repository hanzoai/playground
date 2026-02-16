// Package cloud provides cloud agent provisioning on Kubernetes.
// It creates and manages agent pods in the org's DOKS cluster,
// providing full parity between local bots and cloud nodes.
package cloud

import (
	"context"
	"fmt"
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
	NodeID      string            `json:"node_id"`
	PodName     string            `json:"pod_name"`
	Namespace   string            `json:"namespace"`
	NodeType    NodeType          `json:"node_type"`
	Status      string            `json:"status"`
	Image       string            `json:"image"`
	Endpoint    string            `json:"endpoint"`
	Owner       string            `json:"owner"`
	Org         string            `json:"org"`
	Labels      map[string]string `json:"labels"`
	CreatedAt   time.Time         `json:"created_at"`
	LastSeen    time.Time         `json:"last_seen"`
}

// Provisioner manages cloud agent lifecycle on Kubernetes.
type Provisioner struct {
	config  config.CloudConfig
	k8s     KubernetesClient
	mu      sync.RWMutex
	nodes   map[string]*CloudNode // nodeID -> node
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
	return &Provisioner{
		config: cfg,
		k8s:    k8sClient,
		nodes:  make(map[string]*CloudNode),
	}
}

// Provision creates a new cloud agent node in the DOKS cluster.
func (p *Provisioner) Provision(ctx context.Context, req *ProvisionRequest) (*ProvisionResult, error) {
	if !p.config.Enabled || !p.config.Kubernetes.Enabled {
		return nil, fmt.Errorf("cloud provisioning is not enabled")
	}

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

	// Resource defaults
	cpu := p.config.Kubernetes.DefaultCPU
	memory := p.config.Kubernetes.DefaultMemory
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
	for k, v := range req.Env {
		env[k] = v
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
		CPU:         cpu,
		Memory:      memory,
		LimitCPU:    p.config.Kubernetes.LimitCPU,
		LimitMemory: p.config.Kubernetes.LimitMemory,
		Args:        []string{}, // Agent image entrypoint handles startup
		ControlPlaneURL: fmt.Sprintf("http://hanzo-playground.%s.svc:8080", namespace),
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

	node := &CloudNode{
		NodeID:    nodeID,
		PodName:   podName,
		Namespace: namespace,
		NodeType:  NodeTypeCloud,
		Status:    podStatus.Phase,
		Image:     image,
		Endpoint:  fmt.Sprintf("http://%s.%s.svc:8001", podName, namespace),
		Owner:     req.Owner,
		Org:       req.Org,
		Labels:    labels,
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
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

// Deprovision removes a cloud agent from the cluster.
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
		Msg("deprovisioning cloud agent")

	if err := p.k8s.DeleteAgentPod(ctx, node.Namespace, node.PodName, p.config.Kubernetes.GracefulShutdown); err != nil {
		return fmt.Errorf("failed to delete agent pod: %w", err)
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
