import pytest

from bots.call_chain_agents import (
    ORCHESTRATOR_SPEC,
    WORKER_SPEC,
    create_orchestrator_bot,
    create_worker_bot,
)
from utils import run_bot_server, unique_node_id


@pytest.mark.functional
@pytest.mark.asyncio
async def test_cross_bot_app_call_workflow(async_http_client):
    worker = create_worker_bot(node_id=unique_node_id(WORKER_SPEC.default_node_id))
    orchestrator = create_orchestrator_bot(
        node_id=unique_node_id(ORCHESTRATOR_SPEC.default_node_id),
        target_node_id=worker.node_id,
    )

    async with run_bot_server(worker), run_bot_server(orchestrator):
        payload = {"input": {"text": "Playground rocks"}}

        response = await async_http_client.post(
            f"/api/v1/bots/{orchestrator.node_id}.delegate_pipeline",
            json=payload,
            timeout=30.0,
        )

        assert response.status_code == 200, response.text
        body = response.json()
        result = body["result"]

        assert result["original"] == "Playground rocks"
        delegated = result["delegated"]
        assert delegated["upper"] == "AGENTS ROCKS"
        assert delegated["length"] == len("Playground rocks")
        assert result["tokens"] == 2
