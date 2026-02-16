"""
Reusable bot definitions for functional tests.

Each module in this package should expose:
    - `BOT_SPEC`: metadata describing the bot node
    - `create_bot(openrouter_config, **kwargs)`: factory returning a Bot
"""

from dataclasses import dataclass
from typing import Sequence


@dataclass(frozen=True)
class BotSpec:
    """
    Metadata describing a functional-test bot node.

    Attributes:
        key: Unique identifier for the bot definition (module-level)
        display_name: Human-friendly label for docs/logs
        default_node_id: Canonical node ID; tests may override this per instance
        description: Summary of what this bot does
        bots: Collection of bot IDs exposed by the bot
        skills: Collection of skill IDs exposed by the bot (optional)
    """

    key: str
    display_name: str
    default_node_id: str
    description: str
    bots: Sequence[str]
    skills: Sequence[str] = ()


__all__ = ["BotSpec"]
