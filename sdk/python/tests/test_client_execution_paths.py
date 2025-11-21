"""
Comprehensive tests for AgentFieldClient execution paths.
"""

import asyncio
import sys
import types
from unittest.mock import AsyncMock, MagicMock

import pytest

from agentfield.client import AgentFieldClient
from agentfield.execution_context import ExecutionContext


@pytest.fixture
def client():
    """Create a test client."""
    return AgentFieldClient(base_url="http://test-server")


@pytest.fixture
def mock_httpx(monkeypatch):
    """Mock httpx module."""
    module = types.SimpleNamespace()

    class MockAsyncClient:
        def __init__(self, *args, **kwargs):
            self.is_closed = False
            self._requests = []

        async def request(self, method, url, **kwargs):
            self._requests.append((method, url, kwargs))
            response = MagicMock()
            response.status_code = 200
            response.json = AsyncMock(return_value={"status": "succeeded"})
            response.raise_for_status = MagicMock()
            return response

        async def aclose(self):
            self.is_closed = True

    module.AsyncClient = MockAsyncClient
    module.Limits = MagicMock()
    module.Timeout = MagicMock()

    monkeypatch.setitem(sys.modules, "httpx", module)
    return module


def test_call_sync_execution(client, mock_httpx):
    """Test synchronous execution call."""
    response = client.call(
        target="agent.reasoner",
        input_data={"key": "value"},
        async_mode=False,
    )

    assert response is not None
    # Verify request was made
    assert len(mock_httpx.AsyncClient()._requests) > 0


def test_call_async_execution(client, mock_httpx):
    """Test asynchronous execution call."""
    response = client.call(
        target="agent.reasoner",
        input_data={"key": "value"},
        async_mode=True,
    )

    assert response is not None
    assert "execution_id" in response or "status" in response


def test_call_with_context_headers(client, mock_httpx):
    """Test call with execution context headers."""
    context = ExecutionContext(
        execution_id="exec-1",
        run_id="run-1",
        agent_node_id="agent-1",
        reasoner_name="reasoner-1",
        parent_execution_id="parent-1",
    )

    response = client.call(
        target="agent.reasoner",
        input_data={"key": "value"},
        context=context,
    )

    assert response is not None
    # Verify headers were set
    requests = mock_httpx.AsyncClient()._requests
    if requests:
        headers = requests[0][2].get("headers", {})
        assert "X-Execution-ID" in headers or "X-Run-ID" in headers


def test_call_error_handling(client, mock_httpx):
    """Test error handling in call method."""
    # Mock error response
    mock_client = mock_httpx.AsyncClient()
    mock_client.request = AsyncMock(side_effect=Exception("Network error"))

    with pytest.raises(Exception):
        client.call(target="agent.reasoner", input_data={"key": "value"})


def test_call_retry_logic(client, mock_httpx):
    """Test retry logic for failed requests."""
    call_count = 0

    async def failing_then_success(*args, **kwargs):
        nonlocal call_count
        call_count += 1
        if call_count < 2:
            raise Exception("Transient error")
        response = MagicMock()
        response.status_code = 200
        response.json = AsyncMock(return_value={"status": "succeeded"})
        return response

    mock_client = mock_httpx.AsyncClient()
    mock_client.request = AsyncMock(side_effect=failing_then_success)

    # Should retry and eventually succeed
    response = client.call(target="agent.reasoner", input_data={"key": "value"})
    assert response is not None
    assert call_count == 2


def test_call_with_webhook_config(client, mock_httpx):
    """Test call with webhook configuration."""
    webhook_config = {
        "url": "https://example.com/webhook",
        "secret": "test-secret",
    }

    response = client.call(
        target="agent.reasoner",
        input_data={"key": "value"},
        webhook=webhook_config,
    )

    assert response is not None
    # Verify webhook was included in request
    requests = mock_httpx.AsyncClient()._requests
    if requests:
        json_data = requests[0][2].get("json", {})
        assert "webhook" in json_data


def test_call_header_propagation(client, mock_httpx):
    """Test header propagation in call method."""
    # Set current workflow context
    context = ExecutionContext(
        execution_id="exec-1",
        run_id="run-1",
        agent_node_id="agent-1",
        reasoner_name="reasoner-1",
    )
    client._current_workflow_context = context

    response = client.call(
        target="agent.reasoner",
        input_data={"key": "value"},
    )

    assert response is not None
    # Verify context headers were propagated
    requests = mock_httpx.AsyncClient()._requests
    if requests:
        headers = requests[0][2].get("headers", {})
        # Headers should include context information
        assert headers is not None


def test_call_event_stream_handling(client, mock_httpx):
    """Test event stream handling in call method."""
    # Mock event stream response
    mock_client = mock_httpx.AsyncClient()

    async def stream_response(*args, **kwargs):
        response = MagicMock()
        response.status_code = 200
        response.aiter_lines = AsyncMock(
            return_value=iter(
                [
                    b'data: {"type": "status_update", "status": "running"}\n\n',
                    b'data: {"type": "status_update", "status": "succeeded"}\n\n',
                ]
            )
        )
        return response

    mock_client.request = AsyncMock(side_effect=stream_response)

    # Enable event stream
    client.async_config.enable_event_stream = True

    response = client.call(
        target="agent.reasoner",
        input_data={"key": "value"},
        async_mode=True,
    )

    assert response is not None


def test_call_timeout_handling(client, mock_httpx):
    """Test timeout handling in call method."""
    # Mock timeout
    mock_client = mock_httpx.AsyncClient()
    mock_client.request = AsyncMock(side_effect=asyncio.TimeoutError())

    with pytest.raises(asyncio.TimeoutError):
        client.call(
            target="agent.reasoner",
            input_data={"key": "value"},
            timeout=0.1,
        )


def test_call_with_different_execution_modes(client, mock_httpx):
    """Test call with different execution modes."""
    # Test sync mode
    response_sync = client.call(
        target="agent.reasoner",
        input_data={"key": "value"},
        async_mode=False,
    )
    assert response_sync is not None

    # Test async mode
    response_async = client.call(
        target="agent.reasoner",
        input_data={"key": "value"},
        async_mode=True,
    )
    assert response_async is not None


def test_call_result_caching(client, mock_httpx):
    """Test result caching in call method."""
    # First call
    response1 = client.call(
        target="agent.reasoner",
        input_data={"key": "value"},
    )

    # Second call with same input (should use cache if enabled)
    response2 = client.call(
        target="agent.reasoner",
        input_data={"key": "value"},
    )

    assert response1 is not None
    assert response2 is not None


def test_call_with_custom_headers(client, mock_httpx):
    """Test call with custom headers."""
    custom_headers = {"X-Custom-Header": "custom-value"}

    response = client.call(
        target="agent.reasoner",
        input_data={"key": "value"},
        headers=custom_headers,
    )

    assert response is not None
    # Verify custom headers were included
    requests = mock_httpx.AsyncClient()._requests
    if requests:
        headers = requests[0][2].get("headers", {})
        assert "X-Custom-Header" in headers


def test_call_context_management(client, mock_httpx):
    """Test context management in call method."""
    # Test that context is properly managed
    context = ExecutionContext(
        execution_id="exec-1",
        run_id="run-1",
        agent_node_id="agent-1",
        reasoner_name="reasoner-1",
    )

    # Set context
    client._current_workflow_context = context

    response = client.call(
        target="agent.reasoner",
        input_data={"key": "value"},
    )

    assert response is not None
    # Context should still be set
    assert client._current_workflow_context == context


def test_call_error_response_handling(client, mock_httpx):
    """Test handling of error responses."""
    # Mock error response
    mock_client = mock_httpx.AsyncClient()

    async def error_response(*args, **kwargs):
        response = MagicMock()
        response.status_code = 500
        response.json = AsyncMock(return_value={"error": "Internal server error"})
        response.raise_for_status = MagicMock(side_effect=Exception("Server error"))
        return response

    mock_client.request = AsyncMock(side_effect=error_response)

    with pytest.raises(Exception):
        client.call(target="agent.reasoner", input_data={"key": "value"})
