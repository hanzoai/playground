package orchestrator

import (
	"github.com/hanzoai/playground/control-plane/internal/events"
	"github.com/hanzoai/playground/control-plane/internal/gitops"
	"github.com/hanzoai/playground/control-plane/internal/gossip"
	"github.com/hanzoai/playground/control-plane/internal/policy"
	"github.com/hanzoai/playground/control-plane/internal/zap"
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
) *Orchestrator {
	return &Orchestrator{
		Lifecycle: NewBotLifecycle(zapPool, tracker, router, eventBus, gitManager, policyEngine),
		Presets:   BuiltinPresets(),
	}
}
