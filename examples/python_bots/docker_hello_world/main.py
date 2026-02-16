"""
Docker/Kubernetes-friendly Hello World (Python)

This example is designed to validate the full Playground execution path:

client -> control plane (/api/v1/execute) -> agent callback URL -> response

It is intentionally deterministic (no LLM credentials required).
"""

import os

from playground import Bot


app = Bot(
    node_id=os.getenv("HANZO_NODE_ID", "demo-python-bot"),
    playground_server=os.getenv("PLAYGROUND_URL", "http://localhost:8080"),
    dev_mode=True,
)


@app.bot()
async def hello(name: str = "Playground") -> dict:
    return {"greeting": f"Hello, {name}!", "node_id": app.node_id}


@app.bot()
async def demo_echo(message: str = "Hello!") -> dict:
    return {"echo": message, "node_id": app.node_id}


if __name__ == "__main__":
    port = int(os.getenv("PORT", "8001"))
    # For containerized runs, set HANZO_CALLBACK_URL so the control plane can call back:
    #   HANZO_CALLBACK_URL=http://<service-name>:<port>
    app.run(host="0.0.0.0", port=port, auto_port=False)

