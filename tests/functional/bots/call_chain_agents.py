"""
Bot definitions used to validate app.call behavior across nodes.
"""

from __future__ import annotations

import os
from typing import Optional

from playground import Bot

from bots import BotSpec

WORKER_SPEC = BotSpec(
    key="call_worker",
    display_name="Call Worker Bot",
    default_node_id="call-worker",
    description="Provides utility bots invoked via app.call.",
    bots=("uppercase_echo",),
    skills=(),
)

ORCHESTRATOR_SPEC = BotSpec(
    key="call_orchestrator",
    display_name="Call Orchestrator Bot",
    default_node_id="call-orchestrator",
    description="Delegates to worker nodes using app.call.",
    bots=("delegate_pipeline",),
    skills=(),
)


def create_worker_bot(
    *,
    node_id: Optional[str] = None,
    callback_url: Optional[str] = None,
    **bot_kwargs,
) -> Bot:
    resolved_node_id = node_id or WORKER_SPEC.default_node_id

    bot_kwargs.setdefault("dev_mode", True)
    bot_kwargs.setdefault("callback_url", callback_url or "http://test-bot")
    bot_kwargs.setdefault(
        "agents_server", os.environ.get("PLAYGROUND_SERVER", os.environ.get("AGENTS_SERVER", "http://localhost:8080"))
    )

    bot = Bot(node_id=resolved_node_id, **bot_kwargs)

    @bot.bot(name="uppercase_echo")
    async def uppercase_echo(text: str) -> dict:
        normalized = text.strip()
        return {
            "text": normalized,
            "upper": normalized.upper(),
            "length": len(normalized),
        }

    return bot


def create_orchestrator_bot(
    *,
    target_node_id: str,
    node_id: Optional[str] = None,
    callback_url: Optional[str] = None,
    **bot_kwargs,
) -> Bot:
    resolved_node_id = node_id or ORCHESTRATOR_SPEC.default_node_id

    bot_kwargs.setdefault("dev_mode", True)
    bot_kwargs.setdefault("callback_url", callback_url or "http://test-bot")
    bot_kwargs.setdefault(
        "agents_server", os.environ.get("PLAYGROUND_SERVER", os.environ.get("AGENTS_SERVER", "http://localhost:8080"))
    )

    bot = Bot(node_id=resolved_node_id, **bot_kwargs)

    @bot.bot(name="delegate_pipeline")
    async def delegate_pipeline(text: str) -> dict:
        delegated = await bot.call(f"{target_node_id}.uppercase_echo", text=text)
        return {
            "original": text,
            "delegated": delegated,
            "tokens": len(text.split()),
        }

    return bot


__all__ = [
    "WORKER_SPEC",
    "ORCHESTRATOR_SPEC",
    "create_worker_bot",
    "create_orchestrator_bot",
]
