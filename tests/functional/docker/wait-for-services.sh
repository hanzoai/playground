#!/bin/sh
# wait-for-services.sh - Wait for Playground control plane to be ready

set -e

CONTROL_PLANE_URL="${PLAYGROUND_SERVER:-${AGENTS_SERVER:-http://control-plane:8080}}"
HEALTH_ENDPOINT="${CONTROL_PLANE_URL}/api/v1/health"
# Allow up to 60*2s=120s for control plane to start (includes Docker build time in CI)
MAX_ATTEMPTS="${MAX_ATTEMPTS:-60}"
SLEEP_INTERVAL="${SLEEP_INTERVAL:-2}"

echo "Waiting for Playground control plane at ${CONTROL_PLANE_URL}..."

attempt=0
while [ $attempt -lt $MAX_ATTEMPTS ]; do
    attempt=$((attempt + 1))

    # Try to reach the health endpoint with a simple GET request
    if curl --silent --show-error --fail --max-time 2 "${HEALTH_ENDPOINT}" >/dev/null; then
        echo "Control plane is ready!"
        exit 0
    fi

    echo "Attempt $attempt/$MAX_ATTEMPTS: Control plane not ready yet..."
    sleep $SLEEP_INTERVAL
done

echo "Control plane failed to become ready after $MAX_ATTEMPTS attempts"
exit 1
