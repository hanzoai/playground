# Playground Railway Deployment

Deploy Playground control plane with PostgreSQL and bot nodes on Railway using Docker images.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Railway Project                          │
│                                                              │
│  ┌──────────────────┐    ┌──────────────────┐               │
│  │  Control Plane   │    │    PostgreSQL    │               │
│  │  (Docker Image)  │───▶│   (with pgvector)│               │
│  │                  │    │                  │               │
│  │  - Web UI        │    └──────────────────┘               │
│  │  - REST API      │                                       │
│  │  - Agent Registry│                                       │
│  └────────┬─────────┘                                       │
│           │                                                  │
│           ▼                                                  │
│  ┌──────────────────┐                                       │
│  │   Agent Node     │                                       │
│  │  (Docker Image)  │                                       │
│  │                  │                                       │
│  │  - Reasoners     │                                       │
│  │  - Skills        │                                       │
│  └──────────────────┘                                       │
└─────────────────────────────────────────────────────────────┘
```

## Quick Setup

### 1. Create a New Railway Project

Go to [railway.app](https://railway.app) and create a new empty project.

### 2. Add PostgreSQL

1. Click **New** → **Database** → **Add PostgreSQL**
2. Railway will provision a PostgreSQL instance automatically

### 3. Deploy Control Plane

1. Click **New** → **Docker Image**
2. Enter: `ghcr.io/playground/playground:latest`
3. Add these environment variables:

| Variable | Value | Description |
|----------|-------|-------------|
| `PLAYGROUND_STORAGE_MODE` | `postgres` | Use PostgreSQL backend |
| `PLAYGROUND_STORAGE_POSTGRES_URL` | `${{Postgres.DATABASE_URL}}` | Auto-wired from Railway |
| `PLAYGROUND_API_KEY` | (generate a secure key) | API key for authentication |

4. In **Settings** → **Networking**, click **Generate Domain** to get a public URL
5. Deploy - the control plane will auto-migrate the database on startup

### 4. Deploy a Bot Node (Optional)

1. Click **New** → **Docker Image**
2. Enter: `ghcr.io/playground/init-example:latest`
3. Add these environment variables:

| Variable | Value | Description |
|----------|-------|-------------|
| `PLAYGROUND_URL` | `http://${{control-plane.RAILWAY_PRIVATE_DOMAIN}}:8080` | Internal URL to control plane |
| `PLAYGROUND_API_KEY` | (same as control plane) | Must match control plane key |
| `HANZO_CALLBACK_URL` | `http://${{RAILWAY_PRIVATE_DOMAIN}}:8005` | URL for control plane to reach this bot |
| `PORT` | `8005` | Bot server port |
| `OPENAI_API_KEY` | (your key) | Optional - for AI reasoners |

> **Note:** Replace `control-plane` with your control plane service name if different. The `HANZO_CALLBACK_URL` is critical - without it, the bot will show as "offline" in the UI because the control plane can't reach it for health checks.

## Environment Variables Reference

### Control Plane

| Variable | Required | Description |
|----------|----------|-------------|
| `PLAYGROUND_STORAGE_MODE` | Yes | Set to `postgres` for PostgreSQL |
| `PLAYGROUND_STORAGE_POSTGRES_URL` | Yes | PostgreSQL connection string |
| `PLAYGROUND_API_KEY` | Recommended | API key for authentication |
| `PLAYGROUND_UI_ENABLED` | No | Enable web UI (default: true) |

### Bot Node

| Variable | Required | Description |
|----------|----------|-------------|
| `PLAYGROUND_URL` | Yes | URL to control plane |
| `PLAYGROUND_API_KEY` | Yes* | Must match control plane key |
| `HANZO_CALLBACK_URL` | Yes | URL for control plane to reach this bot for health checks |
| `PORT` | No | Bot HTTP port (default: 8005) |
| `HANZO_NODE_ID` | No | Custom bot/node ID |

*Required if control plane has `PLAYGROUND_API_KEY` set.

## Testing Your Deployment

Once deployed, test the bot via the control plane:

```bash
# Set your control plane URL
export CP_URL=https://your-control-plane.up.railway.app

# Echo reasoner (no AI needed)
curl -X POST $CP_URL/api/v1/execute/init-example.demo_echo \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"input": {"message": "Hello Railway!"}}'

# Sentiment analysis (requires OPENAI_API_KEY on bot)
curl -X POST $CP_URL/api/v1/execute/init-example.demo_analyzeSentiment \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"input": {"text": "I love this deployment!"}}'
```

## Run Bot Locally

Connect a local bot to your Railway control plane:

```bash
# Using the CLI
curl -sSf https://hanzo.bot/get | sh
af init my-agent
cd my-agent

export PLAYGROUND_URL=https://your-control-plane.up.railway.app
export PLAYGROUND_API_KEY=your-api-key
af run

# Or run an example directly
git clone https://github.com/hanzoai/playground.git
cd playground/examples/ts-node-examples/init-example
npm install
PLAYGROUND_URL=https://your-control-plane.up.railway.app \
PLAYGROUND_API_KEY=your-api-key \
npm start
```

## Local Development

For local development with Docker Compose:

```bash
git clone https://github.com/hanzoai/playground.git
cd playground/deployments/docker
docker compose up
```

## Resources

- [Documentation](https://github.com/hanzoai/playground)
- [Examples](https://github.com/hanzoai/playground/tree/main/examples)
- [Python SDK](https://pypi.org/project/playground/)
- [TypeScript SDK](https://www.npmjs.com/package/@hanzo/playground)
