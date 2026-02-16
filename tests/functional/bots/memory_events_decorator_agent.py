"""Functional bot exercising memory event decorator APIs."""

from __future__ import annotations

import asyncio
import os
from typing import Any, Dict, List, Optional

from playground import Bot

from bots import BotSpec


GENERAL_SESSION_ID = "decorator::general-session"
SCOPED_SESSION_ID = "decorator::scoped-session"
EXACT_KEY = "decorator.preferences.exact"
WILDCARD_KEY = "decorator.preferences.theme"
NESTED_KEY = "decorator.settings.layout.primary"
MULTI_WILDCARD_KEY = "decorator.features.beta.flag.rollout"
MULTI_PATTERN_FIRST_KEY = "decorator.multi.first"
MULTI_PATTERN_SECOND_KEY = "decorator.multi.second"
SESSION_KEY = "decorator.session.preference"
GLOBAL_KEY = "decorator.global.feature_flag"

LISTENER_LABELS = {
    "exact": "app.memory::exact",
    "wildcard": "app.memory::wildcard",
    "nested": "app.memory::nested",
    "multi_wildcard": "app.memory::multi-wildcard",
    "multi_pattern": "app.memory::multi-pattern",
    "session": "session.memory::scoped",
    "global": "global.memory::decorator",
}


BOT_SPEC = BotSpec(
    key="memory_events_decorator_validation",
    display_name="Memory Events Decorator Bot",
    default_node_id="memory-events-decorator-bot",
    description="Validates decorator-based memory event subscriptions across scopes.",
    bots=(
        "reset_decorator_events",
        "fire_exact_pattern",
        "fire_wildcard_pattern",
        "fire_nested_pattern",
        "fire_multi_wildcard_pattern",
        "fire_multi_pattern",
        "fire_session_scope_pattern",
        "fire_global_scope_pattern",
        "get_decorator_events",
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

    bot._decorator_events: List[Dict[str, Any]] = []
    bot._decorator_lock = asyncio.Lock()

    general_session_memory = bot.memory.session(GENERAL_SESSION_ID)
    session_scoped_memory = bot.memory.session(SCOPED_SESSION_ID)
    global_memory = bot.memory.global_scope

    async def _record_event(listener: str, event) -> None:
        record = {
            "listener": listener,
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
        async with bot._decorator_lock:
            bot._decorator_events.append(record)

    async def _event_cursor() -> int:
        async with bot._decorator_lock:
            return len(bot._decorator_events)

    async def _wait_for_listener_event(
        listener: str,
        *,
        key: str,
        scope: Optional[str],
        scope_id: Optional[str],
        start_index: int,
        action: str = "set",
        timeout: float = 8.0,
    ) -> Dict[str, Any]:
        loop = asyncio.get_running_loop()
        deadline = loop.time() + timeout
        cursor = start_index

        while True:
            async with bot._decorator_lock:
                snapshot = list(bot._decorator_events)

            for event in snapshot[cursor:]:
                if event["listener"] != listener:
                    continue
                if event["key"] != key:
                    continue
                if event["action"] != action:
                    continue
                if scope and event["scope"] != scope:
                    continue
                if scope_id and event["scope_id"] != scope_id:
                    continue
                return event

            cursor = len(snapshot)
            if loop.time() >= deadline:
                raise asyncio.TimeoutError(
                    f"No memory event matched listener={listener} key={key}"
                )
            await asyncio.sleep(0.1)

    @bot.memory.on_change(EXACT_KEY)
    async def _capture_exact(event) -> None:
        await _record_event(LISTENER_LABELS["exact"], event)

    @bot.memory.on_change("decorator.preferences.*")
    async def _capture_wildcard(event) -> None:
        await _record_event(LISTENER_LABELS["wildcard"], event)

    @bot.memory.on_change("decorator.settings.*.primary")
    async def _capture_nested(event) -> None:
        await _record_event(LISTENER_LABELS["nested"], event)

    @bot.memory.on_change("decorator.features.*.flag.*")
    async def _capture_multi_wildcard(event) -> None:
        await _record_event(LISTENER_LABELS["multi_wildcard"], event)

    @bot.memory.on_change([MULTI_PATTERN_FIRST_KEY, MULTI_PATTERN_SECOND_KEY])
    async def _capture_multi_pattern(event) -> None:
        await _record_event(LISTENER_LABELS["multi_pattern"], event)

    @session_scoped_memory.on_change("decorator.session.*")
    async def _capture_session_scoped(event) -> None:
        await _record_event(LISTENER_LABELS["session"], event)

    @global_memory.on_change("decorator.global.*")
    async def _capture_global_scoped(event) -> None:
        await _record_event(LISTENER_LABELS["global"], event)

    @bot.bot(name="reset_decorator_events")
    async def reset_decorator_events() -> Dict[str, Any]:
        async with bot._decorator_lock:
            bot._decorator_events.clear()
        return {"reset": True}

    @bot.bot(name="fire_exact_pattern")
    async def fire_exact_pattern(value: str) -> Dict[str, Any]:
        cursor = await _event_cursor()
        await general_session_memory.set(EXACT_KEY, value)
        event = await _wait_for_listener_event(
            LISTENER_LABELS["exact"],
            key=EXACT_KEY,
            scope="session",
            scope_id=GENERAL_SESSION_ID,
            start_index=cursor,
        )
        return {"event": event}

    @bot.bot(name="fire_wildcard_pattern")
    async def fire_wildcard_pattern(value: str) -> Dict[str, Any]:
        cursor = await _event_cursor()
        await general_session_memory.set(WILDCARD_KEY, value)
        event = await _wait_for_listener_event(
            LISTENER_LABELS["wildcard"],
            key=WILDCARD_KEY,
            scope="session",
            scope_id=GENERAL_SESSION_ID,
            start_index=cursor,
        )
        return {"event": event}

    @bot.bot(name="fire_nested_pattern")
    async def fire_nested_pattern(value: str) -> Dict[str, Any]:
        cursor = await _event_cursor()
        await general_session_memory.set(NESTED_KEY, value)
        event = await _wait_for_listener_event(
            LISTENER_LABELS["nested"],
            key=NESTED_KEY,
            scope="session",
            scope_id=GENERAL_SESSION_ID,
            start_index=cursor,
        )
        return {"event": event}

    @bot.bot(name="fire_multi_wildcard_pattern")
    async def fire_multi_wildcard_pattern(value: str) -> Dict[str, Any]:
        cursor = await _event_cursor()
        await general_session_memory.set(MULTI_WILDCARD_KEY, value)
        event = await _wait_for_listener_event(
            LISTENER_LABELS["multi_wildcard"],
            key=MULTI_WILDCARD_KEY,
            scope="session",
            scope_id=GENERAL_SESSION_ID,
            start_index=cursor,
        )
        return {"event": event}

    @bot.bot(name="fire_multi_pattern")
    async def fire_multi_pattern(target: str, value: str) -> Dict[str, Any]:
        if target not in {"first", "second"}:
            raise ValueError("target must be 'first' or 'second'")

        key = (
            MULTI_PATTERN_FIRST_KEY if target == "first" else MULTI_PATTERN_SECOND_KEY
        )
        cursor = await _event_cursor()
        await general_session_memory.set(key, value)
        event = await _wait_for_listener_event(
            LISTENER_LABELS["multi_pattern"],
            key=key,
            scope="session",
            scope_id=GENERAL_SESSION_ID,
            start_index=cursor,
        )
        return {"event": event}

    @bot.bot(name="fire_session_scope_pattern")
    async def fire_session_scope_pattern(value: str) -> Dict[str, Any]:
        cursor = await _event_cursor()
        await session_scoped_memory.set(SESSION_KEY, value)
        event = await _wait_for_listener_event(
            LISTENER_LABELS["session"],
            key=SESSION_KEY,
            scope="session",
            scope_id=SCOPED_SESSION_ID,
            start_index=cursor,
        )
        return {"event": event}

    @bot.bot(name="fire_global_scope_pattern")
    async def fire_global_scope_pattern(value: str) -> Dict[str, Any]:
        cursor = await _event_cursor()
        await global_memory.set(GLOBAL_KEY, value)
        event = await _wait_for_listener_event(
            LISTENER_LABELS["global"],
            key=GLOBAL_KEY,
            scope="global",
            scope_id=None,
            start_index=cursor,
        )
        return {"event": event}

    @bot.bot(name="get_decorator_events")
    async def get_decorator_events() -> Dict[str, Any]:
        async with bot._decorator_lock:
            return {"events": list(bot._decorator_events)}

    return bot


__all__ = ["BOT_SPEC", "create_bot", "LISTENER_LABELS"]
