from types import MethodType, SimpleNamespace

import pytest

from playground.bot import Agent
from playground.bot_registry import set_current_bot, clear_current_bot


@pytest.mark.asyncio
async def test_call_local_bot_argument_mapping():
    agent = object.__new__(Agent)
    agent.node_id = "node"
    agent.agents_connected = True
    agent.dev_mode = False
    agent.async_config = SimpleNamespace(
        enable_async_execution=False, fallback_to_sync=False
    )
    agent._async_execution_manager = None
    agent._current_execution_context = None

    recorded = {}

    async def fake_execute(target, input_data, headers):
        recorded["target"] = target
        recorded["input_data"] = input_data
        recorded["headers"] = headers
        return {"result": {"ok": True}}

    agent.client = SimpleNamespace(execute=fake_execute)

    async def local_bot(self, a, b, execution_context=None, extra=None):
        return a + b

    agent.local_bot = MethodType(local_bot, agent)

    set_current_bot(agent)
    try:
        result = await agent.call("node.local_bot", 2, 3, extra=4)
    finally:
        clear_current_bot()

    assert result == {"ok": True}
    assert recorded["target"] == "node.local_bot"
    assert recorded["input_data"] == {"a": 2, "b": 3, "extra": 4}
    assert "X-Execution-ID" in recorded["headers"]


@pytest.mark.asyncio
async def test_call_remote_target_uses_generic_arg_names():
    agent = object.__new__(Agent)
    agent.node_id = "node"
    agent.agents_connected = True
    agent.dev_mode = False
    agent.async_config = SimpleNamespace(
        enable_async_execution=False, fallback_to_sync=False
    )
    agent._async_execution_manager = None
    agent._current_execution_context = None

    recorded = {}

    async def fake_execute(target, input_data, headers):
        recorded["target"] = target
        recorded["input_data"] = input_data
        return {"result": {"value": 10}}

    agent.client = SimpleNamespace(execute=fake_execute)

    set_current_bot(agent)
    try:
        result = await agent.call("other.remote_bot", 5, 6)
    finally:
        clear_current_bot()

    assert result == {"value": 10}
    assert recorded["target"] == "other.remote_bot"
    assert recorded["input_data"] == {"arg_0": 5, "arg_1": 6}


@pytest.mark.asyncio
async def test_call_raises_when_playground_disconnected():
    agent = object.__new__(Agent)
    agent.node_id = "node"
    agent.agents_connected = False
    agent.dev_mode = False
    agent.async_config = SimpleNamespace(
        enable_async_execution=False, fallback_to_sync=False
    )
    agent._async_execution_manager = None
    agent._current_execution_context = None
    agent.client = SimpleNamespace()

    set_current_bot(agent)
    try:
        with pytest.raises(Exception):
            await agent.call("other.bot", 1)
    finally:
        clear_current_bot()
