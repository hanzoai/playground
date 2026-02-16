"""
Bot definition that mirrors the documentation Quick Start (demo_echo router).

This matches the `playground init my-bot --defaults` experience described in
`/docs/quick-start` by exposing the router-prefixed `demo_echo` bot that
works without any AI providers configured.
"""

from __future__ import annotations

import os
from typing import Optional

from playground import Bot, BotRouter

from bots import BotSpec

BOT_SPEC = BotSpec(
    key="docs_quick_start",
    display_name="Docs Quick Start Demo Bot",
    default_node_id="my-bot",
    description="Replicates the docs Quick Start flow with the demo_echo bot.",
    bots=("demo_echo",),
    skills=(),
)


def create_bot(
    *,
    node_id: Optional[str] = None,
    callback_url: Optional[str] = None,
    **bot_kwargs,
) -> Bot:
    """
    Build the Quick Start docs bot with the router-prefixed demo echo bot.
    """
    resolved_node_id = node_id or BOT_SPEC.default_node_id

    bot_kwargs.setdefault("dev_mode", True)
    bot_kwargs.setdefault("callback_url", callback_url or "http://test-bot")
    bot_kwargs.setdefault(
        "playground_server", os.environ.get("PLAYGROUND_SERVER", os.environ.get("AGENTS_SERVER", "http://localhost:8080"))
    )
    bot_kwargs.setdefault("version", "1.0.0")

    bot = Bot(
        node_id=resolved_node_id,
        **bot_kwargs,
    )

    bots_router = BotRouter(prefix="demo", tags=["example"])

    @bots_router.bot()
    async def echo(message: str) -> dict:
        """
        Simple echo bot that mirrors the docs Quick Start output.
        """
        response_text = message if isinstance(message, str) else str(message)
        return {
            "original": response_text,
            "echoed": response_text,
            "length": len(response_text),
        }

    bot.include_router(bots_router)
    return bot


def create_bot_from_env() -> Bot:
    """
    Convenience helper mirroring running the generated bot module directly.
    """
    node_id = os.environ.get("HANZO_NODE_ID", os.environ.get("AGENT_NODE_ID"))
    return create_bot(node_id=node_id)


__all__ = ["BOT_SPEC", "create_bot", "create_bot_from_env"]


if __name__ == "__main__":
    # Allow `python -m bots.docs_quick_start_agent` for local debugging.
    bot = create_bot_from_env()
    bot.run()
