"""Backward-compatible module alias.

``playground.agent`` was renamed to ``playground.bot``.  This shim makes
``playground.agent`` an alias for the same module object so that both
``from playground.agent import X`` and ``monkeypatch.setattr("playground.agent.X", ...)``
work identically to their ``playground.bot`` equivalents.
"""

import sys
from playground import bot as _bot  # noqa: F401

# Replace this module in sys.modules with the real bot module so that
# ``playground.agent`` IS ``playground.bot`` (same object).  This ensures
# monkeypatch.setattr("playground.agent.<name>", ...) patches the actual
# symbol used at runtime.
sys.modules[__name__] = _bot
