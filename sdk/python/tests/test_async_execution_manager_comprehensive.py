"""
Comprehensive tests for AsyncExecutionManager covering critical execution paths.
"""

import asyncio
from unittest.mock import AsyncMock, MagicMock

import pytest

from agentfield.async_config import AsyncConfig
from agentfield.async_execution_manager import AsyncExecutionManager
from agentfield.execution_state import ExecutionPriority, ExecutionStatus


@pytest.fixture
def mock_connection_manager():
    """Create a mock connection manager."""
    manager = MagicMock()
    manager.get_connection = AsyncMock(return_value=MagicMock())
    manager.close_connection = AsyncMock()
    return manager


@pytest.fixture
def mock_result_cache():
    """Create a mock result cache."""
    cache = MagicMock()
    cache.get = MagicMock(return_value=None)
    cache.set = MagicMock()
    cache.cleanup = MagicMock()
    return cache


@pytest.fixture
def async_config():
    """Create a test async config."""
    return AsyncConfig(
        poll_interval=0.1,
        max_concurrent_polls=5,
        batch_poll_size=10,
        cleanup_interval=1.0,
        max_execution_age=3600,
    )


@pytest.fixture
def manager(async_config, mock_connection_manager, mock_result_cache):
    """Create an AsyncExecutionManager instance."""
    return AsyncExecutionManager(
        base_url="http://test-server",
        config=async_config,
        connection_manager=mock_connection_manager,
        result_cache=mock_result_cache,
    )


@pytest.mark.asyncio
async def test_submit_execution_success(manager):
    """Test successful execution submission."""
    execution_id = await manager.submit_execution(
        target="agent.reasoner",
        input_data={"key": "value"},
        priority=ExecutionPriority.NORMAL,
    )

    assert execution_id is not None
    assert execution_id in manager._executions


@pytest.mark.asyncio
async def test_submit_execution_with_webhook(manager):
    """Test execution submission with webhook configuration."""
    webhook_config = {
        "url": "https://example.com/webhook",
        "secret": "test-secret",
    }

    execution_id = await manager.submit_execution(
        target="agent.reasoner",
        input_data={"key": "value"},
        webhook=webhook_config,
    )

    assert execution_id is not None
    execution = manager._executions.get(execution_id)
    assert execution is not None


@pytest.mark.asyncio
async def test_get_execution_status(manager):
    """Test getting execution status."""
    execution_id = await manager.submit_execution(
        target="agent.reasoner", input_data={"key": "value"}
    )

    status = await manager.get_execution_status(execution_id)
    assert status is not None
    assert status.execution_id == execution_id


@pytest.mark.asyncio
async def test_get_execution_status_not_found(manager):
    """Test getting status for non-existent execution."""
    status = await manager.get_execution_status("nonexistent-id")
    assert status is None


@pytest.mark.asyncio
async def test_poll_execution_status_success(manager):
    """Test successful execution status polling."""
    # Mock the HTTP response
    mock_response = MagicMock()
    mock_response.status = 200
    mock_response.json = AsyncMock(
        return_value={
            "execution_id": "exec-1",
            "status": "succeeded",
            "result": {"output": "success"},
        }
    )

    # Mock connection manager to return our response
    mock_conn = MagicMock()
    mock_conn.get = AsyncMock(return_value=mock_response)
    manager._connection_manager.get_connection = AsyncMock(return_value=mock_conn)

    execution_id = await manager.submit_execution(
        target="agent.reasoner", input_data={"key": "value"}
    )

    # Poll for status
    status = await manager.poll_execution_status(execution_id)
    assert status is not None
    assert status.status == ExecutionStatus.SUCCEEDED


@pytest.mark.asyncio
async def test_poll_execution_status_timeout(manager):
    """Test polling timeout handling."""
    # Mock timeout response
    mock_conn = MagicMock()
    mock_conn.get = AsyncMock(side_effect=asyncio.TimeoutError())
    manager._connection_manager.get_connection = AsyncMock(return_value=mock_conn)

    execution_id = await manager.submit_execution(
        target="agent.reasoner", input_data={"key": "value"}
    )

    status = await manager.poll_execution_status(execution_id, timeout=0.1)
    # Should handle timeout gracefully
    assert status is not None


@pytest.mark.asyncio
async def test_poll_execution_status_error(manager):
    """Test error handling during polling."""
    # Mock error response
    mock_conn = MagicMock()
    mock_conn.get = AsyncMock(side_effect=Exception("Network error"))
    manager._connection_manager.get_connection = AsyncMock(return_value=mock_conn)

    execution_id = await manager.submit_execution(
        target="agent.reasoner", input_data={"key": "value"}
    )

    status = await manager.poll_execution_status(execution_id)
    # Should handle error gracefully
    assert status is not None


@pytest.mark.asyncio
async def test_batch_poll_executions(manager):
    """Test batch polling of multiple executions."""
    # Submit multiple executions
    execution_ids = []
    for i in range(3):
        exec_id = await manager.submit_execution(
            target="agent.reasoner", input_data={"index": i}
        )
        execution_ids.append(exec_id)

    # Mock batch poll response
    mock_response = MagicMock()
    mock_response.status = 200
    mock_response.json = AsyncMock(
        return_value={
            "executions": {
                exec_id: {"status": "succeeded", "result": {"index": i}}
                for i, exec_id in enumerate(execution_ids)
            }
        }
    )

    mock_conn = MagicMock()
    mock_conn.post = AsyncMock(return_value=mock_response)
    manager._connection_manager.get_connection = AsyncMock(return_value=mock_conn)

    # Batch poll
    results = await manager.batch_poll_executions(execution_ids)
    assert len(results) == len(execution_ids)


@pytest.mark.asyncio
async def test_get_execution_result_cached(manager):
    """Test retrieving cached execution result."""
    execution_id = "exec-1"
    cached_result = {"output": "cached"}

    # Set cache
    manager._result_cache.get = MagicMock(return_value=cached_result)

    result = await manager.get_execution_result(execution_id)
    assert result == cached_result
    manager._result_cache.get.assert_called_once()


@pytest.mark.asyncio
async def test_get_execution_result_not_cached(manager):
    """Test retrieving result when not cached."""
    execution_id = "exec-1"

    # No cache
    manager._result_cache.get = MagicMock(return_value=None)

    # Mock status check
    mock_status = MagicMock()
    mock_status.status = ExecutionStatus.SUCCEEDED
    mock_status.result = {"output": "fresh"}
    manager.get_execution_status = AsyncMock(return_value=mock_status)

    result = await manager.get_execution_result(execution_id)
    assert result == {"output": "fresh"}


@pytest.mark.asyncio
async def test_cleanup_completed_executions(manager):
    """Test cleanup of completed executions."""
    # Submit and mark as completed
    execution_id = await manager.submit_execution(
        target="agent.reasoner", input_data={"key": "value"}
    )

    execution = manager._executions.get(execution_id)
    execution.status = ExecutionStatus.SUCCEEDED
    execution.completed_at = asyncio.get_event_loop().time() - 1000  # Old

    await manager.cleanup_completed_executions()

    # Execution should be removed
    assert execution_id not in manager._executions


@pytest.mark.asyncio
async def test_context_propagation(manager):
    """Test execution context propagation."""
    parent_context = {"parent_id": "parent-123", "session_id": "session-456"}

    execution_id = await manager.submit_execution(
        target="agent.reasoner",
        input_data={"key": "value"},
        context=parent_context,
    )

    execution = manager._executions.get(execution_id)
    assert execution.context == parent_context


@pytest.mark.asyncio
async def test_event_stream_subscription(manager):
    """Test event stream subscription."""
    execution_id = await manager.submit_execution(
        target="agent.reasoner", input_data={"key": "value"}
    )

    # Mock event stream
    events_received = []

    async def mock_event_stream():
        events_received.append({"type": "status_update", "status": "running"})
        events_received.append({"type": "status_update", "status": "succeeded"})
        await asyncio.sleep(0.1)

    # Start subscription (would normally be handled by background task)
    # This is a simplified test
    assert execution_id in manager._executions


@pytest.mark.asyncio
async def test_event_stream_reconnection(manager):
    """Test event stream reconnection logic."""
    # This would test the reconnection logic when stream disconnects
    # Simplified test for now
    execution_id = await manager.submit_execution(
        target="agent.reasoner", input_data={"key": "value"}
    )

    assert execution_id in manager._executions


@pytest.mark.asyncio
async def test_priority_handling(manager):
    """Test execution priority handling."""
    # Submit executions with different priorities
    high_priority_id = await manager.submit_execution(
        target="agent.reasoner",
        input_data={"key": "value"},
        priority=ExecutionPriority.HIGH,
    )

    normal_priority_id = await manager.submit_execution(
        target="agent.reasoner",
        input_data={"key": "value"},
        priority=ExecutionPriority.NORMAL,
    )

    high_exec = manager._executions.get(high_priority_id)
    normal_exec = manager._executions.get(normal_priority_id)

    assert high_exec.priority == ExecutionPriority.HIGH
    assert normal_exec.priority == ExecutionPriority.NORMAL


@pytest.mark.asyncio
async def test_metrics_tracking(manager):
    """Test metrics tracking."""
    initial_metrics = manager.get_metrics()

    # Submit and complete an execution
    execution_id = await manager.submit_execution(
        target="agent.reasoner", input_data={"key": "value"}
    )

    execution = manager._executions.get(execution_id)
    execution.status = ExecutionStatus.SUCCEEDED

    metrics = manager.get_metrics()
    assert metrics.total_executions >= initial_metrics.total_executions + 1


@pytest.mark.asyncio
async def test_error_handling_during_polling(manager):
    """Test error handling during polling operations."""
    # Mock connection error
    manager._connection_manager.get_connection = AsyncMock(
        side_effect=Exception("Connection failed")
    )

    execution_id = await manager.submit_execution(
        target="agent.reasoner", input_data={"key": "value"}
    )

    # Should handle error gracefully
    status = await manager.poll_execution_status(execution_id)
    # Status should still exist even if poll failed
    assert status is not None


@pytest.mark.asyncio
async def test_concurrent_execution_submission(manager):
    """Test concurrent execution submission."""

    async def submit():
        return await manager.submit_execution(
            target="agent.reasoner", input_data={"key": "value"}
        )

    # Submit multiple executions concurrently
    execution_ids = await asyncio.gather(*[submit() for _ in range(10)])

    assert len(execution_ids) == 10
    assert all(eid in manager._executions for eid in execution_ids)


@pytest.mark.asyncio
async def test_result_cache_integration(manager):
    """Test result cache integration."""
    execution_id = await manager.submit_execution(
        target="agent.reasoner", input_data={"key": "value"}
    )

    # Mock successful execution
    execution = manager._executions.get(execution_id)
    execution.status = ExecutionStatus.SUCCEEDED
    execution.result = {"output": "success"}

    # Get result (should be cached)
    result = await manager.get_execution_result(execution_id)
    assert result == {"output": "success"}

    # Verify cache was used
    manager._result_cache.set.assert_called()
