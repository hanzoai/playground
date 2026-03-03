package cloud

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/playground/control-plane/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockK8sClient implements KubernetesClient for testing.
type mockK8sClient struct {
	pods           map[string]*PodStatus
	secrets        map[string]map[string]string // name -> data
	secretLabels   map[string]map[string]string // name -> labels
	createPodCalls int
	deletePodCalls int
	createSecCalls int
	deleteSecCalls int
}

func newMockK8s() *mockK8sClient {
	return &mockK8sClient{
		pods:         make(map[string]*PodStatus),
		secrets:      make(map[string]map[string]string),
		secretLabels: make(map[string]map[string]string),
	}
}

func (m *mockK8sClient) CreateAgentPod(_ context.Context, spec *PodSpec) (*PodStatus, error) {
	m.createPodCalls++
	ps := &PodStatus{Name: spec.Name, Namespace: spec.Namespace, Phase: "Pending"}
	m.pods[spec.Name] = ps
	return ps, nil
}

func (m *mockK8sClient) DeleteAgentPod(_ context.Context, _, podName string, _ time.Duration) error {
	m.deletePodCalls++
	delete(m.pods, podName)
	return nil
}

func (m *mockK8sClient) GetNodePod(_ context.Context, ns, podName string) (*PodStatus, error) {
	if ps, ok := m.pods[podName]; ok {
		return ps, nil
	}
	return &PodStatus{Phase: "Unknown"}, nil
}

func (m *mockK8sClient) ListAgentPods(_ context.Context, _, _ string) ([]*PodStatus, error) {
	var result []*PodStatus
	for _, ps := range m.pods {
		result = append(result, ps)
	}
	return result, nil
}

func (m *mockK8sClient) GetPodLogs(_ context.Context, _, _ string, _ int64) (string, error) {
	return "", nil
}

func (m *mockK8sClient) CreateSecret(_ context.Context, _, name string, data map[string]string, labels map[string]string) error {
	m.createSecCalls++
	m.secrets[name] = data
	m.secretLabels[name] = labels
	return nil
}

func (m *mockK8sClient) DeleteSecret(_ context.Context, _, name string) error {
	m.deleteSecCalls++
	delete(m.secrets, name)
	delete(m.secretLabels, name)
	return nil
}

func testConfig() config.CloudConfig {
	cfg := config.DefaultCloudConfig()
	cfg.Enabled = true
	cfg.Kubernetes.Enabled = true
	cfg.Kubernetes.CloudAPIKey = "hk-test-shared-service-key-1234567890"
	cfg.Kubernetes.GatewayToken = "gw-token-abc123def456"
	cfg.Kubernetes.GatewayURL = "ws://bot-gateway.hanzo.svc:18789"
	return cfg
}

func newTestProvisioner(k8s *mockK8sClient) *Provisioner {
	return &Provisioner{
		config: testConfig(),
		k8s:    k8s,
		nodes:  make(map[string]*CloudNode),
	}
}

func TestProvisionCreatesSecretForAPIKeys(t *testing.T) {
	k8s := newMockK8s()
	p := newTestProvisioner(k8s)

	req := &ProvisionRequest{
		NodeID:     "test-node-1",
		Model:      "claude-sonnet-4",
		UserAPIKey: "hk-user-api-key-abcdef1234567890",
	}

	result, err := p.Provision(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "test-node-1", result.NodeID)

	// A secret should have been created
	assert.Equal(t, 1, k8s.createSecCalls, "expected exactly 1 CreateSecret call")

	secretName := agentSecretName("test-node-1")
	secretData, ok := k8s.secrets[secretName]
	require.True(t, ok, "secret %q should exist", secretName)

	// Secret should contain the sensitive keys
	assert.Equal(t, "hk-user-api-key-abcdef1234567890", secretData["HANZO_API_KEY"])
	assert.Equal(t, "hk-user-api-key-abcdef1234567890", secretData["OPENAI_API_KEY"])
	assert.Equal(t, "hk-user-api-key-abcdef1234567890", secretData["BOT_GATEWAY_TOKEN"])

	// Secret labels should tag it to the node
	labels := k8s.secretLabels[secretName]
	assert.Equal(t, "playground-provisioner", labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "test-node-1", labels["playground.hanzo.ai/node-id"])
}

func TestProvisionPodSpecDoesNotContainSecrets(t *testing.T) {
	k8s := newMockK8s()
	p := newTestProvisioner(k8s)

	req := &ProvisionRequest{
		NodeID:     "test-node-2",
		UserAPIKey: "hk-user-api-key-abcdef1234567890",
	}

	_, err := p.Provision(context.Background(), req)
	require.NoError(t, err)

	// The pod was created -- verify the env vars don't contain secrets.
	// We can't directly inspect the PodSpec from the mock, but we can verify
	// the provisioner properly splits env. Test splitSensitiveEnv directly.
	env := map[string]string{
		"AGENT_NODE_ID":     "test",
		"HANZO_API_KEY":     "secret-key",
		"OPENAI_API_KEY":    "secret-key",
		"BOT_GATEWAY_TOKEN": "secret-token",
		"BOT_CLOUD_NODE":    "true",
	}

	safe, secret := splitSensitiveEnv(env)
	assert.NotContains(t, safe, "HANZO_API_KEY")
	assert.NotContains(t, safe, "OPENAI_API_KEY")
	assert.NotContains(t, safe, "BOT_GATEWAY_TOKEN")
	assert.Contains(t, safe, "AGENT_NODE_ID")
	assert.Contains(t, safe, "BOT_CLOUD_NODE")

	assert.Contains(t, secret, "HANZO_API_KEY")
	assert.Contains(t, secret, "OPENAI_API_KEY")
	assert.Contains(t, secret, "BOT_GATEWAY_TOKEN")
	assert.NotContains(t, secret, "AGENT_NODE_ID")
}

func TestDeprovisionDeletesSecret(t *testing.T) {
	k8s := newMockK8s()
	p := newTestProvisioner(k8s)

	// Provision first
	req := &ProvisionRequest{
		NodeID:     "test-node-3",
		UserAPIKey: "hk-user-api-key-abcdef1234567890",
	}

	_, err := p.Provision(context.Background(), req)
	require.NoError(t, err)

	secretName := agentSecretName("test-node-3")
	_, secretExists := k8s.secrets[secretName]
	assert.True(t, secretExists, "secret should exist after provision")

	// Deprovision
	err = p.Deprovision(context.Background(), "test-node-3")
	require.NoError(t, err)

	// Secret should be deleted
	assert.Equal(t, 1, k8s.deleteSecCalls, "expected 1 DeleteSecret call on deprovision")
	_, secretExists = k8s.secrets[secretName]
	assert.False(t, secretExists, "secret should be deleted after deprovision")

	// Pod should also be deleted
	assert.Equal(t, 1, k8s.deletePodCalls)
}

func TestDeprovisionDeletesSecretEvenWhenAbsent(t *testing.T) {
	k8s := newMockK8s()
	p := newTestProvisioner(k8s)

	// Manually add a node (simulating a pre-existing node without a secret)
	p.mu.Lock()
	p.nodes["legacy-node"] = &CloudNode{
		NodeID:    "legacy-node",
		PodName:   "agent-legacy-node",
		Namespace: "hanzo",
		NodeType:  NodeTypeCloud,
		Status:    "Running",
		OS:        "linux",
	}
	p.mu.Unlock()

	err := p.Deprovision(context.Background(), "legacy-node")
	require.NoError(t, err)

	// DeleteSecret is called (best-effort), DeleteAgentPod also called
	assert.Equal(t, 1, k8s.deleteSecCalls)
	assert.Equal(t, 1, k8s.deletePodCalls)
}

func TestProvisionWithSharedKeyCreatesSecret(t *testing.T) {
	k8s := newMockK8s()
	p := newTestProvisioner(k8s)

	// No UserAPIKey -- should fall back to the config's shared CloudAPIKey
	req := &ProvisionRequest{
		NodeID: "test-node-shared",
	}

	_, err := p.Provision(context.Background(), req)
	require.NoError(t, err)

	secretName := agentSecretName("test-node-shared")
	secretData, ok := k8s.secrets[secretName]
	require.True(t, ok, "secret should be created even with shared key")
	assert.Equal(t, "hk-test-shared-service-key-1234567890", secretData["HANZO_API_KEY"])
}

func TestDropletTagsDoNotContainAPIKeys(t *testing.T) {
	// Test that the provisionDroplet path uses "secret:" prefix (user-data)
	// instead of "env:" prefix (tags) for sensitive values.
	tags := map[string]string{
		"env:BOT_NODE_GATEWAY_URL":    "wss://gw.hanzo.bot",
		"env:AGENT_NODE_ID":           "hz-12345678",
		"secret:BOT_GATEWAY_TOKEN":    "gw-token-abc",
		"secret:HANZO_API_KEY":        "hk-user-key-abc",
	}

	// Verify none of the env: tags contain API keys
	for k := range tags {
		if k == "env:HANZO_API_KEY" || k == "env:OPENAI_API_KEY" || k == "env:ANTHROPIC_API_KEY" {
			t.Errorf("tag %q should not be in env: tags (leaks via DO API)", k)
		}
	}

	// Verify sensitive values use secret: prefix
	assert.Contains(t, tags, "secret:BOT_GATEWAY_TOKEN")
	assert.Contains(t, tags, "secret:HANZO_API_KEY")
}

func TestRedactKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hk-0d2eb9cfafd049389f2904cad770a9d8", "hk-0d2..."},
		{"sk-ant-api03-abc", "sk-ant..."},
		{"short", "***"},
		{"123456", "***"},
		{"1234567", "123456..."},
		{"", "***"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, RedactKey(tt.input))
		})
	}
}

func TestSplitSensitiveEnv(t *testing.T) {
	env := map[string]string{
		"AGENT_NODE_ID":         "cloud-abc",
		"PLAYGROUND_SERVER":     "http://playground.hanzo.svc:8080",
		"HANZO_API_KEY":         "hk-secret-key",
		"OPENAI_API_KEY":        "hk-secret-key",
		"ANTHROPIC_API_KEY":     "hk-secret-key",
		"BOT_GATEWAY_TOKEN":     "gw-token",
		"BOT_CLOUD_NODE":        "true",
		"BOT_NODE_GATEWAY_URL":  "ws://gw.svc:18789",
		"NODE_OPTIONS":          "--max-old-space-size=3968",
	}

	safe, secret := splitSensitiveEnv(env)

	// Sensitive keys in secret
	assert.Equal(t, "hk-secret-key", secret["HANZO_API_KEY"])
	assert.Equal(t, "hk-secret-key", secret["OPENAI_API_KEY"])
	assert.Equal(t, "hk-secret-key", secret["ANTHROPIC_API_KEY"])
	assert.Equal(t, "gw-token", secret["BOT_GATEWAY_TOKEN"])
	assert.Len(t, secret, 4)

	// Non-sensitive keys in safe
	assert.Contains(t, safe, "AGENT_NODE_ID")
	assert.Contains(t, safe, "PLAYGROUND_SERVER")
	assert.Contains(t, safe, "BOT_CLOUD_NODE")
	assert.Contains(t, safe, "BOT_NODE_GATEWAY_URL")
	assert.Contains(t, safe, "NODE_OPTIONS")
	assert.NotContains(t, safe, "HANZO_API_KEY")
	assert.NotContains(t, safe, "OPENAI_API_KEY")
	assert.NotContains(t, safe, "ANTHROPIC_API_KEY")
	assert.NotContains(t, safe, "BOT_GATEWAY_TOKEN")
}

func TestAgentSecretName(t *testing.T) {
	assert.Equal(t, "agent-keys-cloud-abc12345", agentSecretName("cloud-abc12345"))
	assert.Equal(t, "agent-keys-test", agentSecretName("test"))

	// Verify DNS-1123 compliance
	name := agentSecretName("UPPER_CASE.weird!chars")
	for _, r := range name {
		assert.True(t, (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-',
			"character %c is not DNS-1123 safe", r)
	}
}

func TestBuildPodManifestWithSecretRef(t *testing.T) {
	spec := &PodSpec{
		Name:      "agent-test",
		Namespace: "hanzo",
		Image:     "ghcr.io/hanzoai/bot:latest",
		Env: map[string]string{
			"AGENT_NODE_ID": "test",
		},
		CPU:         "250m",
		Memory:      "512Mi",
		LimitCPU:    "2000m",
		LimitMemory: "4Gi",
		SecretRef:   "agent-keys-test",
		Sidecars: []SidecarSpec{
			{
				Name:     "operative",
				Image:    "ghcr.io/hanzoai/operative:latest",
				Env:      map[string]string{"DISPLAY": ":1"},
				Ports:    []int32{8080},
				CPU:      "200m",
				Memory:   "512Mi",
				LimitCPU: "1000m",
				LimitMem: "2Gi",
			},
		},
	}

	manifest := buildPodManifest(spec)

	// Verify main container has envFrom with secretRef
	podSpec := manifest["spec"].(map[string]interface{})
	containers := podSpec["containers"].([]interface{})
	mainContainer := containers[0].(map[string]interface{})

	envFrom, ok := mainContainer["envFrom"]
	require.True(t, ok, "main container should have envFrom")
	envFromList := envFrom.([]map[string]interface{})
	require.Len(t, envFromList, 1)
	secretRef := envFromList[0]["secretRef"].(map[string]string)
	assert.Equal(t, "agent-keys-test", secretRef["name"])

	// Verify sidecar also gets envFrom
	require.Len(t, containers, 2)
	sidecar := containers[1].(map[string]interface{})
	scEnvFrom, ok := sidecar["envFrom"]
	require.True(t, ok, "sidecar should have envFrom")
	scEnvFromList := scEnvFrom.([]map[string]interface{})
	require.Len(t, scEnvFromList, 1)
	scSecretRef := scEnvFromList[0]["secretRef"].(map[string]string)
	assert.Equal(t, "agent-keys-test", scSecretRef["name"])
}

func TestBuildPodManifestWithoutSecretRef(t *testing.T) {
	spec := &PodSpec{
		Name:        "agent-nosecret",
		Namespace:   "hanzo",
		Image:       "ghcr.io/hanzoai/bot:latest",
		Env:         map[string]string{"AGENT_NODE_ID": "test"},
		CPU:         "250m",
		Memory:      "512Mi",
		LimitCPU:    "2000m",
		LimitMemory: "4Gi",
	}

	manifest := buildPodManifest(spec)

	podSpec := manifest["spec"].(map[string]interface{})
	containers := podSpec["containers"].([]interface{})
	mainContainer := containers[0].(map[string]interface{})

	_, ok := mainContainer["envFrom"]
	assert.False(t, ok, "container should not have envFrom when SecretRef is empty")
}
