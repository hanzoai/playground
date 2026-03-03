package cloud

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ActiveAgents tracks the number of currently provisioned cloud agents.
	ActiveAgents = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "playground_active_agents",
		Help: "Number of currently running cloud agents (K8s pods + Visor VMs).",
	})

	// ProvisionTotal counts provisioning requests by tier, OS, and outcome.
	ProvisionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "playground_provision_total",
		Help: "Total number of cloud agent provisioning attempts.",
	}, []string{"tier", "os", "status"})

	// ProvisionDuration tracks how long provisioning takes.
	ProvisionDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "playground_provision_duration_seconds",
		Help:    "Time to provision a cloud agent, split by OS.",
		Buckets: []float64{0.5, 1, 2, 5, 10, 30, 60, 120, 300},
	}, []string{"os"})

	// DeprovisionTotal counts deprovisioning requests by outcome.
	DeprovisionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "playground_deprovision_total",
		Help: "Total number of cloud agent deprovisioning attempts.",
	}, []string{"status"})

	// BillingCheckTotal counts billing gate checks by result.
	BillingCheckTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "playground_billing_check_total",
		Help: "Total billing pre-checks for provisioning, split by result.",
	}, []string{"result"})

	// BillingHoldTotal counts billing holds created (successful provisioning with spend committed).
	BillingHoldTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "playground_billing_hold_total",
		Help: "Total billing holds created on successful provisioning.",
	}, []string{"tier"})
)

// RecordProvision records a successful or failed provisioning attempt.
func RecordProvision(tier, os, status string, duration time.Duration) {
	if tier == "" {
		tier = "unknown"
	}
	if os == "" {
		os = "linux"
	}
	ProvisionTotal.WithLabelValues(tier, os, status).Inc()
	if status == "success" {
		ProvisionDuration.WithLabelValues(os).Observe(duration.Seconds())
		ActiveAgents.Inc()
	}
}

// RecordDeprovision records a deprovisioning event.
func RecordDeprovision(status string) {
	DeprovisionTotal.WithLabelValues(status).Inc()
	if status == "success" {
		ActiveAgents.Dec()
	}
}

// RecordBillingCheck records a billing gate check result.
func RecordBillingCheck(result string) {
	BillingCheckTotal.WithLabelValues(result).Inc()
}

// RecordBillingHold records a billing hold created for a tier.
func RecordBillingHold(tier string) {
	if tier == "" {
		tier = "unknown"
	}
	BillingHoldTotal.WithLabelValues(tier).Inc()
}

// SyncActiveAgentCount sets the active agent gauge to the actual count.
// Called during Sync() to reconcile the gauge with reality.
func SyncActiveAgentCount(count int) {
	ActiveAgents.Set(float64(count))
}
