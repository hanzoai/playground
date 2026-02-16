"""
Bot exposing bots via BotRouter prefixes.
"""

from __future__ import annotations

import os
from typing import Optional

from playground import Bot, BotRouter

from bots import BotSpec

BOT_SPEC = BotSpec(
    key="router_prefix",
    display_name="Router Prefix Bot",
    default_node_id="router-prefix-bot",
    description="Validates router-prefixed bot registration and execution.",
    bots=("tools_echo", "tools_status"),
    skills=(),
)


def create_bot(
    *,
    node_id: Optional[str] = None,
    callback_url: Optional[str] = None,
    **bot_kwargs,
) -> Bot:
    resolved_node_id = node_id or BOT_SPEC.default_node_id

    bot_kwargs.setdefault("dev_mode", True)
    bot_kwargs.setdefault("callback_url", callback_url or "http://test-bot")
    bot_kwargs.setdefault(
        "playground_server", os.environ.get("PLAYGROUND_SERVER", os.environ.get("AGENTS_SERVER", "http://localhost:8080"))
    )

    bot = Bot(
        node_id=resolved_node_id,
        **bot_kwargs,
    )

    tools_router = BotRouter(prefix="tools")

    @tools_router.bot()
    async def echo(message: str) -> dict:
        return {"message": message, "length": len(message)}

    @tools_router.bot()
    async def status() -> dict:
        return {
            "node_id": bot.node_id,
            "router_prefix": "tools",
            "bots": sorted(r.get("id") for r in bot.bots),
        }

    bot.include_router(tools_router)
    return bot


__all__ = ["BOT_SPEC", "create_bot"]
