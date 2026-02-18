<div align="center">

<img src="assets/github hero.png" alt="Playground - Kubernetes, for AI Bots" width="100%" />

# Kubernetes for AI Bots

### **Deploy, Scale, Observe, and Prove.**

*When AI moves from chatbots into backends, making decisions, not just answering questions, it needs infrastructure, not frameworks.*

[![License](https://img.shields.io/badge/license-Apache%202.0-7c3aed.svg?style=flat&labelColor=1e1e2e)](LICENSE)
[![Downloads](https://img.shields.io/endpoint?url=https%3A%2F%2Fgist.githubusercontent.com%2Fsantoshkumarradha%2Fd98e2ad73502b4075f6a5f0ae4f5cae5%2Fraw%2Fbadge.json&style=flat&logo=download&logoColor=white&labelColor=1e1e2e&cacheSeconds=3600)](https://github.com/hanzoai/playground)
[![Last Commit](https://img.shields.io/github/last-commit/hanzoai/playground?style=flat&logo=git&logoColor=white&color=7c3aed&labelColor=1e1e2e)](https://github.com/hanzoai/playground/commits/main)
[![Go](https://img.shields.io/badge/go-1.21+-00ADD8.svg?style=flat&labelColor=1e1e2e&logo=go&logoColor=white)](https://go.dev/)
[![Python](https://img.shields.io/badge/python-3.9+-3776AB.svg?style=flat&labelColor=1e1e2e&logo=python&logoColor=white)](https://www.python.org/)
[![Deploy with Docker](https://img.shields.io/badge/deploy-docker-2496ED.svg?style=flat&labelColor=1e1e2e&logo=docker&logoColor=white)](https://docs.docker.com/)
[![Discord](https://img.shields.io/badge/discord-join%20us-5865F2.svg?style=flat&labelColor=1e1e2e&logo=discord&logoColor=white)](https://discord.gg/aBHaXMkpqh)

**[Docs](https://hanzo.bot/docs)** | **[Quick Start](https://hanzo.bot/docs/quick-start)** | **[Python SDK](https://hanzo.bot/api/python-sdk/overview)** | **[Go SDK](https://hanzo.bot/api/go-sdk/overview)** | **[TypeScript SDK](https://hanzo.bot/api/typescript-sdk/overview)** | **[REST API](https://hanzo.bot/api/rest-api/overview)** | **[Discord](https://discord.gg/aBHaXMkpqh)**

</div>

## What is Playground?

**Playground is the backend infrastructure layer for autonomous AI.**

AI has outgrown frameworks and is moving from chatbots into backends—making decisions about refunds, coordinating supply chains, managing portfolios. These bots need infrastructure, not prompt wrappers.

Playground is an open-source **control plane** that treats AI bots as first-class backend services and makes bots production-ready.

**Scale Infrastructure** *(think: Kubernetes)*
- **Routing & Discovery**: Bots find and call each other through standard REST APIs
- **Async Execution**: Fire-and-forget tasks that run for minutes, hours, or days
- **Durable State**: Built-in memory with vector search—no Redis or Pinecone required
- **Observability**: Automatic workflow DAGs, Prometheus metrics, structured logs

**Trust Infrastructure** *(think: Okta, rebuilt for bots)*
- **W3C DIDs**: Every bot gets a cryptographic identity—not a shared API key
- **Verifiable Credentials**: Tamper-proof audit trails for every action
- **Policy Enforcement**: Boundaries enforced by infrastructure, not prompts

Write [Python](https://hanzo.bot/api/python-sdk/overview), [Go](https://hanzo.bot/api/go-sdk/overview), [TypeScript](https://hanzo.bot/api/typescript-sdk/overview), or call via [REST](https://hanzo.bot/api/rest-api/overview). Get production infrastructure automatically.

---

## The AI Backend

Software keeps adding layers when complexity demands it. Frontend/backend separation. Data lakes and pipelines. Now: a **reasoning layer** that sits alongside your services, making decisions that used to be hardcoded.

We call this the AI Backend. Not a chatbot, not a copilot—infrastructure for software that can think.

**Guided autonomy:** Bots that reason freely within boundaries you define. Predictable enough to trust. Flexible enough to be useful.

📖 **[Read: The AI Backend](https://hanzo.bot/blog/posts/ai-backend/?utm_source=github-readme)** — Our thesis on why every serious backend will need a reasoning layer.

---

## See It In Action

<div align="center">
<img src="assets/UI.png" alt="Playground Dashboard" width="100%" />
<br/>
<i>Real-time Observability • Execution Flow • Audit Trails</i>
</div>

---

## Build Bots in Any Language

<details open>
<summary><strong>Python</strong></summary>

```python
from playground import Bot, AIConfig

app = Bot(node_id="researcher", ai_config=AIConfig(model="gpt-4o"))

@app.skill()
def fetch_url(url: str) -> str:
    return requests.get(url).text

@app.reasoner()
async def summarize(url: str) -> dict:
    content = fetch_url(url)
    return await app.ai(f"Summarize: {content}")

app.run()  # → POST /api/v1/execute/researcher.summarize
```

[Full Python SDK Documentation →](https://hanzo.bot/api/python-sdk/overview)
</details>

<details>
<summary><strong>Go</strong></summary>

```go
bot, _ := playgroundbot.New(playgroundbot.Config{
    NodeID:        "researcher",
    PlaygroundURL: "http://localhost:8080",
})

bot.RegisterSkill("summarize", func(ctx context.Context, input map[string]any) (any, error) {
    url := input["url"].(string)
    // Your bot logic here
    return map[string]any{"summary": "..."}, nil
})

bot.Run(context.Background())
```

[Full Go SDK Documentation →](https://hanzo.bot/api/go-sdk/overview)
</details>

<details>
<summary><strong>TypeScript</strong></summary>

```typescript
import { Bot } from '@hanzo/playground';

const bot = new Bot({
  nodeId: 'researcher',
  playgroundUrl: 'http://localhost:8080',
});

bot.reasoner('summarize', async (ctx, input: { url: string }) => {
  const content = await fetch(input.url).then(r => r.text());
  return await ctx.ai(`Summarize: ${content}`);
});

bot.run();  // → POST /api/v1/execute/researcher.summarize
```

[Full TypeScript SDK Documentation →](https://hanzo.bot/api/typescript-sdk/overview)
</details>

<details>
<summary><strong>REST / Any Language</strong></summary>

```bash
# Call any bot from anywhere—no SDK required
curl -X POST http://localhost:8080/api/v1/execute/researcher.summarize \
  -H "Content-Type: application/json" \
  -d '{"input": {"url": "https://example.com"}}'
```

```javascript
// Frontend (React, Next.js, etc.)
const result = await fetch("http://localhost:8080/api/v1/execute/researcher.summarize", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ input: { url: "https://example.com" } }),
}).then(r => r.json());
```

[REST API Reference →](https://hanzo.bot/api/rest-api/overview)
</details>

---

## Quick Start

### 1. Install

```bash
curl -fsSL https://hanzo.bot/install.sh | bash
```

### 2. Create Your Bot

```bash
playground init my-bot --defaults
cd my-bot && pip install -r requirements.txt
```

### 3. Start (Two Terminals Required)

Playground uses a **control plane + Node** architecture. You'll need two terminal windows:

**Terminal 1 – Start the Control Plane:**
```bash
playground server
```
> Opens the dashboard at http://localhost:8080

**Terminal 2 – Start Your Bot:**
```bash
python main.py
```
> Bot auto-registers with the control plane

### 4. Test It

```bash
curl -X POST http://localhost:8080/api/v1/execute/my-bot.demo_echo \
  -H "Content-Type: application/json" \
  -d '{"input": {"message": "Hello!"}}'
```

<details>
<summary><strong>Other Languages / Options</strong></summary>

**Go:**
```bash
playground init my-bot --defaults --language go
cd my-bot && go mod download
go run .
```

**TypeScript:**
```bash
playground init my-bot --defaults --language typescript
cd my-bot && npm install
npm run dev
```

**Interactive mode** (choose language, set author info):
```bash
playground init my-bot  # No --defaults flag
```
</details>

<details>
<summary><strong>Docker / Troubleshooting</strong></summary>

If running the **control plane in Docker** and your **Node runs outside that container**, make sure the control plane can reach the bot at the URL it registers.

**Option A (bot on your host, control plane in Docker):**
```bash
docker run -p 8080:8080 playground/control-plane:latest

# Python bots (recommended)
export PLAYGROUND_URL="http://localhost:8080"
export HANZO_CALLBACK_URL="http://host.docker.internal:8001"
python main.py

# Go bots
export PLAYGROUND_URL="http://localhost:8080"
export HANZO_PUBLIC_URL="http://host.docker.internal:8001"
```

**Option B (bot + control plane both in Docker Compose / same network):**
- Set the bot callback/public URL to the bot container's service name, e.g. `http://my-bot:8001`.

**Linux note:** `host.docker.internal` may require `--add-host=host.docker.internal:host-gateway` or using a Compose setup where both containers share a network.
</details>

**Next Steps:** [Build Your First Bot](https://hanzo.bot/guides/getting-started/build-your-first-bot) | [Deploy to Production](https://hanzo.bot/guides/deployment/overview) | [Examples](https://hanzo.bot/examples)

---

## Production Examples

Real-world patterns built on Playground:

| Example | Description | Links |
|---------|-------------|-------|
| **Deep Research API** | Massively parallel research backend. Fans out to 10k+ bots, synthesizing verifiable strategies with deep citation chains. | [GitHub](https://github.com/hanzoai/playground-deep-research) • [Docs](https://hanzo.bot/examples) |
| **RAG Evaluator** | Production monitoring for LLM responses. Scores across 4 dimensions to identify reliability issues. | [Architecture](https://hanzo.bot/examples/complete-bots/rag-evaluator) |

[See all examples →](https://hanzo.bot/examples)

---

## The Production Gap

Most frameworks stop at "make the LLM call." But production bots need:

[See the production-ready feature set →](https://hanzo.bot/docs/why-playground/production-ready-features)

### Scale & Reliability
Bots that run for hours or days. Webhooks with automatic retries. Backpressure handling when downstream services are slow.

```python
# Fire-and-forget: webhook called when done
result = await app.call(
    "research_agent.deep_dive",
    input={"topic": "quantum computing"},
    async_config=AsyncConfig(
        webhook_url="https://myapp.com/webhook",
        timeout_hours=6
    )
)
```

### Multi-Bot Coordination
Bots that discover and invoke each other through the control plane. Every call tracked. Every workflow visualized as a DAG.

```python
# Bot A calls Bot B—routed through control plane, fully traced
analysis = await app.call("analyst.evaluate", input={"data": dataset})
report = await app.call("writer.summarize", input={"analysis": analysis})
```

### Developer Experience
Standard REST APIs. No magic abstractions. Build bots the way you build microservices.

```bash
# Every bot is an API endpoint
curl -X POST http://localhost:8080/api/v1/execute/researcher.summarize \
  -H "Content-Type: application/json" \
  -d '{"input": {"url": "https://example.com"}}'
```

### Enterprise Ready
Cryptographic identity for every bot. Tamper-proof audit trails for every action. [Learn more about Identity & Trust](https://hanzo.bot/docs/core-concepts/identity-and-trust).

---

## A New Backend Paradigm

Playground isn't a framework you extend. It's infrastructure you deploy on.

[See how Playground compares to bot frameworks →](https://hanzo.bot/docs/why-playground/vs-bot-frameworks)

|                    | Bot Frameworks           | DAG/Workflow Engines    | Playground                              |
| ------------------ | -------------------------- | ----------------------- | --------------------------------------- |
| **Architecture**   | Monolithic scripts         | Predetermined pipelines | Distributed microservices               |
| **Execution**      | Synchronous, blocking      | Scheduled, batch        | Async-native (webhooks, SSE, WebSocket) |
| **Coordination**   | Manual message passing     | Central scheduler       | Service mesh with discovery             |
| **Memory**         | External (Redis, Pinecone) | External                | Built-in + vector search                |
| **Multi-language** | SDK-locked                 | Config files            | Native REST APIs (any language)         |
| **Long-running**   | Timeouts, hacks            | Designed for batch      | Hours/days, durable execution           |
| **Audit**          | Logs (trust me)            | Logs                    | Cryptographic proofs (W3C DIDs/VCs)     |

### Performance

**Playground SDKs at Scale** (100,000 handlers)

| | Go | TypeScript | Python |
|---|---:|---:|---:|
| Registration | 17 ms | 14 ms | ~5.7 s |
| Memory/Handler | 280 B | 276 B | 7.5 KB |
| Throughput | 8.2M req/s | 4.0M req/s | 6.7M req/s |

**vs Other Frameworks** (1,000 handlers, same language)

| | Playground | LangChain | CrewAI | Mastra |
|---|---:|---:|---:|---:|
| Registration | 57 ms (py) / 14 ms (ts) | 483 ms | 200 ms | 365 ms |
| Memory/Handler | 7.5 KB (py) / 276 B (ts) | 10.8 KB | 14.3 KB | 1.8 KB |

<sub>Apple M1. Handler registration + invocation overhead (no LLM). [Methodology →](examples/benchmarks/100k-scale/)</sub>

**Not a DAG builder.** Bots decide what to do next—dynamically. The control plane tracks the execution graph automatically.

**Not tool attachment.** You don't just give an LLM a bag of MCP tools and hope. You define **Reasoners** (AI logic) and **Skills** (deterministic code) with explicit boundaries. [Learn more](https://hanzo.bot/docs/core-concepts/reasoners-and-skills).

---

## Key Features

### Scale Infrastructure
- **Control Plane**: Stateless Go service that routes, tracks, and orchestrates
- **Async by Default**: Fire-and-forget or wait. Webhooks with retries. SSE streaming.
- **Long-Running**: Tasks that run for hours or days with durable checkpointing
- **Backpressure**: Built-in queuing and circuit breakers

### Multi-Bot Native
- **Discovery**: Bots register capabilities. Others find them via API.
- **Cross-Bot Calls**: `app.call("other.reasoner", input={...})` routed through control plane
- **Workflow DAGs**: Every execution path visualized automatically
- **Shared Memory**: Scoped to global, bot, session, or run—with vector search

### Enterprise Ready
- **W3C DIDs**: Every bot gets a cryptographic identity
- **Verifiable Credentials**: Tamper-proof receipts for every action
- **Prometheus Metrics**: `/metrics` endpoint out of the box
- **Policy Enforcement**: "Only bots signed by 'Finance' can access this tool"

[Explore the full feature set →](https://hanzo.bot/docs/features)


## Identity & Trust

When bots move from answering questions to making decisions, approving refunds, coordinating supply chains, moving money, "check the logs" isn't enough.

Playground gives every bot a [W3C Decentralized Identifier (DID)](https://www.w3.org/TR/did-core/)—a cryptographic identity. Every execution produces a Verifiable Credential: a tamper-proof receipt showing exactly what happened, who authorized it, and the full delegation chain.

```bash
# Export audit trail for any workflow
curl http://localhost:8080/api/ui/v1/workflows/{workflow_id}/vc-chain
```

For compliance teams: mathematical proof, not trust.

📖 **[Read: IAM for AI Backends](https://hanzo.bot/blog/posts/iam-ai-backends)** — Why OAuth can't secure autonomous software, and what replaces it.

[Full documentation →](https://hanzo.bot/docs/core-concepts/identity-and-trust)



## Architecture

<div align="center">
<img src="assets/arch.png" alt="Playground Architecture Diagram" width="80%" />
</div>

[Learn more about the core architecture →](https://hanzo.bot/docs/why-playground/core-architecture)



## Is Playground for you?

### Yes if:
- You're building an **AI backend** - bots that make decisions, not just answer questions
- You're building **multi-bot systems** that need to coordinate
- You need **production infrastructure**: async, retries, observability
- You want bots as **standard backend services** with REST APIs
- You need **audit trails** for compliance or debugging
- You have **multiple teams** deploying bots independently

### Not yet if:
- You're building a **single chatbot** (prompt orchestration frameworks like LangChain, CrewAI, LlamaIndex etc.. are great for that)
- You're **prototyping** and don't need production concerns yet

*When you're ready to ship bots to production, we'll be here.*

---

If you are **Backend Engineers** shipping AI into production who want standard APIs, not magic or **Platform Teams** who don't want to build another homegrown orchestrator or **Enterprise Teams** in regulated industries (Finance, Health) needing audit trails or **Frontend Developers** who just want to `fetch()` a bot without Python headaches, Playground is built for you.

---

## Learn More

- 📖 **[The AI Backend](https://hanzo.bot/blog/posts/ai-backend)** — Why every backend needs a reasoning layer
- 📖 **[IAM for AI Backends](https://hanzo.bot/blog/posts/iam-ai-backends)** — Why bots need identity, not just API keys
- 📚 **[Documentation](https://hanzo.bot/docs)** — Full technical reference
- 🚀 **[Examples](https://hanzo.bot/examples)** — Production patterns and use cases

---

## Community

**Bots are becoming part of production backends. They need identity, governance, and infrastructure. That's why Playground exists.**

<div align="center">

[![Discord](https://img.shields.io/badge/Join%20our%20Discord-5865F2?style=for-the-badge&logo=discord&logoColor=white)](https://discord.gg/aBHaXMkpqh)

*Ask questions, share what you're building, get help from the team*

</div>

- **[Documentation](https://hanzo.bot/docs)**
- **[GitHub Issues](https://github.com/hanzoai/playground/issues)**
- **[Twitter/X](https://x.com/hanzo.bot)**
- **[Examples](https://hanzo.bot/examples)**

<p align="center">
  <strong>Built by developers who got tired of duct-taping bots together.</strong><br>
  <a href="https://hanzo.bot">hanzo.bot</a>
</p>
