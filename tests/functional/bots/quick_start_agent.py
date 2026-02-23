"""
Bot definition that mirrors the README Quick Start example.

Tests can import `BOT_SPEC` + `create_bot` to obtain a fully configured Bot
without replicating the bot definition inline. Each test can override the
node_id to ensure distinct Playground registrations when multiple nodes run.
"""

from __future__ import annotations

import os
from typing import Dict, Optional

import requests
from playground import AIConfig, Bot

from bots import BotSpec

BOT_SPEC = BotSpec(
    key="quick_start",
    display_name="Quick Start Reference Bot",
    default_node_id="quick-start-bot",
    description="Mirrors README Quick Start sample with fetch_url skill + summarize bot.",
    bots=("summarize",),
    skills=("fetch_url",),
)


def create_bot(
    ai_config: AIConfig,
    *,
    node_id: Optional[str] = None,
    callback_url: Optional[str] = None,
    **bot_kwargs,
) -> Bot:
    """
    Build the Quick Start bot with the canonical fetch_url + summarize flow.
    """
    resolved_node_id = node_id or BOT_SPEC.default_node_id

    bot_kwargs.setdefault("dev_mode", True)
    bot_kwargs.setdefault("callback_url", callback_url or "http://test-bot")
    bot_kwargs.setdefault(
        "agents_server", os.environ.get("PLAYGROUND_SERVER", os.environ.get("AGENTS_SERVER", "http://localhost:8080"))
    )

    bot = Bot(
        node_id=resolved_node_id,
        ai_config=ai_config,
        **bot_kwargs,
    )

    @bot.skill(name="fetch_url")
    def fetch_url(url: str) -> str:
        response = requests.get(url, timeout=10)
        response.raise_for_status()
        return response.text

    @bot.bot(name="summarize")
    async def summarize(url: str) -> Dict[str, str]:
        """
        Fetch a URL, summarize it via OpenRouter, and return metadata.
        """
        content = fetch_url(url)
        truncated = content[:2000]

        ai_response = await bot.ai(
            system=(
                "You summarize documentation for internal verification. "
                "Be concise and focus on the site's purpose."
            ),
            user=(
                "Summarize the following web page in no more than two sentences. "
                "Focus on what the site is intended for.\n"
                f"Content:\n{truncated}"
            ),
        )
        summary_text = getattr(ai_response, "text", str(ai_response)).strip()

        return {
            "url": url,
            "summary": summary_text,
            "content_snippet": truncated[:200],
        }

    return bot


def create_bot_from_env() -> Bot:
    """
    Convenience helper to instantiate the bot from environment variables.

    Useful if you want to run this module as a standalone script.
    """
    api_key = os.environ.get("HANZO_API_KEY", os.environ.get("OPENROUTER_API_KEY", ""))
    if not api_key:
        raise ValueError("HANZO_API_KEY environment variable is required")
    model = os.environ.get("AI_MODEL", os.environ.get("OPENROUTER_MODEL", "openai/google/gemini-2.5-flash-lite"))
    node_id = os.environ.get("HANZO_NODE_ID", os.environ.get("AGENT_NODE_ID"))

    ai_config = AIConfig(
        model=model,
        api_key=api_key,
        base_url=os.environ.get("HANZO_AI_BASE_URL", "https://api.hanzo.ai/v1"),
        temperature=0.7,
        max_tokens=500,
        timeout=60.0,
        retry_attempts=2,
    )
    return create_bot(ai_config, node_id=node_id)


__all__ = ["BOT_SPEC", "create_bot", "create_bot_from_env"]


if __name__ == "__main__":
    # Allow developers to run: `python -m bots.quick_start_agent`
    bot = create_bot_from_env()
    bot.run()
