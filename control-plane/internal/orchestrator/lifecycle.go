package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hanzoai/playground/control-plane/internal/events"
	"github.com/hanzoai/playground/control-plane/internal/gitops"
	"github.com/hanzoai/playground/control-plane/internal/gossip"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/hanzoai/playground/control-plane/internal/policy"
	"github.com/hanzoai/playground/control-plane/internal/zap"
)

// SpawnOpts contains everything needed to bring a bot online.
type SpawnOpts struct {
	BotID          string
	SpaceID        string
	Name           string
	Model          string
	Preset         string // "cto", "senior", "junior", "intern", ""
	ApprovalPolicy string // maps to zap.AskForApproval
	SandboxMode    string // maps to zap.SandboxPolicy type
	Personality    string // system prompt
	Capabilities   []string
	Emoji          string
	Color          string
}

// BotLifecycle manages the complete lifecycle of a bot in a space.
// It connects: ZAP sidecar pool, gossip tracker/router, event bus, git manager, policy engine.
type BotLifecycle struct {
	zapPool      *zap.Pool
	tracker      *gossip.Tracker
	router       *gossip.Router
	eventBus     *events.AgentEventBus
	gitManager   *gitops.Manager
	policyEngine *policy.Engine
	pumps        map[string]context.CancelFunc // botID -> cancel for event pump
	mu           sync.Mutex
}

// NewBotLifecycle creates a BotLifecycle wiring all subsystems together.
func NewBotLifecycle(
	zapPool *zap.Pool,
	tracker *gossip.Tracker,
	router *gossip.Router,
	eventBus *events.AgentEventBus,
	gitManager *gitops.Manager,
	policyEngine *policy.Engine,
) *BotLifecycle {
	return &BotLifecycle{
		zapPool:      zapPool,
		tracker:      tracker,
		router:       router,
		eventBus:     eventBus,
		gitManager:   gitManager,
		policyEngine: policyEngine,
		pumps:        make(map[string]context.CancelFunc),
	}
}

// SpawnBot is the master function that brings a bot online in a space.
// It:
//  1. Applies preset defaults
//  2. Checks policy engine for permission to create bot
//  3. Spawns ZAP sidecar via pool
//  4. Registers agent in gossip tracker
//  5. Subscribes agent to gossip router
//  6. Creates agent git branch in space repo
//  7. Starts event pump goroutine (sidecar events -> event bus)
//  8. Publishes AgentJoinedSpace event
func (l *BotLifecycle) SpawnBot(ctx context.Context, opts SpawnOpts) error {
	// Apply preset defaults for any unset fields.
	if opts.Preset != "" {
		opts = ApplyPreset(opts.Preset, opts)
	}

	if opts.BotID == "" {
		opts.BotID = uuid.New().String()
	}
	if opts.SpaceID == "" {
		return fmt.Errorf("orchestrator: space_id is required")
	}
	if opts.Name == "" {
		suffix := opts.BotID
		if len(suffix) > 8 {
			suffix = suffix[:8]
		}
		opts.Name = "bot-" + suffix
	}

	// 1. Check policy engine for permission.
	allowed, requiresApproval, reason := l.policyEngine.Check(
		opts.BotID, opts.SpaceID,
		policy.ResourceShell, policy.PermExecute,
	)
	if !allowed && !requiresApproval {
		return fmt.Errorf("orchestrator: policy denied spawn: %s", reason)
	}
	if requiresApproval {
		reqID := l.policyEngine.RequestApproval(policy.ApprovalRequest{
			BotID:       opts.BotID,
			SpaceID:     opts.SpaceID,
			Resource:    policy.ResourceShell,
			Permission:  policy.PermExecute,
			Description: fmt.Sprintf("Spawn bot %s (%s) in space %s", opts.Name, opts.BotID, opts.SpaceID),
		})
		logger.Logger.Info().
			Str("approval_id", reqID).
			Str("bot_id", opts.BotID).
			Msg("[orchestrator] spawn requires approval")
	}

	// 2. Spawn ZAP sidecar.
	sidecarOpts := zap.SidecarOpts{
		SpaceID:        opts.SpaceID,
		BotID:          opts.BotID,
		Model:          opts.Model,
		ApprovalPolicy: zap.AskForApproval(opts.ApprovalPolicy),
		Sandbox:        buildSandboxPolicy(opts.SandboxMode),
	}

	// Set working directory to the space repo path if git manager is available.
	if l.gitManager != nil {
		repo, err := l.gitManager.GetOrInit(opts.SpaceID)
		if err != nil {
			return fmt.Errorf("orchestrator: git init for space %s: %w", opts.SpaceID, err)
		}
		sidecarOpts.Cwd = repo.Path()
	}

	sidecar, err := l.zapPool.Spawn(ctx, opts.BotID, sidecarOpts)
	if err != nil {
		return fmt.Errorf("orchestrator: spawn sidecar: %w", err)
	}

	// 3. Register in gossip tracker.
	caps := make([]gossip.AgentCapability, len(opts.Capabilities))
	for i, c := range opts.Capabilities {
		caps[i] = gossip.AgentCapability{Name: c}
	}

	agentInfo := gossip.AgentInfo{
		AgentID:      opts.BotID,
		DID:          "did:hanzo:bot:" + opts.BotID,
		SpaceID:      opts.SpaceID,
		DisplayName:  opts.Name,
		Status:       "online",
		Capabilities: caps,
		Model:        opts.Model,
		JoinedAt:     time.Now(),
	}
	if err := l.tracker.Register(agentInfo); err != nil {
		// Cleanup: remove sidecar on registration failure.
		_ = l.zapPool.Remove(opts.BotID)
		return fmt.Errorf("orchestrator: register in tracker: %w", err)
	}

	// 4. Subscribe to gossip router.
	l.router.Subscribe(opts.BotID)

	// 5. Create agent git branch.
	if l.gitManager != nil {
		repo, _ := l.gitManager.GetOrInit(opts.SpaceID)
		if repo != nil {
			branchName := "agent/" + opts.BotID
			if err := repo.Branch(branchName); err != nil {
				logger.Logger.Warn().
					Str("bot_id", opts.BotID).
					Str("branch", branchName).
					Err(err).
					Msg("[orchestrator] failed to create agent branch (may already exist)")
			}
		}
	}

	// 6. Start event pump.
	l.mu.Lock()
	cancel := l.startEventPump(ctx, sidecar, opts.SpaceID, opts.BotID, opts.Name)
	l.pumps[opts.BotID] = cancel
	l.mu.Unlock()

	// 7. Publish AgentJoinedSpace.
	l.eventBus.Publish(events.AgentEvent{
		Type:      events.AgentJoinedSpace,
		SpaceID:   opts.SpaceID,
		AgentID:   opts.BotID,
		AgentName: opts.Name,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"model":    opts.Model,
			"preset":   opts.Preset,
			"emoji":    opts.Emoji,
			"color":    opts.Color,
		},
	})

	logger.Logger.Info().
		Str("bot_id", opts.BotID).
		Str("space_id", opts.SpaceID).
		Str("name", opts.Name).
		Str("model", opts.Model).
		Msg("[orchestrator] bot spawned")

	return nil
}

// StopBot gracefully shuts down a bot.
func (l *BotLifecycle) StopBot(ctx context.Context, spaceID, botID string) error {
	// 1. Cancel event pump.
	l.mu.Lock()
	if cancel, ok := l.pumps[botID]; ok {
		cancel()
		delete(l.pumps, botID)
	}
	l.mu.Unlock()

	// 2. Send shutdown to sidecar.
	if sidecar, ok := l.zapPool.Get(botID); ok {
		shutdownSub := zap.Submission{
			ID: uuid.New().String(),
			Op: zap.NewShutdownOp(),
		}
		if err := sidecar.Client().SendSubmission(shutdownSub); err != nil {
			logger.Logger.Warn().
				Str("bot_id", botID).
				Err(err).
				Msg("[orchestrator] failed to send shutdown to sidecar")
		}
	}

	// 3. Unregister from gossip tracker.
	if err := l.tracker.Unregister(botID); err != nil {
		logger.Logger.Warn().
			Str("bot_id", botID).
			Err(err).
			Msg("[orchestrator] failed to unregister from tracker")
	}

	// 4. Unsubscribe from gossip router.
	l.router.Unsubscribe(botID)

	// 5. Remove from ZAP pool (stops the sidecar process).
	if err := l.zapPool.Remove(botID); err != nil {
		logger.Logger.Warn().
			Str("bot_id", botID).
			Err(err).
			Msg("[orchestrator] failed to remove sidecar from pool")
	}

	// 6. Publish AgentLeftSpace.
	l.eventBus.Publish(events.AgentEvent{
		Type:      events.AgentLeftSpace,
		SpaceID:   spaceID,
		AgentID:   botID,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"reason": "stopped"},
	})

	logger.Logger.Info().
		Str("bot_id", botID).
		Str("space_id", spaceID).
		Msg("[orchestrator] bot stopped")

	return nil
}

// InjectMessage sends a human message to a specific bot's ZAP sidecar.
func (l *BotLifecycle) InjectMessage(ctx context.Context, spaceID, botID, message, senderName string) error {
	sidecar, ok := l.zapPool.Get(botID)
	if !ok {
		return fmt.Errorf("orchestrator: no active sidecar for bot %s", botID)
	}

	// Verify bot is in the right space.
	if sidecar.Opts().SpaceID != spaceID {
		return fmt.Errorf("orchestrator: bot %s is not in space %s", botID, spaceID)
	}

	// Build user input message.
	text := message
	if senderName != "" {
		text = fmt.Sprintf("[%s]: %s", senderName, message)
	}

	sub := zap.Submission{
		ID: uuid.New().String(),
		Op: zap.NewUserInputOp([]zap.UserInput{
			zap.NewTextInput(text),
		}),
	}

	if err := sidecar.Client().SendSubmission(sub); err != nil {
		return fmt.Errorf("orchestrator: send to sidecar %s: %w", botID, err)
	}

	// Publish human message event.
	l.eventBus.Publish(events.AgentEvent{
		Type:      events.HumanMessageInjected,
		SpaceID:   spaceID,
		AgentID:   botID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"message":     message,
			"sender_name": senderName,
		},
	})

	return nil
}

// BroadcastMessage sends a human message to ALL bots in a space.
func (l *BotLifecycle) BroadcastMessage(ctx context.Context, spaceID, message, senderName string) error {
	sidecars := l.zapPool.ForSpace(spaceID)
	if len(sidecars) == 0 {
		return fmt.Errorf("orchestrator: no active bots in space %s", spaceID)
	}

	var firstErr error
	for _, sidecar := range sidecars {
		botID := sidecar.Opts().BotID
		if err := l.InjectMessage(ctx, spaceID, botID, message, senderName); err != nil {
			logger.Logger.Warn().
				Str("bot_id", botID).
				Err(err).
				Msg("[orchestrator] failed to broadcast to bot")
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

// ActiveBots returns all active bot IDs in a space.
func (l *BotLifecycle) ActiveBots(spaceID string) []string {
	agents := l.tracker.FindInSpace(spaceID)
	ids := make([]string, 0, len(agents))
	for _, a := range agents {
		if a.Status != "offline" {
			ids = append(ids, a.AgentID)
		}
	}
	return ids
}

// buildSandboxPolicy converts a sandbox mode string to a zap.SandboxPolicy.
func buildSandboxPolicy(mode string) zap.SandboxPolicy {
	switch mode {
	case zap.SandboxDangerFullAccess:
		return zap.SandboxPolicy{Type: zap.SandboxDangerFullAccess}
	case zap.SandboxReadOnly:
		return zap.SandboxPolicy{Type: zap.SandboxReadOnly}
	case zap.SandboxWorkspaceWrite:
		return zap.NewWorkspaceWriteSandbox()
	default:
		return zap.NewWorkspaceWriteSandbox()
	}
}
