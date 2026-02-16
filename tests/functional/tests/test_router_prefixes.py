import pytest

from bots.router_prefix_agent import BOT_SPEC, create_bot as create_router_bot
from utils import run_bot_server, unique_node_id


@pytest.mark.functional
@pytest.mark.asyncio
async def test_router_prefix_registration_and_execution(async_http_client):
    bot = create_router_bot(node_id=unique_node_id(BOT_SPEC.default_node_id))

    async with run_bot_server(bot):
        node_response = await async_http_client.get(f"/api/v1/nodes/{bot.node_id}")
        assert node_response.status_code == 200
        node_data = node_response.json()

        bot_ids = {r["id"] for r in node_data.get("bots", [])}
        assert {"tools_echo", "tools_status"} <= bot_ids

        echo_response = await async_http_client.post(
            f"/api/v1/execute/{bot.node_id}.tools_echo",
            json={"input": {"message": "router check"}},
            timeout=20.0,
        )
        assert echo_response.status_code == 200
        echo_result = echo_response.json()["result"]
        assert echo_result["message"] == "router check"
        assert echo_result["length"] == len("router check")

        status_response = await async_http_client.post(
            f"/api/v1/bots/{bot.node_id}.tools_status",
            json={"input": {}},
            timeout=20.0,
        )
        assert status_response.status_code == 200
        status_result = status_response.json()["result"]
        assert status_result["node_id"] == bot.node_id
        assert status_result["router_prefix"] == "tools"
        assert "tools_echo" in status_result["bots"]
