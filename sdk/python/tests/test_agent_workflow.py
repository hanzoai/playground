import pytest

from playground.bot_workflow import BotWorkflow
from playground.execution_context import ExecutionContext
from playground.bot_registry import set_current_bot, clear_current_bot
from tests.helpers import StubAgent


@pytest.mark.asyncio
async def test_execute_with_tracking_root_context(monkeypatch):
    agent = StubAgent()
    workflow = BotWorkflow(agent)

    captured = {}

    async def fake_start(
        execution_id, context, bot_name, input_data, parent_execution_id=None
    ):
        captured["start"] = {
            "execution_id": execution_id,
            "workflow_id": context.workflow_id,
            "bot": bot_name,
            "parent": parent_execution_id,
            "input": input_data,
        }

    async def fake_complete(
        execution_id, workflow_id, result, duration_ms, context, **kwargs
    ):
        captured["complete"] = {
            "execution_id": execution_id,
            "workflow_id": workflow_id,
            "result": result,
        }

    monkeypatch.setattr(workflow, "notify_call_start", fake_start)
    monkeypatch.setattr(workflow, "notify_call_complete", fake_complete)

    async def sample(value: int, execution_context: ExecutionContext = None):
        assert isinstance(execution_context, ExecutionContext)
        return value + 1

    set_current_bot(agent)
    try:
        result = await workflow.execute_with_tracking(sample, (3,), {})
    finally:
        clear_current_bot()

    assert result == 4
    assert captured["start"]["parent"] is None
    assert captured["start"]["bot"] == "sample"
    assert captured["complete"]["result"] == 4


@pytest.mark.asyncio
async def test_execute_with_tracking_child_context(monkeypatch):
    agent = StubAgent()
    workflow = BotWorkflow(agent)

    set_current_bot(agent)
    parent_context = ExecutionContext.create_new(agent.node_id, "root")
    agent._current_execution_context = parent_context

    events = {}

    async def fake_start(
        execution_id, context, bot_name, input_data, parent_execution_id=None
    ):
        events.setdefault("start", []).append(
            (execution_id, context, parent_execution_id)
        )

    async def fake_error(*args, **kwargs):
        events["error"] = True

    monkeypatch.setattr(workflow, "notify_call_start", fake_start)
    monkeypatch.setattr(workflow, "notify_call_error", fake_error)

    async def failing(execution_context: ExecutionContext = None):
        raise RuntimeError("boom")

    with pytest.raises(RuntimeError):
        try:
            await workflow.execute_with_tracking(failing, tuple(), {})
        finally:
            clear_current_bot()

    assert "start" in events
    exec_id, ctx, parent = events["start"][0]
    assert parent == parent_context.execution_id
    assert ctx.parent_execution_id == parent_context.execution_id
    assert events.get("error") is True


@pytest.mark.asyncio
async def test_notify_call_start_payload_includes_hierarchy(monkeypatch):
    agent = StubAgent()
    workflow = BotWorkflow(agent)

    set_current_bot(agent)
    try:
        parent_context = ExecutionContext.create_new(agent.node_id, "root")
        child_context = parent_context.create_child_context()
        child_context.bot_name = "child_bot"

        payloads = []

        async def capture(payload):
            payloads.append(payload)

        monkeypatch.setattr(workflow, "fire_and_forget_update", capture)

        await workflow.notify_call_start(
            child_context.execution_id,
            child_context,
            "child_bot",
            {"input": "value"},
            parent_execution_id=parent_context.execution_id,
        )

    finally:
        clear_current_bot()

    assert len(payloads) == 1
    payload = payloads[0]
    assert payload["execution_id"] == child_context.execution_id
    assert payload["workflow_id"] == parent_context.workflow_id
    assert payload["parent_execution_id"] == parent_context.execution_id
    assert payload["parent_workflow_id"] == parent_context.workflow_id
    assert payload["agent_node_id"] == agent.node_id
    assert payload["bot_id"] == "child_bot"
    assert payload["status"] == "running"
    assert payload["type"] == "child_bot"
    assert payload["input_data"] == {"input": "value"}


@pytest.mark.asyncio
async def test_execute_with_tracking_emits_workflow_updates(monkeypatch):
    agent = StubAgent()
    workflow = BotWorkflow(agent)

    set_current_bot(agent)
    try:
        parent_context = ExecutionContext.create_new(agent.node_id, "root")
        agent._current_execution_context = parent_context

        events = {}

        async def capture_start(
            execution_id, context, bot_name, input_data, parent_execution_id=None
        ):
            events.setdefault("start", []).append(
                {
                    "execution_id": execution_id,
                    "context": context,
                    "bot_name": bot_name,
                    "input_data": input_data,
                    "parent_execution_id": parent_execution_id,
                }
            )

        async def capture_complete(
            execution_id,
            workflow_id,
            result,
            duration_ms,
            context,
            input_data=None,
            parent_execution_id=None,
        ):
            events.setdefault("complete", []).append(
                {
                    "execution_id": execution_id,
                    "workflow_id": workflow_id,
                    "context": context,
                    "result": result,
                    "input_data": input_data,
                    "parent_execution_id": parent_execution_id,
                }
            )

        monkeypatch.setattr(workflow, "notify_call_start", capture_start)
        monkeypatch.setattr(workflow, "notify_call_complete", capture_complete)

        async def child_bot(
            value: int, execution_context: ExecutionContext = None
        ):
            assert execution_context is not None
            return {"doubled": value * 2}

        result = await workflow.execute_with_tracking(child_bot, (7,), {})
    finally:
        clear_current_bot()

    assert result == {"doubled": 14}
    assert "start" in events and "complete" in events

    start_event = events["start"][0]
    complete_event = events["complete"][0]

    assert start_event["parent_execution_id"] == parent_context.execution_id
    assert start_event["context"].parent_workflow_id == parent_context.workflow_id
    assert start_event["context"].parent_execution_id == parent_context.execution_id
    assert start_event["bot_name"] == "child_bot"
    assert start_event["input_data"]["value"] == 7
    assert isinstance(
        start_event["input_data"].get("execution_context"), ExecutionContext
    )

    assert complete_event["parent_execution_id"] == parent_context.execution_id
    assert complete_event["context"].parent_workflow_id == parent_context.workflow_id
    assert complete_event["context"].parent_execution_id == parent_context.execution_id
    assert complete_event["result"] == {"doubled": 14}
    assert complete_event["input_data"]["value"] == 7
    assert isinstance(
        complete_event["input_data"].get("execution_context"), ExecutionContext
    )


@pytest.mark.asyncio
async def test_execute_with_tracking_error_emits_failure(monkeypatch):
    agent = StubAgent()
    workflow = BotWorkflow(agent)

    set_current_bot(agent)
    try:
        parent_context = ExecutionContext.create_new(agent.node_id, "root")
        agent._current_execution_context = parent_context

        events = {}

        async def capture_start(
            execution_id, context, bot_name, input_data, parent_execution_id=None
        ):
            events.setdefault("start", []).append(
                {
                    "execution_id": execution_id,
                    "context": context,
                    "bot_name": bot_name,
                    "input_data": input_data,
                    "parent_execution_id": parent_execution_id,
                }
            )

        async def capture_error(
            execution_id,
            workflow_id,
            error,
            duration_ms,
            context,
            input_data=None,
            parent_execution_id=None,
        ):
            events.setdefault("error", []).append(
                {
                    "execution_id": execution_id,
                    "workflow_id": workflow_id,
                    "context": context,
                    "error": error,
                    "input_data": input_data,
                    "parent_execution_id": parent_execution_id,
                }
            )

        monkeypatch.setattr(workflow, "notify_call_start", capture_start)
        monkeypatch.setattr(workflow, "notify_call_error", capture_error)

        async def failing_bot(
            value: int, execution_context: ExecutionContext = None
        ):
            raise RuntimeError("expected failure")

        with pytest.raises(RuntimeError):
            await workflow.execute_with_tracking(failing_bot, (5,), {})
    finally:
        clear_current_bot()

    assert "start" in events and "error" in events

    start_event = events["start"][0]
    error_event = events["error"][0]

    assert start_event["parent_execution_id"] == parent_context.execution_id
    assert start_event["context"].parent_workflow_id == parent_context.workflow_id
    assert start_event["input_data"]["value"] == 5
    assert isinstance(
        start_event["input_data"].get("execution_context"), ExecutionContext
    )

    assert error_event["parent_execution_id"] == parent_context.execution_id
    assert error_event["context"].parent_workflow_id == parent_context.workflow_id
    assert error_event["input_data"]["value"] == 5
    assert isinstance(
        error_event["input_data"].get("execution_context"), ExecutionContext
    )
    assert "expected failure" in error_event["error"]


@pytest.mark.asyncio
async def test_nested_bots_emit_child_completion_before_parent(monkeypatch):
    agent = StubAgent()
    workflow = BotWorkflow(agent)
    agent.workflow_handler = workflow

    set_current_bot(agent)
    agent._current_execution_context = None

    payloads = []

    async def capture(payload):
        payloads.append(payload)

    monkeypatch.setattr(workflow, "fire_and_forget_update", capture)

    async def child_bot(execution_context: ExecutionContext = None):
        assert execution_context is not None
        return "child-result"

    async def parent_bot(execution_context: ExecutionContext = None):
        assert execution_context is not None
        # Invoke child bot within the tracked context.
        return await workflow.execute_with_tracking(child_bot, tuple(), {})

    try:
        result = await workflow.execute_with_tracking(parent_bot, tuple(), {})
    finally:
        clear_current_bot()
        agent._current_execution_context = None

    assert result == "child-result"
    # We expect exactly four lifecycle events: parent start, child start, child complete, parent complete.
    assert len(payloads) == 4

    timeline = [(payload["bot_id"], payload["status"]) for payload in payloads]

    assert timeline[0] == ("parent_bot", "running")
    assert timeline[1] == ("child_bot", "running")

    child_complete_index = next(
        index
        for index, entry in enumerate(timeline)
        if entry == ("child_bot", "succeeded")
    )

    parent_complete_index = next(
        index
        for index, entry in enumerate(timeline)
        if entry[1] == "succeeded" and entry[0].endswith("parent_bot")
    )

    assert (
        child_complete_index < parent_complete_index
    ), "Parent bot completion emitted before child finished"
