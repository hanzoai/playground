package orchestrator

import (
	"github.com/hanzoai/playground/internal/events"
	"github.com/hanzoai/playground/internal/gitops"
	"github.com/hanzoai/playground/internal/gossip"
	"github.com/hanzoai/playground/internal/policy"
	"github.com/hanzoai/playground/internal/zap"
)

// Orchestrator is the top-level coordinator that owns the lifecycle manager
// and provides the interface for handlers.
type Orchestrator struct {
	Lifecycle *BotLifecycle
	Presets   map[string]Preset
}

// New creates an Orchestrator wired to all subsystems.
func New(
	zapPool *zap.Pool,
	tracker *gossip.Tracker,
	router *gossip.Router,
	eventBus *events.AgentEventBus,
	gitManager *gitops.Manager,
	policyEngine *policy.Engine,
	cfg ...BotLifecycleConfig,
) *Orchestrator {
	return &Orchestrator{
		Lifecycle: NewBotLifecycle(zapPool, tracker, router, eventBus, gitManager, policyEngine, cfg...),
		Presets:   BuiltinPresets(),
	}
}
