package policy

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Engine evaluates whether a bot action is permitted.
type Engine struct {
	spacePolicies    map[string]*SpacePolicy      // spaceID -> policy
	botPolicies      map[string]*BotPolicy         // botID -> policy
	pendingApprovals map[string]*ApprovalRequest   // requestID -> request
	approvalCh       chan ApprovalRequest           // notify UI of new requests
	mu               sync.RWMutex
}

// NewEngine creates a new policy engine.
func NewEngine() *Engine {
	return &Engine{
		spacePolicies:    make(map[string]*SpacePolicy),
		botPolicies:      make(map[string]*BotPolicy),
		pendingApprovals: make(map[string]*ApprovalRequest),
		approvalCh:       make(chan ApprovalRequest, 64),
	}
}

// SetSpacePolicy sets the default policy for a space.
func (e *Engine) SetSpacePolicy(policy SpacePolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = now
	}
	policy.UpdatedAt = now
	e.spacePolicies[policy.SpaceID] = &policy
}

// SetBotPolicy sets the policy for a specific bot (overrides space default).
func (e *Engine) SetBotPolicy(policy BotPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = now
	}
	policy.UpdatedAt = now
	e.botPolicies[policy.BotID] = &policy
}

// GetBotPolicy returns the effective policy for a bot.
// If a bot-specific policy exists, it is returned. Otherwise, the space
// default is synthesized into a BotPolicy. Returns nil if no policy exists.
func (e *Engine) GetBotPolicy(botID, spaceID string) *BotPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if bp, ok := e.botPolicies[botID]; ok {
		return bp
	}

	sp, ok := e.spacePolicies[spaceID]
	if !ok {
		return nil
	}

	// Synthesize a BotPolicy from the space default.
	return &BotPolicy{
		BotID:        botID,
		SpaceID:      spaceID,
		ApprovalMode: sp.ApprovalMode,
		Rules:        sp.DefaultRules,
		MaxSpendUSD:  sp.MaxSpendUSD,
		CreatedAt:    sp.CreatedAt,
		UpdatedAt:    sp.UpdatedAt,
	}
}

// GetSpacePolicy returns the space policy for the given space ID, or nil.
func (e *Engine) GetSpacePolicy(spaceID string) *SpacePolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.spacePolicies[spaceID]
}

// Check evaluates whether a bot action is allowed.
// Returns:
//   - allowed: the action is permitted (or will be after approval)
//   - requiresApproval: a human must approve before proceeding
//   - reason: human-readable explanation
func (e *Engine) Check(botID, spaceID string, resource ResourceType, perm Permission) (allowed bool, requiresApproval bool, reason string) {
	bp := e.GetBotPolicy(botID, spaceID)
	if bp == nil {
		return false, false, "no policy configured for bot or space"
	}

	// Check for explicit deny first — deny overrides everything, even bypass.
	for _, rule := range bp.Rules {
		if rule.Resource == resource && rule.Permission == PermDeny {
			return false, false, fmt.Sprintf("explicitly denied: %s on %s", perm, resource)
		}
	}

	// Bypass mode: everything not explicitly denied is allowed.
	if bp.ApprovalMode == ApprovalBypass {
		return true, false, "bypass mode"
	}

	// Check if the requested permission is covered by any rule on this resource.
	granted := false
	for _, rule := range bp.Rules {
		if rule.Resource == resource && permLevel(rule.Permission) >= permLevel(perm) {
			granted = true
			break
		}
	}

	if !granted {
		// In managed mode, ungrant-ed write/execute/admin actions require approval.
		if bp.ApprovalMode == ApprovalManaged && perm != PermRead {
			return false, true, fmt.Sprintf("no rule grants %s on %s; approval required", perm, resource)
		}
		return false, false, fmt.Sprintf("no rule grants %s on %s", perm, resource)
	}

	// In managed mode, write/admin actions on sensitive resources need approval.
	if bp.ApprovalMode == ApprovalManaged {
		if isSensitiveAction(resource, perm) {
			return true, true, fmt.Sprintf("%s on %s is sensitive; approval recommended", perm, resource)
		}
	}

	return true, false, "allowed by policy"
}

// isSensitiveAction returns true for actions that should trigger approval
// even when the rule technically grants the permission in managed mode.
func isSensitiveAction(resource ResourceType, perm Permission) bool {
	switch resource {
	case ResourceDeploy, ResourceBilling, ResourceSecrets, ResourceIAM:
		return perm != PermRead
	case ResourceDNS, ResourceKMS:
		return perm == PermWrite || perm == PermAdmin
	default:
		return perm == PermAdmin
	}
}

// RequestApproval creates a pending approval request for a human.
// Returns the request ID.
func (e *Engine) RequestApproval(req ApprovalRequest) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.RequestedAt.IsZero() {
		req.RequestedAt = time.Now()
	}
	req.Status = ApprovalStatusPending
	e.pendingApprovals[req.ID] = &req

	// Non-blocking send to notification channel.
	select {
	case e.approvalCh <- req:
	default:
	}

	return req.ID
}

// ResolveApproval approves or denies a pending request.
func (e *Engine) ResolveApproval(requestID string, approved bool, resolvedBy string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	req, ok := e.pendingApprovals[requestID]
	if !ok {
		return fmt.Errorf("approval request %s not found", requestID)
	}
	if req.Status != ApprovalStatusPending {
		return fmt.Errorf("approval request %s already resolved: %s", requestID, req.Status)
	}

	now := time.Now()
	req.ResolvedAt = &now
	req.ResolvedBy = resolvedBy
	if approved {
		req.Status = ApprovalStatusApproved
	} else {
		req.Status = ApprovalStatusDenied
	}

	return nil
}

// PendingApprovals returns all pending requests for a space.
func (e *Engine) PendingApprovals(spaceID string) []ApprovalRequest {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []ApprovalRequest
	for _, req := range e.pendingApprovals {
		if req.SpaceID == spaceID && req.Status == ApprovalStatusPending {
			result = append(result, *req)
		}
	}
	return result
}

// ApprovalRequests returns a channel that notifies of new approval requests.
func (e *Engine) ApprovalRequests() <-chan ApprovalRequest {
	return e.approvalCh
}

// IsBypassMode returns true if the bot or space is in bypass mode.
func (e *Engine) IsBypassMode(botID, spaceID string) bool {
	bp := e.GetBotPolicy(botID, spaceID)
	if bp == nil {
		return false
	}
	return bp.ApprovalMode == ApprovalBypass
}

// SetBypassMode toggles bypass mode for a space.
func (e *Engine) SetBypassMode(spaceID string, bypass bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	sp, ok := e.spacePolicies[spaceID]
	if !ok {
		// Create a new space policy if one doesn't exist.
		if bypass {
			bp := BypassPolicy(spaceID)
			e.spacePolicies[spaceID] = &bp
		}
		return
	}

	if bypass {
		sp.ApprovalMode = ApprovalBypass
		bp := BypassPolicy(spaceID)
		sp.DefaultRules = bp.DefaultRules
	} else {
		sp.ApprovalMode = ApprovalManaged
		dp := DefaultSpacePolicy(spaceID)
		sp.DefaultRules = dp.DefaultRules
	}
	sp.UpdatedAt = time.Now()
}
