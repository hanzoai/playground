package billing

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/hanzoai/playground/control-plane/internal/logger"
)

// NodeInfo contains the billing-relevant fields for a running cloud node.
// The provisioner populates this via the NodeLister interface.
type NodeInfo struct {
	NodeID        string
	BillingUserID string
	BearerToken   string
	CentsPerHour  int
	ProvisionedAt time.Time
}

// NodeLister returns the set of running cloud nodes that need usage metering.
// Implemented by the cloud.Provisioner.
type NodeLister interface {
	RunningNodes() []NodeInfo
}

// MeteringService periodically records usage for all running cloud nodes.
// It iterates running nodes, calculates elapsed compute time, and posts
// usage events to Commerce so billing stays current between hold and settle.
type MeteringService struct {
	client   *Client
	lister   NodeLister
	interval time.Duration

	mu          sync.Mutex
	lastMetered map[string]time.Time // nodeID -> last metered timestamp
}

// NewMeteringService creates a metering service that records usage at the given interval.
func NewMeteringService(client *Client, lister NodeLister, interval time.Duration) *MeteringService {
	return &MeteringService{
		client:      client,
		lister:      lister,
		interval:    interval,
		lastMetered: make(map[string]time.Time),
	}
}

// Run starts the metering loop. It blocks until ctx is cancelled.
// Call this in a goroutine: go meteringService.Run(ctx)
func (m *MeteringService) Run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	logger.Logger.Info().
		Dur("interval", m.interval).
		Msg("billing metering service started")

	for {
		select {
		case <-ctx.Done():
			logger.Logger.Info().Msg("billing metering service stopped")
			return
		case <-ticker.C:
			m.meterAll(ctx)
		}
	}
}

// meterAll records usage for every running node since its last metered time.
func (m *MeteringService) meterAll(ctx context.Context) {
	nodes := m.lister.RunningNodes()
	if len(nodes) == 0 {
		return
	}

	now := time.Now()

	m.mu.Lock()
	// Prune nodes that are no longer running.
	activeIDs := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		activeIDs[n.NodeID] = true
	}
	for id := range m.lastMetered {
		if !activeIDs[id] {
			delete(m.lastMetered, id)
		}
	}
	m.mu.Unlock()

	for _, node := range nodes {
		if node.CentsPerHour <= 0 || node.BillingUserID == "" {
			continue
		}

		m.mu.Lock()
		since, ok := m.lastMetered[node.NodeID]
		if !ok {
			since = node.ProvisionedAt
		}
		m.lastMetered[node.NodeID] = now
		m.mu.Unlock()

		elapsed := now.Sub(since)
		if elapsed <= 0 {
			continue
		}

		// Calculate cents for the elapsed period, rounding up to the nearest cent.
		hours := elapsed.Hours()
		centsUsed := int(math.Ceil(hours * float64(node.CentsPerHour)))
		if centsUsed <= 0 {
			continue
		}

		metadata := map[string]string{
			"node_id": node.NodeID,
			"type":    "compute",
			"period":  elapsed.String(),
		}

		if err := m.client.RecordUsage(ctx, node.BillingUserID, node.BearerToken, centsUsed, metadata); err != nil {
			logger.Logger.Warn().
				Err(err).
				Str("node_id", node.NodeID).
				Str("user", node.BillingUserID).
				Int("cents", centsUsed).
				Msg("failed to record usage for node")
		} else {
			logger.Logger.Debug().
				Str("node_id", node.NodeID).
				Int("cents", centsUsed).
				Str("period", elapsed.String()).
				Msg("recorded compute usage")
		}
	}
}
