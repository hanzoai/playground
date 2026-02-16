import pytest

from bots.memory_agent import BOT_SPEC, create_bot as create_memory_bot
from utils import run_bot_server, unique_node_id


async def _invoke_remember_user(async_http_client, endpoint: str, payload: dict):
    response = await async_http_client.post(
        endpoint,
        json={"input": payload},
        timeout=30.0,
    )
    assert response.status_code == 200, response.text
    return response.json()["result"]


@pytest.mark.functional
@pytest.mark.asyncio
async def test_app_memory_via_bot_endpoint(async_http_client):
    bot = create_memory_bot(node_id=unique_node_id(BOT_SPEC.default_node_id))

    async with run_bot_server(bot):
        endpoint = f"/api/v1/bots/{bot.node_id}.remember_user"
        user_id = unique_node_id("memory-user-bot")
        first = await _invoke_remember_user(
            async_http_client,
            endpoint,
            {"user_id": user_id, "message": "Hello memory"},
        )
        assert first["messages_seen"] == 1
        assert first["recent_history"] == ["Hello memory"]
        assert first["global_key_exists"] is True

        second = await _invoke_remember_user(
            async_http_client,
            endpoint,
            {"user_id": user_id, "message": "Second visit"},
        )
        assert second["messages_seen"] == 2
        assert second["recent_history"][-2:] == ["Hello memory", "Second visit"]


@pytest.mark.functional
@pytest.mark.asyncio
async def test_app_memory_via_execute_endpoint(async_http_client):
    bot = create_memory_bot(node_id=unique_node_id(BOT_SPEC.default_node_id))

    async with run_bot_server(bot):
        endpoint = f"/api/v1/execute/{bot.node_id}.remember_user"
        user_id = unique_node_id("memory-user-execute")

        first = await _invoke_remember_user(
            async_http_client,
            endpoint,
            {"user_id": user_id, "message": "Execute API hello"},
        )
        assert first["messages_seen"] == 1
        assert len(first["recent_history"]) == 1

        second = await _invoke_remember_user(
            async_http_client,
            endpoint,
            {"user_id": user_id, "message": "Execute API follow-up"},
        )
        assert second["messages_seen"] == 2
        assert second["recent_history"][-1] == "Execute API follow-up"
