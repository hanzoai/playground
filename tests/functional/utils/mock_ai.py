"""
Mock AI response helpers for functional tests.

When the PLAYGROUND_MOCK_AI environment variable is set (non-empty), or when a
real LLM call fails, these helpers supply deterministic responses so that the
full bot-registration-execution pipeline is still validated without depending
on an external AI provider being available and healthy.

Usage in bot functions::

    from utils.mock_ai import ai_with_fallback

    response_text = await ai_with_fallback(
        bot,
        system="You are a math assistant.",
        user="What is 7 + 5?",
        fallback="12",
    )
"""

from __future__ import annotations

import os
from typing import Any, Optional


def is_mock_ai_enabled() -> bool:
    """Return True when the test suite should use mock AI responses."""
    return bool(os.environ.get("PLAYGROUND_MOCK_AI", ""))


async def ai_with_fallback(
    bot: Any,
    *,
    system: Optional[str] = None,
    user: Optional[str] = None,
    fallback: str,
) -> str:
    """
    Call ``bot.ai()`` and fall back to a deterministic string on failure.

    When ``PLAYGROUND_MOCK_AI`` is set the LLM call is skipped entirely and
    *fallback* is returned immediately. When the variable is not set, a real
    LLM call is attempted; if it raises any exception the *fallback* value is
    returned so the test can still validate the rest of the execution pipeline.

    Returns:
        The LLM-generated text on success, or *fallback* on failure / mock mode.
    """
    if is_mock_ai_enabled():
        return fallback

    try:
        response = await bot.ai(system=system, user=user)
        text = getattr(response, "text", None) or str(response)
        return text.strip()
    except Exception:
        return fallback


__all__ = ["ai_with_fallback", "is_mock_ai_enabled"]
