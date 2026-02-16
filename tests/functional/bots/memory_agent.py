"""
Bot used for validating app.memory behaviors across scopes.
"""

from __future__ import annotations

import os
from typing import Optional

from playground import Bot
from playground.execution_context import ExecutionContext

from bots import BotSpec

BOT_SPEC = BotSpec(
    key="memory_validation",
    display_name="Memory Validation Bot",
    default_node_id="memory-bot",
    description="Exercises session, actor, and global memory scopes.",
    bots=("remember_user",),
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

    @bot.bot(name="remember_user")
    async def remember_user(
        user_id: str,
        message: str,
        actor_id: Optional[str] = None,
        execution_context: Optional[ExecutionContext] = None,
    ) -> dict:
        """
        Persist user message history using app.memory scoped helpers.
        """
        if execution_context and not execution_context.session_id:
            execution_context.session_id = f"session::{user_id}"

        session_scope = bot.memory.session(user_id)
        history = await session_scope.get("history", default=[])
        history.append(message)
        await session_scope.set("history", history)

        global_scope = bot.memory.global_scope
        global_key = f"user::{user_id}::count"
        global_count = int(await global_scope.get(global_key, default=0) or 0) + 1
        await global_scope.set(global_key, global_count)

        key_exists = await global_scope.exists(global_key)
        recent = history[-5:]
        return {
            "user_id": user_id,
            "messages_seen": global_count,
            "recent_history": recent,
            "global_key_exists": key_exists,
        }

    return bot


__all__ = ["BOT_SPEC", "create_bot"]
