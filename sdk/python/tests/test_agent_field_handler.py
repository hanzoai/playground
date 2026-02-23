import threading

import pytest
import requests

from playground.bot_field_handler import PlaygroundHandler
from tests.helpers import StubAgent, DummyPlaygroundClient


@pytest.mark.asyncio
async def test_register_with_playground_server_sets_base_url(monkeypatch):
    agent = StubAgent(callback_url="agent.local", base_url=None)
    agent.client = DummyPlaygroundClient()
    agent.agents_connected = False

    monkeypatch.setattr(
        "playground.agent._resolve_callback_url",
        lambda url, port: f"http://resolved:{port}",
    )
    monkeypatch.setattr(
        "playground.agent._build_callback_candidates",
        lambda value, port, include_defaults=True: [f"http://resolved:{port}"],
    )
    monkeypatch.setattr("playground.agent._is_running_in_container", lambda: False)

    playground = PlaygroundHandler(agent)
    await playground.register_with_playground_server(port=8080)

    assert agent.base_url == "http://resolved:8080"
    assert agent.agents_connected is True
    assert agent.client.register_calls[0]["base_url"] == "http://resolved:8080"


@pytest.mark.asyncio
async def test_register_with_playground_server_handles_failure(monkeypatch):
    async def failing_register(*args, **kwargs):
        raise RuntimeError("boom")

    agent = StubAgent(callback_url=None, base_url="http://already", dev_mode=True)
    agent.client = DummyPlaygroundClient()
    monkeypatch.setattr(agent.client, "register_bot", failing_register)
    monkeypatch.setattr(
        "playground.agent._build_callback_candidates",
        lambda value, port, include_defaults=True: [],
    )
    monkeypatch.setattr("playground.agent._is_running_in_container", lambda: False)

    playground = PlaygroundHandler(agent)
    agent.agents_connected = True

    await playground.register_with_playground_server(port=9000)
    assert agent.agents_connected is False


@pytest.mark.asyncio
async def test_register_with_playground_updates_existing_port(monkeypatch):
    agent = StubAgent(callback_url=None, base_url="http://host:5000")
    agent.client = DummyPlaygroundClient()

    monkeypatch.setattr(
        "playground.agent._build_callback_candidates",
        lambda value, port, include_defaults=True: [],
    )
    monkeypatch.setattr("playground.agent._is_running_in_container", lambda: False)

    playground = PlaygroundHandler(agent)
    await playground.register_with_playground_server(port=6000)

    assert agent.base_url == "http://host:6000"
    assert agent.client.register_calls[0]["base_url"] == "http://host:6000"


@pytest.mark.asyncio
async def test_register_with_playground_preserves_container_urls(monkeypatch):
    agent = StubAgent(
        callback_url=None,
        base_url="http://service.railway.internal:5000",
        dev_mode=True,
    )
    agent.client = DummyPlaygroundClient()

    monkeypatch.setattr(
        "playground.agent._build_callback_candidates",
        lambda value, port, include_defaults=True: [],
    )
    monkeypatch.setattr("playground.agent._is_running_in_container", lambda: True)

    playground = PlaygroundHandler(agent)
    await playground.register_with_playground_server(port=7000)

    assert agent.base_url == "http://service.railway.internal:5000"


@pytest.mark.asyncio
async def test_register_with_playground_server_resolves_when_no_candidates(monkeypatch):
    agent = StubAgent(callback_url=None, base_url=None)
    agent.client = DummyPlaygroundClient()

    monkeypatch.setattr("playground.agent._build_callback_candidates", lambda *a, **k: [])
    monkeypatch.setattr(
        "playground.agent._resolve_callback_url",
        lambda url, port: f"http://resolved:{port}",
    )
    monkeypatch.setattr("playground.agent._is_running_in_container", lambda: False)

    playground = PlaygroundHandler(agent)
    await playground.register_with_playground_server(port=7100)

    assert agent.base_url == "http://resolved:7100"
    assert agent.agents_connected is True


@pytest.mark.asyncio
async def test_register_with_playground_server_reorders_candidates(monkeypatch):
    agent = StubAgent(callback_url=None, base_url="http://preferred:8000")
    agent.client = DummyPlaygroundClient()
    agent.callback_candidates = ["http://other:8000", "http://preferred:8000"]

    monkeypatch.setattr(
        "playground.agent._build_callback_candidates",
        lambda value, port, include_defaults=True: agent.callback_candidates,
    )
    monkeypatch.setattr("playground.agent._is_running_in_container", lambda: False)

    playground = PlaygroundHandler(agent)
    await playground.register_with_playground_server(port=8000)

    assert agent.callback_candidates[0] == "http://preferred:8000"


@pytest.mark.asyncio
async def test_register_with_playground_server_propagates_request_exception(
    monkeypatch,
):
    class DummyResponse:
        def __init__(self):
            self.status_code = 503
            self.text = "unavailable"

    exception = requests.exceptions.RequestException("fail")
    exception.response = DummyResponse()

    async def failing_register(*args, **kwargs):
        raise exception

    agent = StubAgent(callback_url=None, base_url="http://already", dev_mode=False)
    agent.client = DummyPlaygroundClient()
    monkeypatch.setattr(agent.client, "register_bot", failing_register)
    monkeypatch.setattr("playground.agent._build_callback_candidates", lambda *a, **k: [])
    monkeypatch.setattr("playground.agent._resolve_callback_url", lambda url, port: "http://already")
    monkeypatch.setattr("playground.agent._is_running_in_container", lambda: False)

    playground = PlaygroundHandler(agent)
    with pytest.raises(requests.exceptions.RequestException):
        await playground.register_with_playground_server(port=9001)
    assert agent.agents_connected is False


@pytest.mark.asyncio
async def test_register_with_playground_server_unsuccessful_response(monkeypatch):
    agent = StubAgent(callback_url=None, base_url="http://host:5000")
    agent.client = DummyPlaygroundClient()

    async def register_returns_false(*args, **kwargs):
        return False, None

    monkeypatch.setattr(agent.client, "register_bot", register_returns_false)
    monkeypatch.setattr("playground.agent._build_callback_candidates", lambda *a, **k: [])
    monkeypatch.setattr("playground.agent._resolve_callback_url", lambda url, port: "http://host:5000")
    monkeypatch.setattr("playground.agent._is_running_in_container", lambda: False)

    playground = PlaygroundHandler(agent)
    await playground.register_with_playground_server(port=5000)
    assert agent.agents_connected is False


@pytest.mark.asyncio
async def test_register_with_playground_applies_discovery_payload(monkeypatch):
    from tests.helpers import create_test_agent

    agent, playground_client = create_test_agent(monkeypatch)
    agent.callback_candidates = []

    async def fake_register(
        node_id,
        bots,
        skills,
        base_url,
        discovery=None,
        vc_metadata=None,
        **kwargs,
    ):
        return True, {
            "resolved_base_url": "https://public:9000",
            "callback_discovery": {
                "candidates": ["https://public:9000", "http://fallback:9000"],
            },
        }

    monkeypatch.setattr(playground_client, "register_bot", fake_register)
    monkeypatch.setattr(
        "playground.agent._build_callback_candidates",
        lambda value, port, include_defaults=True: [f"http://detected:{port}"],
    )
    monkeypatch.setattr("playground.agent._is_running_in_container", lambda: False)

    await agent.agents_handler.register_with_playground_server(port=9000)

    assert agent.base_url == "https://public:9000"
    assert agent.callback_candidates[0] == "https://public:9000"
    assert "http://fallback:9000" in agent.callback_candidates


def test_send_heartbeat(monkeypatch):
    agent = StubAgent()
    playground = PlaygroundHandler(agent)

    calls = {}

    def fake_post(url, headers=None, timeout=None):
        calls["url"] = url

        class Dummy:
            status_code = 200
            text = "ok"

        return Dummy()

    monkeypatch.setattr("requests.post", fake_post)
    playground.send_heartbeat()
    assert calls["url"].endswith(f"/api/v1/nodes/{agent.node_id}/heartbeat")


def test_send_heartbeat_warns_on_non_200(monkeypatch):
    agent = StubAgent()
    agent.agents_connected = True
    playground = PlaygroundHandler(agent)

    class Dummy:
        status_code = 500
        text = "error"

    monkeypatch.setattr("requests.post", lambda *a, **k: Dummy())
    playground.send_heartbeat()


@pytest.mark.asyncio
async def test_enhanced_heartbeat_returns_false_when_disconnected():
    agent = StubAgent()
    playground = PlaygroundHandler(agent)
    agent.agents_connected = False
    assert await playground.send_enhanced_heartbeat() is False


def test_start_and_stop_heartbeat(monkeypatch):
    agent = StubAgent()
    playground = PlaygroundHandler(agent)

    called = []

    def fake_worker(interval):
        called.append(interval)

    monkeypatch.setattr(playground, "heartbeat_worker", fake_worker)

    playground.start_heartbeat(interval=1)
    assert isinstance(agent._heartbeat_thread, threading.Thread)
    playground.stop_heartbeat()


@pytest.mark.asyncio
async def test_enhanced_heartbeat_and_shutdown(monkeypatch):
    agent = StubAgent()
    agent.client = DummyPlaygroundClient()
    agent.mcp_handler = type("MCP", (), {"_get_mcp_server_health": lambda self: ["mcp"]})()
    agent.dev_mode = True
    playground = PlaygroundHandler(agent)

    success = await playground.send_enhanced_heartbeat()
    assert success is True
    assert agent.client.heartbeat_calls

    success_shutdown = await playground.notify_shutdown()
    assert success_shutdown is True
    assert agent.client.shutdown_calls == [agent.node_id]


@pytest.mark.asyncio
async def test_enhanced_heartbeat_failure_returns_false(monkeypatch):
    agent = StubAgent()
    agent.client = DummyPlaygroundClient()
    playground = PlaygroundHandler(agent)

    async def boom(*args, **kwargs):
        raise RuntimeError("boom")

    monkeypatch.setattr(agent.client, "send_enhanced_heartbeat", boom)
    agent.agents_connected = True
    agent.dev_mode = True
    assert await playground.send_enhanced_heartbeat() is False


@pytest.mark.asyncio
async def test_notify_shutdown_failure_returns_false(monkeypatch):
    agent = StubAgent()
    agent.client = DummyPlaygroundClient()
    playground = PlaygroundHandler(agent)

    async def boom(*args, **kwargs):
        raise RuntimeError("boom")

    monkeypatch.setattr(agent.client, "notify_graceful_shutdown", boom)
    agent.agents_connected = True
    agent.dev_mode = True
    assert await playground.notify_shutdown() is False


def test_send_heartbeat_handles_error(monkeypatch):
    agent = StubAgent()
    agent.agents_connected = True
    playground = PlaygroundHandler(agent)

    def boom(*args, **kwargs):
        raise requests.RequestException("boom")

    monkeypatch.setattr("requests.post", boom)
    playground.send_heartbeat()


def test_start_heartbeat_skips_when_disconnected():
    agent = StubAgent()
    agent.agents_connected = False
    playground = PlaygroundHandler(agent)
    playground.start_heartbeat()
    assert agent._heartbeat_thread is None
