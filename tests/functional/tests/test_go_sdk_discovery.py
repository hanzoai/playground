import asyncio
import json
import os

import pytest

from utils import get_go_bot_binary, run_go_bot, unique_node_id
from tests.test_go_sdk_cli import _wait_for_bot_health, _wait_for_registration


async def _run_discovery_cli(binary: str, env: dict[str, str]) -> dict:
    proc = await asyncio.create_subprocess_exec(
        binary,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
        env=env,
    )
    stdout, stderr = await proc.communicate()
    if proc.returncode != 0:
        raise AssertionError(
            f"discovery cli failed rc={proc.returncode} stderr={stderr.decode()}"
        )
    return json.loads(stdout)


@pytest.mark.functional
@pytest.mark.asyncio
async def test_go_sdk_discovery_flow(async_http_client, control_plane_url):
    try:
        get_go_bot_binary("hello")
        discovery_binary = get_go_bot_binary("discovery")
    except FileNotFoundError:
        pytest.skip("Go discovery binaries not available in test image")

    node_id = unique_node_id("go-discovery-bot")
    env_server = {
        **os.environ,
        "PLAYGROUND_URL": control_plane_url,
        "HANZO_NODE_ID": node_id,
        "HANZO_LISTEN_ADDR": ":8101",
        "HANZO_PUBLIC_URL": "http://test-runner:8101",
    }
    # Preserve fallback for older Go SDK binaries
    env_server.setdefault("AGENTS_URL", control_plane_url)
    env_server.setdefault("AGENT_NODE_ID", node_id)
    env_server.setdefault("AGENT_LISTEN_ADDR", ":8101")
    env_server.setdefault("AGENT_PUBLIC_URL", "http://test-runner:8101")

    async with run_go_bot("hello", args=["serve"], env=env_server):
        await _wait_for_registration(async_http_client, node_id)
        await _wait_for_bot_health("http://127.0.0.1:8101/health")

        base_env = {
            k: v
            for k, v in os.environ.items()
            if k not in {"PLAYGROUND_URL", "AGENTS_URL", "HANZO_PUBLIC_URL", "AGENT_PUBLIC_URL"}
        }
        base_env.update(
            {
                "PLAYGROUND_URL": control_plane_url,
                "AGENTS_URL": control_plane_url,
                "HANZO_NODE_ID": unique_node_id("go-discovery-client"),
                "AGENT_NODE_ID": unique_node_id("go-discovery-client"),
            }
        )

        # JSON format filtered to the Go bot with schema inclusion.
        env_json = {
            **base_env,
            "DISCOVERY_AGENT": node_id,
            "DISCOVERY_INCLUDE_INPUT_SCHEMA": "true",
            "DISCOVERY_INCLUDE_OUTPUT_SCHEMA": "true",
            "DISCOVERY_FORMAT": "json",
        }
        json_payload = await _run_discovery_cli(discovery_binary, env_json)
        assert json_payload["format"] == "json"
        totals = json_payload.get("totals", {})
        assert totals.get("agents") == 1
        assert totals.get("bots") >= 1
        caps = json_payload.get("capabilities", [])
        assert caps and caps[0]["agent_id"] == node_id
        bot_ids = {item["id"] for item in caps[0]["bots"]}
        assert {"demo_echo", "say_hello", "add_emoji"}.issubset(bot_ids)

        # Compact format filtered by bot pattern.
        env_compact = {
            **base_env,
            "DISCOVERY_AGENT": node_id,
            "DISCOVERY_REASONER_PATTERN": "say_*",
            "DISCOVERY_FORMAT": "compact",
        }
        compact_payload = await _run_discovery_cli(discovery_binary, env_compact)
        assert compact_payload["format"] == "compact"
        compact_totals = compact_payload.get("totals", {})
        assert compact_totals.get("bots") == 1
        targets = {item["target"] for item in compact_payload.get("bots", [])}
        assert f"{node_id}:say_hello" in targets

        # XML format sanity check.
        env_xml = {**base_env, "DISCOVERY_FORMAT": "xml"}
        xml_payload = await _run_discovery_cli(discovery_binary, env_xml)
        assert xml_payload["format"] == "xml"
        xml_body = xml_payload.get("xml", "")
        assert node_id in xml_body
