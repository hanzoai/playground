"""
Bot used for validating the memory event pipeline end-to-end.
"""

from __future__ import annotations

import asyncio
import os
from typing import Any, Dict, List, Optional

from playground import Bot
from playground.execution_context import ExecutionContext

from bots import BotSpec

BOT_SPEC = BotSpec(
    key="memory_events_validation",
    display_name="Memory Events Validation Bot",
    default_node_id="memory-events-bot",
    description="Validates @app.on_change callbacks and memory event history.",
    bots=(
        "record_session_preference",
        "clear_session_preference",
        "get_captured_events",
        "get_event_history",
    ),
    skills=(),
)


def create_bot(
    *,
    node_id: Optional[str] = None,
    callback_url: Optional[str] = None,
    **bot_kwargs: Any,
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

    bot._captured_events: List[Dict[str, Any]] = []
    bot._event_lock = asyncio.Lock()

    async def _append_event(record: Dict[str, Any]) -> None:
        async with bot._event_lock:
            bot._captured_events.append(record)

    async def _wait_for_event(scope_id: str, key: str, action: str, timeout: float = 8.0):
        loop = asyncio.get_running_loop()
        deadline = loop.time() + timeout
        cursor = 0

        while True:
            async with bot._event_lock:
                snapshot = list(bot._captured_events)

            for event in snapshot[cursor:]:
                if (
                    event["scope_id"] == scope_id
                    and event["key"] == key
                    and event["action"] == action
                ):
                    return event

            cursor = len(snapshot)
            if loop.time() >= deadline:
                raise asyncio.TimeoutError(
                    f"No memory event for {scope_id}/{key} ({action}) within timeout"
                )
            await asyncio.sleep(0.1)

    async def capture_session_preferences(event):
        record = {
            "event_id": event.id,
            "scope": event.scope,
            "scope_id": event.scope_id,
            "key": event.key,
            "action": event.action,
            "data": event.data,
            "previous_data": event.previous_data,
            "metadata": event.metadata,
            "timestamp": event.timestamp,
        }
        await _append_event(record)
    if not bot.memory_event_client:
        raise RuntimeError("Memory event client is not initialized")
    bot.memory_event_client.subscribe(
        ["preferences.*"], capture_session_preferences
    )

    def _ensure_execution_context(
        execution_context: Optional[ExecutionContext], session_id: str, actor_id: str
    ) -> None:
        if execution_context:
            if not execution_context.session_id:
                execution_context.session_id = session_id
            if not execution_context.actor_id:
                execution_context.actor_id = actor_id

    @bot.bot(name="record_session_preference")
    async def record_session_preference(
        user_id: str,
        preference: str,
        execution_context: Optional[ExecutionContext] = None,
    ) -> Dict[str, Any]:
        session_id = f"session::{user_id}"
        _ensure_execution_context(execution_context, session_id, user_id)

        scoped_memory = bot.memory.session(session_id)
        await scoped_memory.set("preferences.favorite_color", preference)

        try:
            event = await _wait_for_event(session_id, "preferences.favorite_color", "set")
        except asyncio.TimeoutError as exc:  # pragma: no cover - should not happen
            raise RuntimeError("Timed out waiting for memory set event") from exc

        return {
            "session_id": session_id,
            "preference": preference,
            "event": event,
        }

    @bot.bot(name="clear_session_preference")
    async def clear_session_preference(
        user_id: str,
        execution_context: Optional[ExecutionContext] = None,
    ) -> Dict[str, Any]:
        session_id = f"session::{user_id}"
        _ensure_execution_context(execution_context, session_id, user_id)

        scoped_memory = bot.memory.session(session_id)
        await scoped_memory.delete("preferences.favorite_color")

        try:
            event = await _wait_for_event(session_id, "preferences.favorite_color", "delete")
        except asyncio.TimeoutError as exc:  # pragma: no cover - should not happen
            raise RuntimeError("Timed out waiting for memory delete event") from exc

        return {
            "session_id": session_id,
            "event": event,
        }

    @bot.bot(name="get_captured_events")
    async def get_captured_events() -> Dict[str, Any]:
        async with bot._event_lock:
            return {"events": list(bot._captured_events)}

    @bot.bot(name="get_event_history")
    async def get_event_history(limit: int = 5) -> Dict[str, Any]:
        events = await bot.memory.events.history(patterns="preferences.*", limit=limit)
        serialized = [event.to_dict() for event in events]
        return {"history": serialized}

    return bot


__all__ = ["BOT_SPEC", "create_bot"]
