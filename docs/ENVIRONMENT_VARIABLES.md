# Environment Variables

This repo supports running Playground in multiple modes (local binary, Docker, Kubernetes). Most configuration is loaded via a YAML config file and can be overridden via environment variables.

Playground uses Viper with the prefix `PLAYGROUND` and maps nested config keys using `_` (for example `storage.mode` → `PLAYGROUND_STORAGE_MODE`).

> **Backward Compatibility:** The old `AGENTS_*` (server) and `AGENT_*` (node) prefixes are still supported as fallbacks. New deployments should use `PLAYGROUND_*` and `HANZO_*` respectively.

## Control Plane (Server)

### Core

- `PLAYGROUND_PORT` (optional): HTTP port for the control plane (default: `8080`).
- `PLAYGROUND_CONFIG_FILE` (optional): Path to `playground.yaml` (in containers this is typically `/etc/playground/config/playground.yaml`).
- `PLAYGROUND_HOME` (recommended in containers): Base directory where Playground stores local state (SQLite DB, Bolt DB, keys, logs). In Kubernetes, mount a PVC and set `PLAYGROUND_HOME=/data`.

### Storage

Playground supports:
- **local** (SQLite + BoltDB, stored under `PLAYGROUND_HOME`)
- **postgres** (PostgreSQL + pgvector)

Common:
- `PLAYGROUND_STORAGE_MODE`: `local` (default) or `postgres`.

Local storage (usually not needed if `PLAYGROUND_HOME` is set):
- `PLAYGROUND_STORAGE_LOCAL_DATABASE_PATH`: SQLite path.
- `PLAYGROUND_STORAGE_LOCAL_KV_STORE_PATH`: BoltDB path.

PostgreSQL storage:
- `PLAYGROUND_POSTGRES_URL` (preferred) or `PLAYGROUND_STORAGE_POSTGRES_URL`: PostgreSQL DSN/URL (examples below).
- Alternatively, individual fields:
  - `PLAYGROUND_STORAGE_POSTGRES_HOST`
  - `PLAYGROUND_STORAGE_POSTGRES_PORT`
  - `PLAYGROUND_STORAGE_POSTGRES_DATABASE`
  - `PLAYGROUND_STORAGE_POSTGRES_USER`
  - `PLAYGROUND_STORAGE_POSTGRES_PASSWORD`
  - `PLAYGROUND_STORAGE_POSTGRES_SSLMODE`

Example DSNs:
- `postgres://playground:playground@postgres:5432/playground?sslmode=disable`
- `postgresql://playground:playground@postgres:5432/playground?sslmode=disable`

### API Authentication (optional)

If set, the control plane requires an API key for most endpoints.

- `PLAYGROUND_API_KEY` or `PLAYGROUND_API_AUTH_API_KEY`: API key checked by the control plane.

### UI

- `PLAYGROUND_UI_ENABLED` (default: `true`)
- `PLAYGROUND_UI_MODE` (default: `embedded`)

### CORS (HTTP API)

These map to `api.cors.*` in config. When set via env, use comma-separated values.

- `PLAYGROUND_API_CORS_ALLOWED_ORIGINS` (comma-separated)
- `PLAYGROUND_API_CORS_ALLOWED_METHODS` (comma-separated)
- `PLAYGROUND_API_CORS_ALLOWED_HEADERS` (comma-separated)
- `PLAYGROUND_API_CORS_EXPOSED_HEADERS` (comma-separated)
- `PLAYGROUND_API_CORS_ALLOW_CREDENTIALS` (`true`/`false`)

## Nodes

Nodes run as separate processes/pods and register with the control plane. The most important Kubernetes-specific concept is:

- The **control plane must be able to reach the node** at the URL it registers (its callback/public URL).
- In Kubernetes, this should usually be a `Service` DNS name (for example `http://my-bot.default.svc.cluster.local:8001`).

The same concept applies to **Docker**:

- If the control plane runs in a container and the node runs on your host, set the node's callback/public URL to `host.docker.internal` (or the Docker host gateway on Linux).
- If both run in the same Docker network/Compose project, set the callback/public URL to the service name (for example `http://demo-go-bot:8001`).

### Go SDK bots (example: `examples/go_bots`)

- `PLAYGROUND_URL` (optional): Control plane base URL (example: `http://playground:8080`).
- `PLAYGROUND_TOKEN` (optional): Bearer token (use this if you enable `PLAYGROUND_API_KEY` on the control plane).
- `HANZO_NODE_ID` (optional): Node id (default varies by example).
- `HANZO_LISTEN_ADDR` (optional): Listen address (default: `:8001`).
- `HANZO_PUBLIC_URL` (recommended in Docker/Kubernetes): Public URL the control plane will call back to (example: `http://my-bot:8001`).

### Python SDK bots

- `PLAYGROUND_URL` (recommended): Control plane base URL.
- `HANZO_NODE_ID` (optional): Node id.
- `HANZO_CALLBACK_URL` (recommended in Docker/Kubernetes): URL the control plane will call back to (examples: `http://my-bot:8001`, or for host-run bots with Dockerized control plane: `http://host.docker.internal:8001`).

Many Python examples also require model provider credentials (for example `OPENAI_API_KEY`), depending on the `AIConfig` you choose.
