package cloud

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestRecordProvisionSuccess(t *testing.T) {
	RecordProvision("s-2vcpu-4gb", "linux", "success", 2*time.Second)
	require.GreaterOrEqual(t, testutil.ToFloat64(ProvisionTotal.WithLabelValues("s-2vcpu-4gb", "linux", "success")), float64(1))
}

func TestRecordProvisionFailure(t *testing.T) {
	RecordProvision("s-2vcpu-4gb", "linux", "error", 0)
	require.GreaterOrEqual(t, testutil.ToFloat64(ProvisionTotal.WithLabelValues("s-2vcpu-4gb", "linux", "error")), float64(1))
}

func TestRecordProvisionDefaultLabels(t *testing.T) {
	RecordProvision("", "", "success", time.Second)
	require.GreaterOrEqual(t, testutil.ToFloat64(ProvisionTotal.WithLabelValues("unknown", "linux", "success")), float64(1))
}

func TestRecordDeprovisionSuccess(t *testing.T) {
	RecordDeprovision("success")
	require.GreaterOrEqual(t, testutil.ToFloat64(DeprovisionTotal.WithLabelValues("success")), float64(1))
}

func TestRecordBillingCheck(t *testing.T) {
	RecordBillingCheck("allowed")
	require.GreaterOrEqual(t, testutil.ToFloat64(BillingCheckTotal.WithLabelValues("allowed")), float64(1))

	RecordBillingCheck("denied")
	require.GreaterOrEqual(t, testutil.ToFloat64(BillingCheckTotal.WithLabelValues("denied")), float64(1))
}

func TestRecordBillingHold(t *testing.T) {
	RecordBillingHold("s-2vcpu-4gb")
	require.GreaterOrEqual(t, testutil.ToFloat64(BillingHoldTotal.WithLabelValues("s-2vcpu-4gb")), float64(1))
}

func TestRecordBillingHoldDefault(t *testing.T) {
	RecordBillingHold("")
	require.GreaterOrEqual(t, testutil.ToFloat64(BillingHoldTotal.WithLabelValues("unknown")), float64(1))
}

func TestSyncActiveAgentCount(t *testing.T) {
	SyncActiveAgentCount(7)
	require.Equal(t, float64(7), testutil.ToFloat64(ActiveAgents))
}
