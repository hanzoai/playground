#!/bin/bash
# Railway build script that waits for playground to be available on PyPI
# This handles the race condition where Railway deploys before PyPI upload completes

set -e

# Set up virtual environment (replaces Nixpacks default install phase)
python -m venv --copies /opt/venv
. /opt/venv/bin/activate

# Extract playground version requirement from requirements.txt
AGENTS_REQ=$(grep -E "^playground" requirements.txt || echo "")

if [ -z "$AGENTS_REQ" ]; then
    echo "No playground requirement found, proceeding with normal install"
    pip install -r requirements.txt
    exit 0
fi

# Parse minimum version from requirement (handles >=X.Y.Z format)
MIN_VERSION=$(echo "$AGENTS_REQ" | sed -E 's/playground[>=<]+//' | tr -d ' ')

echo "Checking for playground>=$MIN_VERSION on PyPI..."

MAX_RETRIES=30
RETRY_INTERVAL=10

for i in $(seq 1 $MAX_RETRIES); do
    # Try to install the specific version to check if it exists
    if pip install --dry-run "playground>=$MIN_VERSION" >/dev/null 2>&1; then
        echo "playground>=$MIN_VERSION is available on PyPI"
        break
    fi

    if [ "$i" -eq "$MAX_RETRIES" ]; then
        echo "Warning: Timed out waiting for playground $MIN_VERSION on PyPI"
        echo "Attempting install anyway..."
        break
    fi

    echo "Attempt $i/$MAX_RETRIES: playground>=$MIN_VERSION not yet available, waiting ${RETRY_INTERVAL}s..."
    sleep $RETRY_INTERVAL
done

pip install -r requirements.txt
echo "Installation complete"
