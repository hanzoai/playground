package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/playground/internal/events"
	"github.com/hanzoai/playground/internal/gitops"
	"github.com/hanzoai/playground/internal/gossip"
	"github.com/hanzoai/playground/internal/policy"
	"github.com/hanzoai/playground/internal/zap"
)

// ---------------------------------------------------------------------------
// Preset tests
// ---------------------------------------------------------------------------

func TestBuiltinPresetsContainsAllExpectedKeys(t *testing.T) {
	presets := BuiltinPresets()

	expected := []string{
		"cto", "senior", "junior", "intern",
		"vision", "marketing", "sales", "design",
		"devops", "security",
	}
	for _, name := range expected {
		_, ok := presets[name]
		assert.True(t, ok, "missing preset: %s", name)
	}
	assert.Equal(t, len(expected), len(presets))
}

func TestGetPresetKnownName(t *testing.T) {
	p := GetPreset("cto")
	assert.Equal(t, "cto", p.Name)
	assert.Equal(t, "opus", p.Model)
	assert.Equal(t, "never", p.ApprovalPolicy)
	assert.Equal(t, "danger-full-access", p.SandboxMode)
}

func TestGetPresetUnknownFallsBackToJunior(t *testing.T) {
	p := GetPreset("nonexistent")
	assert.Equal(t, "junior", p.Name)
	assert.Equal(t, "haiku", p.Model)
}

func TestApplyPresetFillsDefaults(t *testing.T) {
	opts := SpawnOpts{
		BotID:   "b1",
		SpaceID: "s1",
	}
	result := ApplyPreset("senior", opts)

	assert.Equal(t, "sonnet", result.Model)
	assert.Equal(t, "untrusted", result.ApprovalPolicy)
	assert.Equal(t, "workspace-write", result.SandboxMode)
	assert.Equal(t, "Senior Engineer", result.Name)
	assert.NotEmpty(t, result.Capabilities)
}

func TestApplyPresetExplicitOverridesTakePrecedence(t *testing.T) {
	opts := SpawnOpts{
		BotID:          "b1",
		SpaceID:        "s1",
		Model:          "custom-model",
		ApprovalPolicy: "never",
		Name:           "Custom Bot",
	}
	result := ApplyPreset("senior", opts)

	// Explicit values preserved.
	assert.Equal(t, "custom-model", result.Model)
	assert.Equal(t, "never", result.ApprovalPolicy)
	assert.Equal(t, "Custom Bot", result.Name)
	// Preset defaults fill gaps.
	assert.Equal(t, "workspace-write", result.SandboxMode)
}

// ---------------------------------------------------------------------------
// Orchestrator construction
// ---------------------------------------------------------------------------

func TestNewOrchestratorWiresSubsystems(t *testing.T) {
	pool := zap.NewPool()
	tracker := gossip.NewTracker()
	router := gossip.NewRouter(tracker)
	bus := events.NewAgentEventBus()
	gm := gitops.NewManager(t.TempDir())
	pe := policy.NewEngine()

	o := New(pool, tracker, router, bus, gm, pe)

	require.NotNil(t, o)
	require.NotNil(t, o.Lifecycle)
	assert.Len(t, o.Presets, 10)
}

// ---------------------------------------------------------------------------
// Lifecycle tests (unit-level, no real sidecar process)
// ---------------------------------------------------------------------------

func newTestLifecycle(t *testing.T) (*BotLifecycle, *zap.Pool, *gossip.Tracker, *gossip.Router, *events.AgentEventBus, *policy.Engine) {
	t.Helper()
	pool := zap.NewPool()
	tracker := gossip.NewTracker()
	router := gossip.NewRouter(tracker)
	bus := events.NewAgentEventBus()
	gm := gitops.NewManager(t.TempDir())
	pe := policy.NewEngine()

	// Set up a bypass policy so spawns are allowed.
	pe.SetSpacePolicy(policy.BypassPolicy("test-space"))

	lc := NewBotLifecycle(pool, tracker, router, bus, gm, pe)
	return lc, pool, tracker, router, bus, pe
}

func TestSpawnBotMissingSpaceID(t *testing.T) {
	lc, _, _, _, _, _ := newTestLifecycle(t)
	err := lc.SpawnBot(t.Context(), SpawnOpts{BotID: "b1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "space_id is required")
}

func TestSpawnBotPolicyDenied(t *testing.T) {
	pool := zap.NewPool()
	tracker := gossip.NewTracker()
	router := gossip.NewRouter(tracker)
	bus := events.NewAgentEventBus()
	gm := gitops.NewManager(t.TempDir())
	pe := policy.NewEngine()

	// No policy set -- spawn should be denied.
	lc := NewBotLifecycle(pool, tracker, router, bus, gm, pe)

	err := lc.SpawnBot(t.Context(), SpawnOpts{
		BotID:   "b1",
		SpaceID: "no-policy-space",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy denied")
}

func TestActiveBots(t *testing.T) {
	lc, _, tracker, _, _, _ := newTestLifecycle(t)

	// Manually register agents to test ActiveBots without a real sidecar.
	_ = tracker.Register(gossip.AgentInfo{
		AgentID:     "a1",
		DID:         "did:hanzo:bot:a1",
		SpaceID:     "test-space",
		DisplayName: "Bot A",
		Status:      "online",
	})
	_ = tracker.Register(gossip.AgentInfo{
		AgentID:     "a2",
		DID:         "did:hanzo:bot:a2",
		SpaceID:     "test-space",
		DisplayName: "Bot B",
		Status:      "offline",
	})
	_ = tracker.Register(gossip.AgentInfo{
		AgentID:     "a3",
		DID:         "did:hanzo:bot:a3",
		SpaceID:     "test-space",
		DisplayName: "Bot C",
		Status:      "busy",
	})

	bots := lc.ActiveBots("test-space")
	// Should include a1 (online) and a3 (busy), not a2 (offline).
	assert.Len(t, bots, 2)
	assert.Contains(t, bots, "a1")
	assert.Contains(t, bots, "a3")
}

func TestActiveBotsEmptySpace(t *testing.T) {
	lc, _, _, _, _, _ := newTestLifecycle(t)
	bots := lc.ActiveBots("empty-space")
	assert.Empty(t, bots)
}

func TestInjectMessageNoSidecar(t *testing.T) {
	lc, _, _, _, _, _ := newTestLifecycle(t)
	err := lc.InjectMessage(t.Context(), "test-space", "missing-bot", "hello", "human")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active sidecar")
}

func TestBroadcastMessageNoBotsInSpace(t *testing.T) {
	lc, _, _, _, _, _ := newTestLifecycle(t)
	err := lc.BroadcastMessage(t.Context(), "empty-space", "hello", "human")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active bots")
}

func TestStopBotCleanup(t *testing.T) {
	lc, _, tracker, router, bus, _ := newTestLifecycle(t)

	// Register a bot manually in tracker and router.
	_ = tracker.Register(gossip.AgentInfo{
		AgentID:     "stop-bot",
		DID:         "did:hanzo:bot:stop-bot",
		SpaceID:     "test-space",
		DisplayName: "Stop Bot",
		Status:      "online",
	})
	router.Subscribe("stop-bot")

	// Add a pump cancel func.
	lc.mu.Lock()
	called := false
	lc.pumps["stop-bot"] = func() { called = true }
	lc.mu.Unlock()

	// Subscribe to bus to catch the leave event.
	ch, unsub := bus.Subscribe("test-space")
	defer unsub()

	err := lc.StopBot(t.Context(), "test-space", "stop-bot")
	require.NoError(t, err)

	// Pump cancel was called.
	assert.True(t, called)

	// Agent unregistered from tracker.
	_, found := tracker.Get("stop-bot")
	assert.False(t, found)

	// Router unsubscribed.
	assert.Equal(t, 0, router.SubscriberCount())

	// AgentLeftSpace event published.
	select {
	case evt := <-ch:
		assert.Equal(t, events.AgentLeftSpace, evt.Type)
		assert.Equal(t, "stop-bot", evt.AgentID)
	default:
		t.Fatal("expected AgentLeftSpace event")
	}
}

// ---------------------------------------------------------------------------
// Event pump mapper tests
// ---------------------------------------------------------------------------

func TestMapZAPEventKnownTypes(t *testing.T) {
	cases := []struct {
		zapType       string
		expectedAgent events.AgentEventType
	}{
		{zap.EventTurnStarted, events.AgentTurnStarted},
		{zap.EventTurnComplete, events.AgentTurnCompleted},
		{zap.EventAgentMessage, events.AgentMessage},
		{zap.EventAgentMessageDelta, events.AgentMessageDelta},
		{zap.EventExecCommandBegin, events.AgentExecBegin},
		{zap.EventExecCommandEnd, events.AgentExecEnd},
		{zap.EventMcpToolCallBegin, events.AgentToolCallBegin},
		{zap.EventMcpToolCallEnd, events.AgentToolCallEnd},
	}

	for _, tc := range cases {
		t.Run(tc.zapType, func(t *testing.T) {
			msg := zap.EventMsg{Type: tc.zapType}
			result := mapZAPEvent(msg, "s1", "b1", "Bot1")
			require.NotNil(t, result)
			assert.Equal(t, tc.expectedAgent, result.Type)
			assert.Equal(t, "s1", result.SpaceID)
			assert.Equal(t, "b1", result.AgentID)
			assert.Equal(t, "Bot1", result.AgentName)
		})
	}
}

func TestMapZAPEventUnknownTypeReturnsNil(t *testing.T) {
	msg := zap.EventMsg{Type: "some_unknown_event"}
	result := mapZAPEvent(msg, "s1", "b1", "Bot1")
	assert.Nil(t, result)
}

func TestMapZAPEventWithRawData(t *testing.T) {
	raw := []byte(`{"message":"hello world","type":"agent_message"}`)
	msg := zap.EventMsg{Type: zap.EventAgentMessage, Raw: raw}
	result := mapZAPEvent(msg, "s1", "b1", "Bot1")
	require.NotNil(t, result)
	assert.Equal(t, "hello world", result.Data["message"])
}

// ---------------------------------------------------------------------------
// buildSandboxPolicy tests
// ---------------------------------------------------------------------------

func TestBuildSandboxPolicy(t *testing.T) {
	cases := []struct {
		mode     string
		expected string
	}{
		{"danger-full-access", zap.SandboxDangerFullAccess},
		{"read-only", zap.SandboxReadOnly},
		{"workspace-write", zap.SandboxWorkspaceWrite},
		{"unknown", zap.SandboxWorkspaceWrite}, // default
		{"", zap.SandboxWorkspaceWrite},         // default
	}

	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			sp := buildSandboxPolicy(tc.mode)
			assert.Equal(t, tc.expected, sp.Type)
		})
	}
}

// ---------------------------------------------------------------------------
// Runtime auth tests
// ---------------------------------------------------------------------------

// mockSecretsGetter is a fake KMS client for testing.
type mockSecretsGetter struct {
	secrets map[string]map[string]string // orgID -> key -> value
}

func (m *mockSecretsGetter) GetSecret(_ context.Context, orgID, key string) ([]byte, error) {
	if m.secrets == nil {
		return nil, nil
	}
	if org, ok := m.secrets[orgID]; ok {
		if v, ok := org[key]; ok {
			return []byte(v), nil
		}
	}
	return nil, nil
}

func newMinimalLifecycle(apiKey string, secrets SecretsGetter) *BotLifecycle {
	var cfg []BotLifecycleConfig
	if apiKey != "" || secrets != nil {
		cfg = append(cfg, BotLifecycleConfig{
			ServerAPIKey:  apiKey,
			SecretsClient: secrets,
		})
	}
	return NewBotLifecycle(
		zap.NewPool(), gossip.NewTracker(), gossip.NewRouter(gossip.NewTracker()),
		events.NewAgentEventBus(), nil, policy.NewEngine(),
		cfg...,
	)
}

func TestBuildRuntimeEnvHanzoDevDefault(t *testing.T) {
	lc := newMinimalLifecycle("sk-test-hanzo-key", nil)
	env := lc.buildRuntimeEnv(context.Background(), SpawnOpts{SpaceID: "s1"})

	assert.Contains(t, env, "AGENT_RUNTIME=hanzo-dev")
	assert.Contains(t, env, "HANZO_API_KEY=sk-test-hanzo-key")
}

func TestBuildRuntimeEnvHanzoDevExplicit(t *testing.T) {
	lc := newMinimalLifecycle("sk-hanzo", nil)
	env := lc.buildRuntimeEnv(context.Background(), SpawnOpts{SpaceID: "s1", Runtime: "hanzo-dev"})

	assert.Contains(t, env, "AGENT_RUNTIME=hanzo-dev")
	assert.Contains(t, env, "HANZO_API_KEY=sk-hanzo")
}

func TestBuildRuntimeEnvHanzoDevNoKey(t *testing.T) {
	lc := newMinimalLifecycle("", nil)
	env := lc.buildRuntimeEnv(context.Background(), SpawnOpts{SpaceID: "s1", Runtime: "hanzo-dev"})

	assert.Contains(t, env, "AGENT_RUNTIME=hanzo-dev")
	assert.Len(t, env, 1, "should only have AGENT_RUNTIME when no API key is set")
}

func TestBuildRuntimeEnvClaude(t *testing.T) {
	mock := &mockSecretsGetter{secrets: map[string]map[string]string{
		"space-1": {"ANTHROPIC_API_KEY": "sk-ant-test"},
	}}
	lc := newMinimalLifecycle("", mock)
	env := lc.buildRuntimeEnv(context.Background(), SpawnOpts{SpaceID: "space-1", Runtime: "claude"})

	assert.Contains(t, env, "AGENT_RUNTIME=claude")
	assert.Contains(t, env, "ANTHROPIC_API_KEY=sk-ant-test")
}

func TestBuildRuntimeEnvGemini(t *testing.T) {
	mock := &mockSecretsGetter{secrets: map[string]map[string]string{
		"space-2": {"GOOGLE_API_KEY": "goog-test"},
	}}
	lc := newMinimalLifecycle("", mock)
	env := lc.buildRuntimeEnv(context.Background(), SpawnOpts{SpaceID: "space-2", Runtime: "gemini"})

	assert.Contains(t, env, "AGENT_RUNTIME=gemini")
	assert.Contains(t, env, "GOOGLE_API_KEY=goog-test")
}

func TestBuildRuntimeEnvOpenAI(t *testing.T) {
	mock := &mockSecretsGetter{secrets: map[string]map[string]string{
		"s3": {"OPENAI_API_KEY": "sk-oai-test"},
	}}
	lc := newMinimalLifecycle("", mock)
	env := lc.buildRuntimeEnv(context.Background(), SpawnOpts{SpaceID: "s3", Runtime: "openai"})

	assert.Contains(t, env, "AGENT_RUNTIME=openai")
	assert.Contains(t, env, "OPENAI_API_KEY=sk-oai-test")
}

func TestBuildRuntimeEnvUnknownRuntime(t *testing.T) {
	mock := &mockSecretsGetter{secrets: map[string]map[string]string{
		"s4": {"CUSTOM_BOT_API_KEY": "custom-key"},
	}}
	lc := newMinimalLifecycle("", mock)
	env := lc.buildRuntimeEnv(context.Background(), SpawnOpts{SpaceID: "s4", Runtime: "custom-bot"})

	assert.Contains(t, env, "AGENT_RUNTIME=custom-bot")
	assert.Contains(t, env, "CUSTOM_BOT_API_KEY=custom-key")
}

func TestBuildRuntimeEnvNoSecretsClient(t *testing.T) {
	lc := newMinimalLifecycle("", nil)
	env := lc.buildRuntimeEnv(context.Background(), SpawnOpts{SpaceID: "s1", Runtime: "claude"})

	assert.Contains(t, env, "AGENT_RUNTIME=claude")
	assert.Len(t, env, 1, "should only have AGENT_RUNTIME when no secrets client")
}

func TestApplyPresetSetsRuntime(t *testing.T) {
	result := ApplyPreset("senior", SpawnOpts{BotID: "b1", SpaceID: "s1"})
	assert.Equal(t, "hanzo-dev", result.Runtime)
}

func TestApplyPresetExplicitRuntimeOverride(t *testing.T) {
	result := ApplyPreset("senior", SpawnOpts{BotID: "b1", SpaceID: "s1", Runtime: "claude"})
	assert.Equal(t, "claude", result.Runtime)
}

func TestAllPresetsHaveHanzoDevRuntime(t *testing.T) {
	presets := BuiltinPresets()
	for name, p := range presets {
		assert.Equal(t, "hanzo-dev", p.Runtime, "preset %q should have hanzo-dev runtime", name)
	}
}
