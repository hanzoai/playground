"""
Tests for parent-child workflow tracking in bot chains.

These tests verify that when bots call other bots directly (as normal
async functions), the parent-child relationships are correctly established in
the execution context and propagated to workflow events.

This is critical for building accurate workflow DAG visualizations.
"""

import asyncio
from typing import Dict, List, Any

import pytest

from playground.bot_workflow import BotWorkflow
from playground.execution_context import ExecutionContext
from playground.bot_registry import set_current_bot, clear_current_bot
from playground.decorators import bot
from tests.helpers import StubAgent


class TestParentChildWorkflowTracking:
    """Tests for parent-child relationship in workflow execution."""

    @pytest.mark.asyncio
    async def test_root_execution_has_no_parent(self, monkeypatch):
        """Root bot execution should have parent_execution_id=None."""
        agent = StubAgent()
        workflow = BotWorkflow(agent)

        captured_events: List[Dict[str, Any]] = []

        async def capture_update(payload: Dict[str, Any]):
            captured_events.append(payload)

        monkeypatch.setattr(workflow, "fire_and_forget_update", capture_update)

        async def root_bot(value: int, execution_context: ExecutionContext = None):
            return {"value": value}

        set_current_bot(agent)
        agent._current_execution_context = None  # No parent context

        try:
            result = await workflow.execute_with_tracking(root_bot, (42,), {})
        finally:
            clear_current_bot()

        assert result == {"value": 42}
        assert len(captured_events) == 2  # start + complete

        start_event = next(e for e in captured_events if e["status"] == "running")
        assert start_event["parent_execution_id"] is None
        assert start_event["bot_id"] == "root_bot"

    @pytest.mark.asyncio
    async def test_direct_call_creates_parent_child_relationship(self, monkeypatch):
        """When bot A calls bot B directly, B should have A as parent."""
        agent = StubAgent()
        workflow = BotWorkflow(agent)
        agent.workflow_handler = workflow

        captured_events: List[Dict[str, Any]] = []

        async def capture_update(payload: Dict[str, Any]):
            captured_events.append(payload)

        monkeypatch.setattr(workflow, "fire_and_forget_update", capture_update)

        async def child_bot(x: int, execution_context: ExecutionContext = None):
            return x * 2

        async def parent_bot(x: int, execution_context: ExecutionContext = None):
            # Direct call to child bot
            child_result = await workflow.execute_with_tracking(child_bot, (x,), {})
            return {"parent": x, "child_result": child_result}

        set_current_bot(agent)
        agent._current_execution_context = None

        try:
            result = await workflow.execute_with_tracking(parent_bot, (5,), {})
        finally:
            clear_current_bot()

        assert result == {"parent": 5, "child_result": 10}

        # Should have 4 events: parent_start, child_start, child_complete, parent_complete
        assert len(captured_events) == 4

        # Extract execution IDs
        parent_start = next(
            e
            for e in captured_events
            if e["bot_id"] == "parent_bot" and e["status"] == "running"
        )
        child_start = next(
            e
            for e in captured_events
            if e["bot_id"] == "child_bot" and e["status"] == "running"
        )

        # Parent should have no parent
        assert parent_start["parent_execution_id"] is None

        # Child should reference parent's execution_id
        assert child_start["parent_execution_id"] == parent_start["execution_id"]

    @pytest.mark.asyncio
    async def test_parallel_calls_all_have_same_parent(self, monkeypatch):
        """asyncio.gather(B, C) from A should show B and C both as children of A."""
        agent = StubAgent()
        workflow = BotWorkflow(agent)
        agent.workflow_handler = workflow

        captured_events: List[Dict[str, Any]] = []

        async def capture_update(payload: Dict[str, Any]):
            captured_events.append(payload)

        monkeypatch.setattr(workflow, "fire_and_forget_update", capture_update)

        async def child_b(execution_context: ExecutionContext = None):
            return "B"

        async def child_c(execution_context: ExecutionContext = None):
            return "C"

        async def parent_bot(execution_context: ExecutionContext = None):
            # Parallel calls
            results = await asyncio.gather(
                workflow.execute_with_tracking(child_b, tuple(), {}),
                workflow.execute_with_tracking(child_c, tuple(), {}),
            )
            return {"results": results}

        set_current_bot(agent)
        agent._current_execution_context = None

        try:
            result = await workflow.execute_with_tracking(parent_bot, tuple(), {})
        finally:
            clear_current_bot()

        assert result == {"results": ["B", "C"]}

        # Should have 6 events: parent_start, child_b_start, child_c_start,
        # child_b_complete, child_c_complete, parent_complete
        assert len(captured_events) == 6

        parent_start = next(
            e
            for e in captured_events
            if e["bot_id"] == "parent_bot" and e["status"] == "running"
        )
        child_b_start = next(
            e
            for e in captured_events
            if e["bot_id"] == "child_b" and e["status"] == "running"
        )
        child_c_start = next(
            e
            for e in captured_events
            if e["bot_id"] == "child_c" and e["status"] == "running"
        )

        # Both children should reference the same parent
        assert child_b_start["parent_execution_id"] == parent_start["execution_id"]
        assert child_c_start["parent_execution_id"] == parent_start["execution_id"]

    @pytest.mark.asyncio
    async def test_three_level_nesting_maintains_hierarchy(self, monkeypatch):
        """A→B→C chain should maintain correct parent-child links at each level."""
        agent = StubAgent()
        workflow = BotWorkflow(agent)
        agent.workflow_handler = workflow

        captured_events: List[Dict[str, Any]] = []

        async def capture_update(payload: Dict[str, Any]):
            captured_events.append(payload)

        monkeypatch.setattr(workflow, "fire_and_forget_update", capture_update)

        async def grandchild(execution_context: ExecutionContext = None):
            return "grandchild"

        async def child(execution_context: ExecutionContext = None):
            result = await workflow.execute_with_tracking(grandchild, tuple(), {})
            return f"child->{result}"

        async def parent(execution_context: ExecutionContext = None):
            result = await workflow.execute_with_tracking(child, tuple(), {})
            return f"parent->{result}"

        set_current_bot(agent)
        agent._current_execution_context = None

        try:
            result = await workflow.execute_with_tracking(parent, tuple(), {})
        finally:
            clear_current_bot()

        assert result == "parent->child->grandchild"

        # Should have 6 events: 3 starts + 3 completes
        assert len(captured_events) == 6

        # Get start events for each level
        parent_start = next(
            e
            for e in captured_events
            if e["bot_id"] == "parent" and e["status"] == "running"
        )
        child_start = next(
            e
            for e in captured_events
            if e["bot_id"] == "child" and e["status"] == "running"
        )
        grandchild_start = next(
            e
            for e in captured_events
            if e["bot_id"] == "grandchild" and e["status"] == "running"
        )

        # Verify hierarchy
        assert parent_start["parent_execution_id"] is None
        assert child_start["parent_execution_id"] == parent_start["execution_id"]
        assert grandchild_start["parent_execution_id"] == child_start["execution_id"]

    @pytest.mark.asyncio
    async def test_workflow_id_preserved_across_chain(self, monkeypatch):
        """All executions in a chain should share the same workflow_id."""
        agent = StubAgent()
        workflow = BotWorkflow(agent)
        agent.workflow_handler = workflow

        captured_events: List[Dict[str, Any]] = []

        async def capture_update(payload: Dict[str, Any]):
            captured_events.append(payload)

        monkeypatch.setattr(workflow, "fire_and_forget_update", capture_update)

        async def child(execution_context: ExecutionContext = None):
            return "child"

        async def parent(execution_context: ExecutionContext = None):
            return await workflow.execute_with_tracking(child, tuple(), {})

        set_current_bot(agent)
        agent._current_execution_context = None

        try:
            await workflow.execute_with_tracking(parent, tuple(), {})
        finally:
            clear_current_bot()

        # All events should have the same workflow_id
        workflow_ids = {e["workflow_id"] for e in captured_events}
        assert len(workflow_ids) == 1

    @pytest.mark.asyncio
    async def test_event_payload_contains_all_required_fields(self, monkeypatch):
        """Verify event payloads contain all fields needed for DAG construction."""
        agent = StubAgent()
        workflow = BotWorkflow(agent)
        agent.workflow_handler = workflow

        captured_events: List[Dict[str, Any]] = []

        async def capture_update(payload: Dict[str, Any]):
            captured_events.append(payload)

        monkeypatch.setattr(workflow, "fire_and_forget_update", capture_update)

        async def sample(execution_context: ExecutionContext = None):
            return "ok"

        set_current_bot(agent)
        agent._current_execution_context = None

        try:
            await workflow.execute_with_tracking(sample, tuple(), {})
        finally:
            clear_current_bot()

        # Check required fields in start event
        start_event = next(e for e in captured_events if e["status"] == "running")

        required_fields = [
            "execution_id",
            "workflow_id",
            "run_id",
            "bot_id",
            "agent_node_id",
            "status",
            "type",
            "parent_execution_id",
            "parent_workflow_id",
        ]

        for field in required_fields:
            assert field in start_event, f"Missing required field: {field}"

        # Verify non-None for required values
        assert start_event["execution_id"] is not None
        assert start_event["workflow_id"] is not None
        assert start_event["bot_id"] == "sample"
        assert start_event["agent_node_id"] == agent.node_id


class TestDecoratorParentChildTracking:
    """Tests for parent-child tracking via @bot decorator."""

    @pytest.mark.asyncio
    async def test_decorated_bot_propagates_parent_context(self, monkeypatch):
        """@bot decorated functions should propagate parent context."""
        captured_payloads: List[Dict[str, Any]] = []

        async def capture_start(agent, ctx, payload):
            captured_payloads.append(
                {
                    "type": "start",
                    "execution_id": ctx.execution_id,
                    "parent_execution_id": ctx.parent_execution_id,
                    "bot_name": ctx.bot_name,
                }
            )

        async def capture_complete(agent, ctx, result, duration_ms, payload):
            captured_payloads.append(
                {
                    "type": "complete",
                    "execution_id": ctx.execution_id,
                    "parent_execution_id": ctx.parent_execution_id,
                    "bot_name": ctx.bot_name,
                }
            )

        monkeypatch.setattr("playground.decorators._send_workflow_start", capture_start)
        monkeypatch.setattr(
            "playground.decorators._send_workflow_completion", capture_complete
        )

        agent = StubAgent()
        set_current_bot(agent)

        tasks = []

        def capture_task(coro):
            task = asyncio.ensure_future(coro)
            tasks.append(task)
            return task

        monkeypatch.setattr(asyncio, "create_task", capture_task)

        @bot
        async def child_bot(x: int, execution_context: ExecutionContext = None):
            return x * 2

        @bot
        async def parent_bot(x: int, execution_context: ExecutionContext = None):
            # Child call happens within parent's context
            return await child_bot(x)

        try:
            result = await parent_bot(5)
        finally:
            clear_current_bot()
            if tasks:
                await asyncio.gather(*tasks, return_exceptions=True)

        assert result == 10

        # Find parent and child start events
        starts = [p for p in captured_payloads if p["type"] == "start"]
        assert len(starts) == 2

        parent_start = next(p for p in starts if p["bot_name"] == "parent_bot")
        child_start = next(p for p in starts if p["bot_name"] == "child_bot")

        # Parent should have no parent
        assert parent_start["parent_execution_id"] is None

        # Child should reference parent
        assert child_start["parent_execution_id"] == parent_start["execution_id"]


class TestErrorHandlingWithParentChild:
    """Tests for parent-child tracking during error scenarios."""

    @pytest.mark.asyncio
    async def test_child_error_preserves_parent_link(self, monkeypatch):
        """When child fails, error event should still have correct parent_execution_id."""
        agent = StubAgent()
        workflow = BotWorkflow(agent)
        agent.workflow_handler = workflow

        captured_events: List[Dict[str, Any]] = []

        async def capture_update(payload: Dict[str, Any]):
            captured_events.append(payload)

        monkeypatch.setattr(workflow, "fire_and_forget_update", capture_update)

        async def failing_child(execution_context: ExecutionContext = None):
            raise ValueError("child failed")

        async def parent_bot(execution_context: ExecutionContext = None):
            return await workflow.execute_with_tracking(failing_child, tuple(), {})

        set_current_bot(agent)
        agent._current_execution_context = None

        with pytest.raises(ValueError):
            try:
                await workflow.execute_with_tracking(parent_bot, tuple(), {})
            finally:
                clear_current_bot()

        # Find parent start and child error events
        parent_start = next(
            e
            for e in captured_events
            if e["bot_id"] == "parent_bot" and e["status"] == "running"
        )
        child_error = next(
            e
            for e in captured_events
            if e["bot_id"] == "failing_child" and e["status"] == "failed"
        )

        # Child error should reference parent
        assert child_error["parent_execution_id"] == parent_start["execution_id"]
        assert "child failed" in child_error.get("error", "")

    @pytest.mark.asyncio
    async def test_partial_parallel_failure_preserves_relationships(self, monkeypatch):
        """When one parallel child fails, others should still have correct parent links."""
        agent = StubAgent()
        workflow = BotWorkflow(agent)
        agent.workflow_handler = workflow

        captured_events: List[Dict[str, Any]] = []

        async def capture_update(payload: Dict[str, Any]):
            captured_events.append(payload)

        monkeypatch.setattr(workflow, "fire_and_forget_update", capture_update)

        async def success_child(execution_context: ExecutionContext = None):
            return "ok"

        async def failing_child(execution_context: ExecutionContext = None):
            await asyncio.sleep(0.01)  # Let success_child complete first
            raise ValueError("failed")

        async def parent_bot(execution_context: ExecutionContext = None):
            results = await asyncio.gather(
                workflow.execute_with_tracking(success_child, tuple(), {}),
                workflow.execute_with_tracking(failing_child, tuple(), {}),
                return_exceptions=True,
            )
            return results

        set_current_bot(agent)
        agent._current_execution_context = None

        try:
            await workflow.execute_with_tracking(parent_bot, tuple(), {})
        finally:
            clear_current_bot()

        # Get parent start
        parent_start = next(
            e
            for e in captured_events
            if e["bot_id"] == "parent_bot" and e["status"] == "running"
        )

        # Both children should reference the same parent
        success_start = next(
            e
            for e in captured_events
            if e["bot_id"] == "success_child" and e["status"] == "running"
        )
        failing_start = next(
            e
            for e in captured_events
            if e["bot_id"] == "failing_child" and e["status"] == "running"
        )

        assert success_start["parent_execution_id"] == parent_start["execution_id"]
        assert failing_start["parent_execution_id"] == parent_start["execution_id"]
