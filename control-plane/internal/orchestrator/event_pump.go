package orchestrator

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hanzoai/playground/control-plane/internal/events"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/hanzoai/playground/control-plane/internal/zap"
)

// startEventPump starts a goroutine that reads ZAP EventMsg from a sidecar
// and publishes them as AgentEvents to the event bus.
// Returns a cancel function to stop the pump.
func (l *BotLifecycle) startEventPump(ctx context.Context, sidecar *zap.Sidecar, spaceID, botID, botName string) context.CancelFunc {
	pumpCtx, cancel := context.WithCancel(ctx)

	go func() {
		evCh := sidecar.Client().Events()

		for {
			select {
			case <-pumpCtx.Done():
				return
			case msg, ok := <-evCh:
				if !ok {
					// Channel closed -- sidecar disconnected.
					l.handleDisconnect(spaceID, botID, botName)
					return
				}

				agentEvt := mapZAPEvent(msg, spaceID, botID, botName)
				if agentEvt == nil {
					continue
				}
				l.eventBus.Publish(*agentEvt)
			}
		}
	}()

	return cancel
}

// handleDisconnect updates tracker and publishes a leave event when a sidecar disconnects.
func (l *BotLifecycle) handleDisconnect(spaceID, botID, botName string) {
	if err := l.tracker.UpdateStatus(botID, "offline"); err != nil {
		logger.Logger.Warn().
			Str("bot_id", botID).
			Err(err).
			Msg("[orchestrator] failed to update tracker status on disconnect")
	}

	l.eventBus.Publish(events.AgentEvent{
		Type:      events.AgentLeftSpace,
		SpaceID:   spaceID,
		AgentID:   botID,
		AgentName: botName,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"reason": "sidecar_disconnected"},
	})
}

// mapZAPEvent translates a ZAP EventMsg to an AgentEvent.
// Returns nil for unmapped event types.
func mapZAPEvent(msg zap.EventMsg, spaceID, botID, botName string) *events.AgentEvent {
	var eventType events.AgentEventType

	switch msg.Type {
	case zap.EventTurnStarted:
		eventType = events.AgentTurnStarted
	case zap.EventTurnComplete:
		eventType = events.AgentTurnCompleted
	case zap.EventAgentMessage:
		eventType = events.AgentMessage
	case zap.EventAgentMessageDelta:
		eventType = events.AgentMessageDelta
	case zap.EventExecCommandBegin:
		eventType = events.AgentExecBegin
	case zap.EventExecCommandEnd:
		eventType = events.AgentExecEnd
	case zap.EventMcpToolCallBegin:
		eventType = events.AgentToolCallBegin
	case zap.EventMcpToolCallEnd:
		eventType = events.AgentToolCallEnd
	default:
		// Unmapped event type -- skip.
		return nil
	}

	// Parse raw payload into data map if available.
	var data map[string]interface{}
	if len(msg.Raw) > 0 {
		_ = json.Unmarshal(msg.Raw, &data)
	}

	return &events.AgentEvent{
		Type:      eventType,
		SpaceID:   spaceID,
		AgentID:   botID,
		AgentName: botName,
		Timestamp: time.Now(),
		Data:      data,
	}
}
