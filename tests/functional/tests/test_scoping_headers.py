"""Functional tests validating scoped memory helpers override inbound headers."""

from __future__ import annotations

import uuid

import pytest

from bots.scoping_agent import BOT_SPEC, create_bot as create_scoping_bot
from utils import run_bot_server, unique_node_id


async def _invoke_scoped_bot(
    async_http_client,
    bot,
    payload: dict,
    headers: dict | None = None,
):
    response = await async_http_client.post(
        f"/api/v1/bots/{bot.node_id}.scoped_memory",
        json={"input": payload},
        headers=headers or {},
        timeout=30.0,
    )
    assert response.status_code == 200, response.text
    return response.json()["result"]


def _random_id(prefix: str) -> str:
    return f"{prefix}-{uuid.uuid4().hex[:8]}"


@pytest.mark.functional
@pytest.mark.asyncio
async def test_session_scope_helper_overrides_headers(async_http_client):
    bot = create_scoping_bot(node_id=unique_node_id(BOT_SPEC.default_node_id))

    async with run_bot_server(bot):
        key = _random_id("session-key")
        scope_id = _random_id("session")
        write_headers = {
            "X-Session-ID": _random_id("incoming-session"),
            "X-Workflow-ID": _random_id("workflow"),
        }
        write_result = await _invoke_scoped_bot(
            async_http_client,
            bot,
            {
                "scope": "session",
                "scope_id": scope_id,
                "key": key,
                "value": "session-value",
                "action": "write",
            },
            headers=write_headers,
        )
        assert write_result["scoped_value"] == "session-value"
        assert write_result["execution_context"]["session_id"] == write_headers["X-Session-ID"]

        read_headers = {
            "X-Session-ID": _random_id("incoming-session"),
            "X-Workflow-ID": _random_id("workflow"),
        }
        read_result = await _invoke_scoped_bot(
            async_http_client,
            bot,
            {
                "scope": "session",
                "scope_id": scope_id,
                "key": key,
                "action": "read",
            },
            headers=read_headers,
        )
        assert read_result["scoped_value"] == "session-value"
        assert read_result["exists"] is True
        assert key in (read_result["keys"] or [])
        assert read_result["execution_context"]["session_id"] == read_headers["X-Session-ID"]


@pytest.mark.functional
@pytest.mark.asyncio
async def test_actor_scope_helper_overrides_headers(async_http_client):
    bot = create_scoping_bot(node_id=unique_node_id(BOT_SPEC.default_node_id))

    async with run_bot_server(bot):
        key = _random_id("actor-key")
        scope_id = _random_id("actor")
        write_headers = {
            "X-Actor-ID": _random_id("incoming-actor"),
            "X-Workflow-ID": _random_id("workflow"),
        }
        await _invoke_scoped_bot(
            async_http_client,
            bot,
            {
                "scope": "actor",
                "scope_id": scope_id,
                "key": key,
                "value": "actor-value",
                "action": "write",
            },
            headers=write_headers,
        )

        read_headers = {
            "X-Actor-ID": _random_id("incoming-actor"),
            "X-Workflow-ID": _random_id("workflow"),
        }
        read_result = await _invoke_scoped_bot(
            async_http_client,
            bot,
            {
                "scope": "actor",
                "scope_id": scope_id,
                "key": key,
                "action": "read",
            },
            headers=read_headers,
        )
        assert read_result["scoped_value"] == "actor-value"
        assert key in (read_result["keys"] or [])
        assert read_result["execution_context"]["actor_id"] == read_headers["X-Actor-ID"]


@pytest.mark.functional
@pytest.mark.asyncio
async def test_workflow_scope_helper_overrides_headers(async_http_client):
    bot = create_scoping_bot(node_id=unique_node_id(BOT_SPEC.default_node_id))

    async with run_bot_server(bot):
        key = _random_id("workflow-key")
        scope_id = _random_id("manual-wf")
        write_headers = {"X-Workflow-ID": _random_id("workflow")}
        await _invoke_scoped_bot(
            async_http_client,
            bot,
            {
                "scope": "workflow",
                "scope_id": scope_id,
                "key": key,
                "value": "workflow-value",
                "action": "write",
            },
            headers=write_headers,
        )

        read_headers = {"X-Workflow-ID": _random_id("workflow")}
        read_result = await _invoke_scoped_bot(
            async_http_client,
            bot,
            {
                "scope": "workflow",
                "scope_id": scope_id,
                "key": key,
                "action": "read",
            },
            headers=read_headers,
        )
        assert read_result["scoped_value"] == "workflow-value"
        assert key in (read_result["keys"] or [])
        assert read_result["execution_context"]["workflow_id"] == read_headers["X-Workflow-ID"]
