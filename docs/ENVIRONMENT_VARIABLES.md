# Environment Variables

This repo supports running Playground in multiple modes (local binary, Docker, Kubernetes). Most configuration is loaded via a YAML config file and can be overridden via environment variables.

Playground uses Viper with the prefix `AGENTS` and maps nested config keys using `_` (for example `storage.mode` → `AGENTS_STORAGE_MODE`).

## Control Plane (Server)

### Core

- `AGENTS_PORT` (optional): HTTP port for the control plane (default: `8080`).
- `AGENTS_CONFIG_FILE` (optional): Path to `agents.yaml` (in containers this is typically `/etc/playground/config/agents.yaml`).
- `AGENTS_HOME` (recommended in containers): Base directory where Playground stores local state (SQLite DB, Bolt DB, keys, logs). In Kubernetes, mount a PVC and set `AGENTS_HOME=/data`.

### Storage

Playground supports:
- **local** (SQLite + BoltDB, stored under `AGENTS_HOME`)
- **postgres** (PostgreSQL + pgvector)

Common:
- `AGENTS_STORAGE_MODE`: `local` (default) or `postgres`.

Local storage (usually not needed if `AGENTS_HOME` is set):
- `AGENTS_STORAGE_LOCAL_DATABASE_PATH`: SQLite path.
- `AGENTS_STORAGE_LOCAL_KV_STORE_PATH`: BoltDB path.

PostgreSQL storage:
- `AGENTS_POSTGRES_URL` (preferred) or `AGENTS_STORAGE_POSTGRES_URL`: PostgreSQL DSN/URL (examples below).
- Alternatively, individual fields:
  - `AGENTS_STORAGE_POSTGRES_HOST`
  - `AGENTS_STORAGE_POSTGRES_PORT`
  - `AGENTS_STORAGE_POSTGRES_DATABASE`
  - `AGENTS_STORAGE_POSTGRES_USER`
  - `AGENTS_STORAGE_POSTGRES_PASSWORD`
  - `AGENTS_STORAGE_POSTGRES_SSLMODE`

Example DSNs:
- `postgres://playground:playground@postgres:5432/playground?sslmode=disable`
- `postgresql://playground:playground@postgres:5432/playground?sslmode=disable`

### API Authentication (optional)

If set, the control plane requires an API key for most endpoints.

- `AGENTS_API_KEY` or `AGENTS_API_AUTH_API_KEY`: API key checked by the control plane.

### UI

- `AGENTS_UI_ENABLED` (default: `true`)
- `AGENTS_UI_MODE` (default: `embedded`)

### CORS (HTTP API)

These map to `api.cors.*` in config. When set via env, use comma-separated values.

- `AGENTS_API_CORS_ALLOWED_ORIGINS` (comma-separated)
- `AGENTS_API_CORS_ALLOWED_METHODS` (comma-separated)
- `AGENTS_API_CORS_ALLOWED_HEADERS` (comma-separated)
- `AGENTS_API_CORS_EXPOSED_HEADERS` (comma-separated)
- `AGENTS_API_CORS_ALLOW_CREDENTIALS` (`true`/`false`)

## Agent Nodes

Agent nodes run as separate processes/pods and register with the control plane. The most important Kubernetes-specific concept is:

- The **control plane must be able to reach the agent** at the URL the agent registers (its callback/public URL).
- In Kubernetes, this should usually be a `Service` DNS name (for example `http://my-agent.default.svc.cluster.local:8001`).

The same concept applies to **Docker**:

- If the control plane runs in a container and the agent runs on your host, set the agent’s callback/public URL to `host.docker.internal` (or the Docker host gateway on Linux).
- If both run in the same Docker network/Compose project, set the callback/public URL to the agent service name (for example `http://demo-go-agent:8001`).

### Go SDK agents (example: `examples/go_agent_nodes`)

- `AGENTS_URL` (optional): Control plane base URL (example: `http://playground:8080`).
- `AGENTS_TOKEN` (optional): Bearer token (use this if you enable `AGENTS_API_KEY` on the control plane).
- `AGENT_NODE_ID` (optional): Node id (default varies by example).
- `AGENT_LISTEN_ADDR` (optional): Listen address (default: `:8001`).
- `AGENT_PUBLIC_URL` (recommended in Docker/Kubernetes): Public URL the control plane will call back to (example: `http://my-agent:8001`).

### Python SDK agents

- `AGENTS_URL` (recommended): Control plane base URL.
- `AGENT_NODE_ID` (optional): Node id.
- `AGENT_CALLBACK_URL` (recommended in Docker/Kubernetes): URL the control plane will call back to (examples: `http://my-agent:8001`, or for host-run agents with Dockerized control plane: `http://host.docker.internal:8001`).

Many Python examples also require model provider credentials (for example `OPENAI_API_KEY`), depending on the `AIConfig` you choose.
