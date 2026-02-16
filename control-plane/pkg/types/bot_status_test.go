package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBotStatusHealthHelpers(t *testing.T) {
	status := &BotStatus{State: BotStateActive, HealthScore: 85}
	require.True(t, status.IsHealthy())
	require.False(t, status.IsTransitioning())
	require.Equal(t, BotStateActive, status.GetEffectiveState())
	require.Equal(t, HealthStatusActive, status.ToLegacyHealthStatus())

	status.HealthScore = 50
	status.State = BotStateInactive
	require.False(t, status.IsHealthy())
	require.Equal(t, HealthStatusInactive, status.ToLegacyHealthStatus())

	status.State = BotStateActive
	status.StateTransition = &StateTransition{To: BotStateStopping}
	require.Equal(t, BotStateStopping, status.GetEffectiveState())
}

func TestBotStatusLifecycleMapping(t *testing.T) {
	status := &BotStatus{State: BotStateActive, HealthScore: 95}
	require.Equal(t, BotStatusReady, status.ToLegacyLifecycleStatus())

	status.HealthScore = 60
	require.Equal(t, BotStatusDegraded, status.ToLegacyLifecycleStatus())

	status.State = BotStateStarting
	require.Equal(t, BotStatusStarting, status.ToLegacyLifecycleStatus())

	status.State = BotStateInactive
	require.Equal(t, BotStatusOffline, status.ToLegacyLifecycleStatus())
}

func TestNewBotStatusAndLegacyConversion(t *testing.T) {
	status := NewBotStatus(BotStateStarting, StatusSourceManual)
	require.Equal(t, BotStateStarting, status.State)
	require.Equal(t, StatusSourceManual, status.Source)
	require.Equal(t, 100, status.HealthScore)
	require.Equal(t, HealthStatusUnknown, status.HealthStatus)
	require.Equal(t, BotStatusStarting, status.LifecycleStatus)
	require.WithinDuration(t, time.Now(), status.LastSeen, 2*time.Second)

	lastBeat := time.Now().Add(-2 * time.Minute)
	fromLegacy := FromLegacyStatus(HealthStatusActive, BotStatusReady, lastBeat)
	require.Equal(t, BotStateActive, fromLegacy.State)
	require.Equal(t, lastBeat, fromLegacy.LastSeen)
	require.Equal(t, StatusSourceReconcile, fromLegacy.Source)
}

func TestUpdateFromHeartbeat(t *testing.T) {
	lastBeat := time.Now().Add(-5 * time.Minute)
	status := FromLegacyStatus(HealthStatusInactive, BotStatusOffline, lastBeat)

	lifecycle := BotStatusReady
	mcp := &MCPStatusInfo{OverallHealth: 0.75}
	status.UpdateFromHeartbeat(&lifecycle, mcp)

	require.Equal(t, BotStateActive, status.State)
	require.Equal(t, lifecycle, status.LifecycleStatus)
	require.Equal(t, HealthStatusActive, status.HealthStatus)
	require.Equal(t, StatusSourceHeartbeat, status.Source)
	require.Equal(t, mcp, status.MCPStatus)
	require.GreaterOrEqual(t, status.HealthScore, 80)
}

func TestStateTransitions(t *testing.T) {
	status := NewBotStatus(BotStateActive, StatusSourceManual)

	status.StartTransition(BotStateStopping, "maintenance")
	require.True(t, status.IsTransitioning())
	require.Equal(t, BotStateStopping, status.StateTransition.To)

	status.CompleteTransition()
	require.False(t, status.IsTransitioning())
	require.Equal(t, BotStateStopping, status.State)
	require.Equal(t, status.HealthStatus, status.ToLegacyHealthStatus())
	require.Equal(t, status.LifecycleStatus, status.ToLegacyLifecycleStatus())
}
