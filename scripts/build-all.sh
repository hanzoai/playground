#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "==> Building control plane"
(cd "$ROOT_DIR/playground" && go build ./... )

echo "==> Building control plane web UI"
(cd "$ROOT_DIR/./web/client" && npm run build)

echo "==> Building Go SDK"
(cd "$ROOT_DIR/sdk/go" && go build ./...)

echo "==> Building Python SDK package"
(cd "$ROOT_DIR/sdk/python" && python3 -m build)

echo "Build complete."
