package middleware

import (
	"context"
	"time"
)

// testContext returns a context that cancels after a short delay, used for
// simulating SSE connection teardown in tests.
func testContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}

// testWaitBriefly pauses briefly to let goroutines reach their blocking point.
func testWaitBriefly() {
	time.Sleep(50 * time.Millisecond)
}
