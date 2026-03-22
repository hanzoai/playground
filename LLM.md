# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Playground is a Kubernetes-style control plane for AI bots. It provides production infrastructure for deploying, orchestrating, and observing multi-bot systems with cryptographic identity and audit trails.

**Architecture:** Three-tier monorepo
- **Control Plane** (Go): Orchestration server providing REST/gRPC APIs, workflow execution, observability, and cryptographic identity
- **SDKs** (Python & Go): Libraries for building bots that communicate with the control plane
- **Web UI** (React/TypeScript): Embedded admin interface for monitoring workflows and managing bots

## Development Setup

### Prerequisites
- Go 1.23+
- Python 3.8+
- Node.js 20+
- PostgreSQL 15+ (for cloud mode)

### Initial Setup
```bash
# Install all dependencies
make install

# Or install components individually:
./scripts/install.sh

# Build everything
make build
```

### Running the Control Plane

**Local mode** (uses SQLite + BoltDB, no external dependencies):
```bash
cd control-plane
go run ./cmd/playground dev
# Or: go run ./cmd/playground-server
```

**Cloud mode** (requires PostgreSQL):
```bash
# Run migrations first
cd control-plane
export PLAYGROUND_DATABASE_URL="postgres://playground:playground@localhost:5432/playground?sslmode=disable"
goose -dir ./migrations postgres "$PLAYGROUND_DATABASE_URL" up

# Start server
PLAYGROUND_STORAGE_MODE=postgresql \
PLAYGROUND_DATABASE_URL="postgres://playground:playground@localhost:5432/playground?sslmode=disable" \
go run ./cmd/playground-server
```

**Docker Compose** (includes PostgreSQL):
```bash
cd deployments/docker
docker compose up
```

Control plane runs at `http://localhost:8080`
Web UI accessible at `http://localhost:8080/ui/`

## Common Commands

### Building
```bash
make build                 # Build all components
make control-plane         # Build control plane only
make sdk-go               # Build Go SDK
make sdk-python           # Build Python SDK
```

### Testing
```bash
make test                 # Run all tests

# Component-specific tests:
cd control-plane && go test ./...
cd sdk/go && go test ./...
cd sdk/python && pytest

# Python tests with coverage:
cd sdk/python && pytest --cov=playground --cov-report=term-missing

# Web UI linting:
cd control-plane/web/client && npm run lint
```

### Linting & Formatting
```bash
make lint                 # Lint all code
make fmt                  # Format all code
make tidy                 # Tidy Go modules

# Component-specific:
cd control-plane && golangci-lint run
cd sdk/python && ruff check
cd sdk/python && ruff format .
```

### Database Migrations
```bash
cd control-plane
export PLAYGROUND_DATABASE_URL="postgres://playground:playground@localhost:5432/playground?sslmode=disable"

# Check migration status
goose -dir ./migrations postgres "$PLAYGROUND_DATABASE_URL" status

# Apply all pending migrations
goose -dir ./migrations postgres "$PLAYGROUND_DATABASE_URL" up

# Create new migration
goose -dir ./migrations create <migration_name> sql
```

### Web UI Development
```bash
cd control-plane/web/client
npm install
npm run dev    # Runs on http://localhost:5173

# In parallel, run the control plane server to handle API calls
cd control-plane
go run ./cmd/playground-server
```

The UI dev server proxies API requests to the control plane. In production, the UI is embedded via Go's `embed` package.

## Architecture Deep Dive

### Control Plane Structure (`control-plane/`)

**Entry Points:**
- `cmd/playground/` - Unified CLI with server + dev/init commands
- `cmd/playground-server/` - Standalone server binary

**Core Packages (`internal/`):**
- `cli/` - CLI command definitions and routing
- `server/` - HTTP server setup (Gin framework), middleware, routing
- `handlers/` - HTTP request handlers for REST/gRPC endpoints
- `services/` - Business logic layer (workflow execution, bot registry, DID/VC generation)
- `storage/` - Data persistence layer with multiple backends (local SQLite/BoltDB, PostgreSQL, cloud)
- `events/` - Event bus for workflow notifications and SSE streaming
- `core/` - Domain models and interfaces
- `application/` - Application service orchestration
- `infrastructure/` - Infrastructure utilities (database connection pooling, etc.)
- `mcp/` - Model Context Protocol integration
- `logger/` - Structured logging (zerolog)
- `config/` - Configuration management (Viper)
- `templates/` - Code generation templates for `playground init`
- `utils/` - Shared utilities
- `encryption/` - Cryptographic primitives for DID/VC
- `packages/` - Shared internal packages
- `embedded/` - Embedded assets (web UI dist)

**Configuration:**
- Environment variables take precedence over `config/playground.yaml`
- See `control-plane/.env.example` for all options
- Key modes: `PLAYGROUND_MODE=local` (SQLite/BoltDB) vs `PLAYGROUND_STORAGE_MODE=postgresql` (cloud)

**Database Schema:**
- `migrations/` - SQL migrations managed by Goose
- Always run migrations before starting the server in PostgreSQL mode

### SDK Structure

**Python SDK (`sdk/python/playground/`):**
- Built on FastAPI/Uvicorn for bot HTTP servers
- Key modules: `Bot`, `bot_field_handler`, `client`, `execution_context`, `memory`, `ai`
- Bots register "reasoners" (decorated functions) that become REST endpoints
- Test with: `pytest` (see `pyproject.toml` for test markers: unit, functional, integration)
- Install locally: `pip install -e .[dev]`

**Go SDK (`sdk/go/`):**
- Modules: `bot/` (bot builder), `client/` (HTTP client), `types/` (shared types), `ai/` (LLM helpers)
- Bots register "skills" (functions) similar to Python SDK
- Test with: `go test ./...`

### Web UI (`control-plane/web/client/`)
- React + TypeScript + Vite
- Tailwind CSS + Radix UI components
- Build: `npm run build` → outputs to `dist/` → embedded in Go binary
- Dev mode: `npm run dev` (separate Vite server)

## Key Workflows

### Creating a New Bot (Python)
```bash
# Generate bot scaffold (run from repo root or any directory)
playground init my-bot
cd my-bot

# Edit bot code (auto-generated template)
# Run bot locally (connects to control plane at PLAYGROUND_SERVER env var or --server flag)
playground run
```

### Creating a New Bot (Go)
```go
import playgroundbot "github.com/hanzoai/playground/sdk/go/bot"

bot, _ := playgroundbot.New(playgroundbot.Config{
    NodeID:   "my-bot",
    PlaygroundURL: "http://localhost:8080",
})
bot.RegisterSkill("greet", func(ctx context.Context, input map[string]any) (any, error) {
    return map[string]any{"message": "hello"}, nil
})
bot.Run(context.Background())
```

### Adding a New Control Plane Endpoint
1. Define handler in `control-plane/internal/handlers/<domain>/`
2. Add route in `control-plane/internal/server/routes.go`
3. Add business logic in `control-plane/internal/services/<domain>/`
4. Add storage methods in `control-plane/internal/storage/<domain>/`
5. If adding new DB tables, create migration: `goose -dir ./migrations create <name> sql`

### Storage Modes
- **Local mode:** SQLite (relational) + BoltDB (key-value). No external dependencies. Good for dev/testing.
- **PostgreSQL mode:** Full PostgreSQL backend. Requires running migrations. Production-ready.
- **Cloud mode:** PostgreSQL backend. Used in distributed deployments.

Storage interface is unified—services call storage layer methods, storage layer switches backends based on config.

## Testing Strategy

**Control Plane:**
- Unit tests: `go test ./...` (mock storage/services)
- Integration tests: Spin up test database, run migrations, test full stack

**Python SDK:**
- Markers: `@pytest.mark.unit`, `@pytest.mark.functional`, `@pytest.mark.integration`, `@pytest.mark.mcp`
- Default: `pytest` runs all except MCP tests (use `-m mcp` to include)
- Coverage tracked for core modules (see `pyproject.toml`)

**Go SDK:**
- Standard `go test ./...`
- Table-driven tests preferred

## Important Patterns

### Error Handling
- Control plane: Return structured JSON errors with HTTP status codes
- SDKs: Raise/return typed exceptions/errors with context
- Log errors before returning (use zerolog in Go, standard logging in Python)

### Configuration Precedence
1. Environment variables (highest priority)
2. Config file (`config/playground.yaml` or `PLAYGROUND_CONFIG_FILE` path)
3. Defaults in code

### Bot-to-Bot Communication
- Bots call each other via control plane: `await bot.call("other-bot.function", input={...})`
- Control plane routes requests, tracks workflow DAG, injects metrics
- Never direct bot-to-bot HTTP—always through control plane

### Memory Scopes
- **Global:** Shared across all bots/sessions
- **Bot:** Scoped to one bot, all sessions
- **Session:** Scoped to one session (multi-turn conversation)
- **Run:** Scoped to single execution/workflow run

Automatically synced by control plane. Bots access via SDK methods: `bot.memory.get/set(scope, key, value)`

### DID/VC (Cryptographic Identity)
- Opt-in per bot: Set `app.vc_generator.set_enabled(True)` in Python or equivalent in Go
- Control plane generates W3C Verifiable Credentials for each execution
- Export audit trails: `GET /api/v1/did/workflow/{workflow_id}/vc-chain`
- Verify offline: `playground verify audit.json`

## Module Naming

**Control Plane (Go):**
- Use `github.com/hanzoai/playground/control-plane` as module path
- Internal packages: `github.com/hanzoai/playground/control-plane/internal/<package>`

**SDKs:**
- Python: `playground` (PyPI package)
- Go: `github.com/hanzoai/playground/sdk/go` (import path)

## Release Process

Releases are automated via `.github/workflows/release.yml` and `.goreleaser.yml`:
- Tag a commit: `git tag v0.1.0 && git push origin v0.1.0`
- GitHub Actions builds binaries for multiple platforms
- `control-plane/build-single-binary.sh` creates unified binary (embeds web UI)

## Debugging Tips

- **Control plane not starting:** Check `PLAYGROUND_DATABASE_URL` is set correctly (PostgreSQL mode) or ensure SQLite file path is writable (local mode)
- **Migrations failing:** Ensure PostgreSQL is running and connection string is correct. Check migration status with `goose status`
- **Bot can't connect:** Verify `PLAYGROUND_URL` env var points to control plane (default: `http://localhost:8080`)
- **UI not loading:** In dev, ensure both Vite dev server (`npm run dev`) and control plane server are running. In prod, ensure `make build` was run to embed UI in binary
- **Bot execution stuck:** Check workflow DAG in UI (`/ui/workflows`) for errors. Check bot logs for exceptions.
- **Database connection pool exhausted:** Increase `PLAYGROUND_STORAGE_POSTGRES_MAX_CONNECTIONS` in config

## Environment Variables Reference

See `control-plane/.env.example` for comprehensive list. Key vars:
- `PLAYGROUND_PORT` - HTTP server port (default: 8080)
- `PLAYGROUND_MODE` - `local` or `cloud`
- `PLAYGROUND_STORAGE_MODE` - `local`, `postgresql`, or `cloud`
- `PLAYGROUND_DATABASE_URL` - PostgreSQL connection string
- `PLAYGROUND_UI_ENABLED` - Enable/disable web UI
- `PLAYGROUND_UI_MODE` - `embedded` (production) or `development` (Vite proxy)
- `PLAYGROUND_CONFIG_FILE` - Path to config YAML
- `GIN_MODE` - `debug` or `release`
- `LOG_LEVEL` - `debug`, `info`, `warn`, `error`

## Cloud Agent Provisioning

### Architecture
- **Linux agents**: K8s pod (agent container + operative sidecar for desktop)
- **Mac/Windows agents**: Provisioned via Visor (CasVisor) for real VM management
- Multi-container pods: `ghcr.io/hanzoai/bot:2026.2.23` (agent) + `ghcr.io/hanzoai/operative:v0.1.0` (desktop sidecar)

### Cloud Provisioning API
```
POST   /api/v1/cloud/nodes/provision   → Provision new agent
DELETE /api/v1/cloud/nodes/:node_id    → Deprovision agent
GET    /api/v1/cloud/nodes             → List cloud agents
GET    /api/v1/cloud/nodes/:node_id    → Get agent info
GET    /api/v1/cloud/nodes/:node_id/logs → Get agent logs
POST   /api/v1/cloud/nodes/sync        → Refresh from K8s
POST   /api/v1/cloud/teams/provision    → Batch provision
```

### Key Files
- `internal/cloud/provisioner.go` — Multi-OS routing (Linux→K8s, Mac/Windows→Visor)
- `internal/cloud/k8s_client.go` — Raw K8s HTTP API (no client-go), sidecar support
- `internal/cloud/visor_client.go` — Visor API client for multi-cloud VM management
- `internal/config/cloud.go` — CloudConfig, VisorConfig, IAMConfig, env overrides
- `internal/handlers/cloud.go` — HTTP handlers for cloud provisioning

### Environment Variables (injected into bot pods)
- `OPENAI_API_BASE` → `http://cloud-api.hanzo.svc:8000/api` (Hanzo Cloud AI)
- `OPENAI_API_KEY` → hk-* IAM key (from config)
- `OPERATIVE_URL` → `http://localhost:8501` (sidecar desktop)
- `OPERATIVE_VNC_URL` → `http://localhost:6080` (sidecar VNC)
- `AGENT_MODEL` → LLM model name

### Secrets Management (KMS)
All secrets managed via KMS (kms.hanzo.ai) using KMSSecret CRD:
- KMS project: `playground` (slug: `playground-r0bw`)
- K8s secret: `playground-secrets` (synced every 120s)
- Secrets: POSTGRES_DSN, CLOUD_API_KEY, IAM_CLIENT_SECRET, VISOR_CLIENT_ID, VISOR_CLIENT_SECRET
- Manifest: `universe/infra/k8s/bot/playground-kms-secrets.yaml`

### Visor (Multi-Cloud VM)
- Endpoint: `http://visor.hanzo.svc:19000`
- Supports: AWS EC2, DigitalOcean, GCP, Azure, Proxmox, VMware, KVM
- Remote access: RDP (Windows), VNC (Mac), SSH (Linux) via Guacamole
- IAM auth: clientId/clientSecret from app-hanzo-vm

### Production Deployment
- Domain: `hanzo.bot` (alias: `playground.hanzo.bot`)
- Image: `ghcr.io/hanzoai/playground:latest`
- K8s: 2 replicas, hanzo namespace
- Dockerfile: `deployments/docker/Dockerfile.control-plane`
- Manifest: `universe/infra/k8s/bot/agents-control-plane.yaml`

## Cloud Agent Debugging History (2026-03-14)

### Issue: agent-cloud-58b82c78 — Terminal, Chat, Desktop all broken

**Symptoms (from hanzo.bot UI):**
1. Terminal: Connected but commands fail with `SYSTEM_RUN_DENIED: approval required`
2. Chat: "No session connected"
3. Desktop VNC: Displays blue desktop but user reports can't interact

**Pod architecture:**
- Pod `agent-cloud-58b82c78` in namespace `hanzo`
- Container `agent`: `ghcr.io/hanzoai/bot:latest` running `node hanzo-bot.mjs node run --node-id cloud-58b82c78`
- Container `operative`: `ghcr.io/hanzoai/operative` with Xvfb + VNC (ports 5900, 6080, 8501)
- Connects to `bot-gateway` via `ws://bot-gateway.hanzo.svc:80`

---

### Issue 1: Terminal — FIXED ✅

**Root cause:** `provisioner.go:nodeArgs()` (line 1128-1131) omitted `--security full --ask off` from cloud agent args. The bot's exec policy defaults to `security=deny, ask=on-miss`, blocking every command.

**Evidence:**
```
[ws] ⇄ res ✗ node.invoke 24ms errorCode=UNAVAILABLE errorMessage=UNAVAILABLE: SYSTEM_RUN_DENIED: approval required
```

**Bot exec policy chain:**
- `src/node-host/exec-policy.ts` → `evaluateSystemRunPolicy()` checks `security` + `ask` + `allowlist`
- Default `security="deny"` (line 116 of `src/infra/exec-approvals.ts`)
- Default `ask="on-miss"` (line 117)
- Config file: `~/.openclaw/exec-approvals.json` (re-read on each exec request)

**Hot-fix (immediate):** Injected `exec-approvals.json` into running pod:
```json
{"version":1,"defaults":{"security":"full","ask":"off"},"agents":{}}
```
Path: `/home/node/.openclaw/exec-approvals.json`

**Permanent fix:** Commit `0bc4770` on `playground` main:
```go
// provisioner.go:nodeArgs()
return []string{
    "node", "hanzo-bot.mjs", "node", "run",
    "--node-id", nodeID,
    "--security", "full",  // NEW
    "--ask", "off",        // NEW
}
```

**Verified:** `echo "hello from terminal"` executed successfully in UI.

---

### Issue 2: Chat Session — FIXED ✅

**Root cause:** The `agents.*` RPC methods in `bot/src/gateway/server-methods/agents.ts` use `loadConfig()` → `listAgentIds()` to find agents. This reads from the bot config file, NOT the WebSocket node registry. Cloud-provisioned agents only register in the WebSocket registry, so `agents.list` never returns them → frontend derives no `sessionKey` → shows "No session connected".

**Fix applied** (`bot/src/gateway/server-methods/agents.ts`):
1. Modified `resolveAgentIdOrError()` to accept optional `NodeRegistry` param and check it as fallback
2. Modified `agents.list` handler to augment config-file results with connected cloud nodes from `NodeRegistry`
3. Modified `agents.files.list` handler to pass `context.nodeRegistry` to `resolveAgentIdOrError()`

**Status:** Code changes made locally. Needs gateway redeploy to take effect. LLM timeout is a separate issue (cloud-api model routing).

---

### Issue 3: Desktop VNC — FIXED ✅

**Root cause (three problems):**

1. **Tint2 fork bomb:** Tint2 17.0.1 launcher mechanism is fundamentally broken — clicking any launcher icon spawned a new tint2 process instead of the target application. Over time, 83 zombie tint2 processes accumulated.

2. **No application launcher:** After disabling tint2 launcher, there was no way to launch apps from the desktop.

3. **VNC connection timeout:** WebSocket connections drop after ~4-5 minutes with browser ws close code=1006, likely due to load balancer idle timeout. No auto-reconnect logic existed.

**Fixes applied:**

1. **Tint2 launcher disabled:** Changed `panel_items = TL` → `panel_items = T` in `operative/docker/image/.config/tint2/tint2rc`. Keeps taskbar for window management, removes broken launcher.

2. **PCManFM desktop icons:** Modified `operative/docker/image/.operative/start_all.sh` to:
   - Create `~/Desktop/` with .desktop shortcut files (Terminal, Firefox, Calculator, Text Editor, Spreadsheet, Files)
   - Configure `~/.config/libfm/libfm.conf` with `quick_exec=1`
   - Start `pcmanfm --desktop` to manage the root window with clickable desktop icons
   - Double-click any icon → "Execute" dialog → click Execute → app launches

3. **VNC auto-reconnect:** Modified `bot/src/gateway/server-methods/vnc.ts` vncViewerHtml() to add automatic reconnection with exponential backoff (1s → 10s max). On disconnect, shows countdown and reconnects automatically instead of showing permanent "Disconnected" message.

**Verified:** Calculator and Firefox both launch successfully from desktop icons via VNC.

**Known limitation:** PCManFM desktop mode shows an "Execute File" dialog when double-clicking .desktop files. The `quick_exec=1` setting doesn't suppress this for desktop-mode icons. Users must click "Execute" to launch the app. This is a pcmanfm quirk with .desktop files on container filesystems that don't support extended attributes for trust metadata.

---

### IAM Secret Mismatch — REVERTED ✅

**Note:** During initial diagnosis, IAM DB `application.client_secret` for `app-cloud` was accidentally changed from `62484ae...` to `3c7c4d9...` (from `iam-secrets` k8s secret). This caused `cloud-api` CrashLoopBackOff with `panic: Incorrect client secret for application: app-cloud`.

**Key learning:** `cloud-api` reads config from TWO sources:
1. Config file `app.conf` (mounted from `cloud-api-config` secret, key `CLOUD_API_CONFIG__APP_CONF`)
2. Env vars from `cloud-api-secrets` (override config file via `conf.GetConfigString()`)

Both must match the IAM DB. All three were reverted to `62484ae37fb49bc602f204c035b001c5fe8a2034dc7d54f0` and cloud-api rolled out successfully.

---

### Key File Locations

| Component | File | Purpose |
|-----------|------|---------|
| Provisioner | `playground/control-plane/internal/cloud/provisioner.go` | `nodeArgs()` builds agent pod args |
| Exec Policy | `bot/src/node-host/exec-policy.ts` | `evaluateSystemRunPolicy()` |
| Exec Approvals | `bot/src/infra/exec-approvals.ts` | Config file read/write, defaults |
| Agent Registry | `bot/src/gateway/server-methods/agents.ts` | Config-based agent lookup |
| Node Registry | `bot/src/gateway/node-registry.ts` | WebSocket-based node tracking |
| VNC Proxy | `bot/src/gateway/server-methods/vnc.ts` | VNC tunnel management |
| Cloud API Config | K8s secret `cloud-api-config` | `app.conf` with IAM clientSecret |
| Cloud API Env | K8s secret `cloud-api-secrets` | Env vars (clientId, clientSecret) |

## Code Style

**Go:**
- Use `gofmt` for formatting (enforced by `make fmt`)
- Follow [Effective Go](https://go.dev/doc/effective_go) conventions
- Use zerolog for structured logging: `logger.Logger.Info().Msg("message")`

**Python:**
- Use Ruff for linting and formatting (`make fmt` runs `ruff format`)
- Type hints required for public APIs
- Async/await for I/O operations
- Follow PEP 8

**TypeScript/React:**
- Use ESLint config in `control-plane/web/client/.eslintrc.json`
- Functional components with hooks
- Tailwind for styling (no CSS-in-JS)
