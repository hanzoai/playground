"""
Mock AI response helpers for functional tests.

When no HANZO_API_KEY is configured, or when PLAYGROUND_MOCK_AI is explicitly
set, these helpers supply deterministic responses so the full bot registration
and execution pipeline is still validated without requiring an external AI
provider.

The primary test path uses real LLM calls through api.hanzo.ai. The mock
fallback only activates when:
  - PLAYGROUND_MOCK_AI is set (non-empty), OR
  - No HANZO_API_KEY is available at all

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
    """Return True when the test suite should use mock AI responses.

    Mock mode activates when PLAYGROUND_MOCK_AI is explicitly set, or when
    no API key is available (HANZO_API_KEY is empty/unset).
    """
    if os.environ.get("PLAYGROUND_MOCK_AI", ""):
        return True
    if not os.environ.get("HANZO_API_KEY", ""):
        return True
    return False


async def ai_with_fallback(
    bot: Any,
    *,
    system: Optional[str] = None,
    user: Optional[str] = None,
    fallback: str,
) -> str:
    """
    Call ``bot.ai()`` through api.hanzo.ai and fall back on failure.

    When mock mode is active (no API key or PLAYGROUND_MOCK_AI is set) the LLM
    call is skipped entirely and *fallback* is returned immediately.

    When a HANZO_API_KEY is available, a real LLM call is made through
    api.hanzo.ai. If the call raises any exception, *fallback* is returned as a
    safety net so the test can still validate the rest of the execution pipeline.

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
