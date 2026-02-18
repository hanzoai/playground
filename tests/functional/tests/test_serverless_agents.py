"""
End-to-end validation for serverless bot nodes across SDKs.

Each test spins up a lightweight serverless handler (Python, TypeScript, Go),
registers it through the control plane's `/nodes/register-serverless` endpoint,
and executes a bot via the normal execution gateway to ensure discovery,
invocation, and parent/child call wiring all work without heartbeats.
"""

from __future__ import annotations

import asyncio
import json
import os
import shutil
import socket
import sys
import threading
from contextlib import asynccontextmanager
from pathlib import Path
from typing import AsyncIterator, Optional, Tuple

import pytest
import uvicorn
from playground import Bot
from playground.async_config import AsyncConfig
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

from utils import run_go_bot, unique_node_id

TEST_BIND_HOST = os.environ.get(
    "TEST_BOT_BIND_HOST",
    os.environ.get("TEST_AGENT_BIND_HOST", "0.0.0.0"),
)
TEST_CALLBACK_HOST = os.environ.get(
    "TEST_BOT_CALLBACK_HOST",
    os.environ.get("TEST_AGENT_CALLBACK_HOST", "test-runner"),
)


def _get_free_port(host: str = TEST_BIND_HOST) -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind((host, 0))
        return s.getsockname()[1]


async def _wait_for_port(host: str, port: int, timeout: float = 15.0, process=None):
    deadline = asyncio.get_event_loop().time() + timeout
    last_error: Optional[BaseException] = None
    while asyncio.get_event_loop().time() < deadline:
        if process and process.returncode is not None:
            stdout, stderr = await process.communicate()
            raise AssertionError(
                f"Process exited early (code {process.returncode}). "
                f"stdout={stdout.decode()} stderr={stderr.decode()}"
            )
        try:
            reader, writer = await asyncio.open_connection(host=host, port=port)
            writer.close()
            await writer.wait_closed()
            return
        except (ConnectionRefusedError, OSError) as exc:  # noqa: PERF203
            last_error = exc
            await asyncio.sleep(0.2)
    raise AssertionError(f"Port {host}:{port} did not open in time: {last_error}")


async def _register_serverless(_async_http_client, invocation_url: str, *, retries: int = 6):
    """
    Register a serverless function using the CLI exactly as documented.

    The control plane Docker image already builds and installs the CLI as `playground`,
    so we treat a missing CLI as a hard failure rather than silently falling
    back to the HTTP API. Retries help absorb the control plane coming online.
    """
    last_error = None
    for attempt in range(retries):
        cli_result = await _register_serverless_via_cli(invocation_url)
        if cli_result.get("ok"):
            return cli_result
        last_error = cli_result
        await asyncio.sleep(0.5)

    raise AssertionError(f"playground nodes register-serverless failed: {last_error}")


async def _register_serverless_via_cli(invocation_url: str):
    bin_override = os.environ.get("PLAYGROUND_CLI") or os.environ.get("AF_BIN") or os.environ.get("AGENTS_CLI")
    candidates = [bin_override] if bin_override else []
    candidates.extend(["playground", "af"])

    cli_bin: Optional[str] = None
    for candidate in candidates:
        if not candidate:
            continue
        path = shutil.which(candidate)
        if path:
            cli_bin = path
            break

    if not cli_bin:
        return {"ok": False, "error": "missing-cli", "candidates": candidates}

    env = os.environ.copy()
    env.setdefault("PLAYGROUND_SERVER", env.get("CONTROL_PLANE_URL", env.get("AGENTS_SERVER", "http://localhost:8080")))
    # Backward-compatible fallback
    env.setdefault("AGENTS_SERVER", env.get("PLAYGROUND_SERVER", "http://localhost:8080"))
    token = env.get("PLAYGROUND_TOKEN") or env.get("AGENTS_TOKEN")

    cmd = [cli_bin, "nodes", "register-serverless", "--url", invocation_url, "--json"]
    if token:
        cmd.extend(["--token", token])

    try:
        proc = await asyncio.create_subprocess_exec(
            *cmd,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
            env=env,
        )
    except FileNotFoundError:
        return {"ok": False, "error": "missing-cli"}

    stdout, stderr = await proc.communicate()
    if proc.returncode != 0:
        return {
            "ok": False,
            "error": "cli-failed",
            "code": proc.returncode,
            "stderr": stderr.decode(),
            "stdout": stdout.decode(),
        }

    payload = {}
    if stdout:
        try:
            payload = json.loads(stdout.decode())
        except json.JSONDecodeError:
            payload = {"raw": stdout.decode()}
    return {"ok": True, "response": payload}


@asynccontextmanager
async def run_python_serverless_bot(node_id: str, control_plane_url: str) -> AsyncIterator[str]:
    """
    Start a lightweight FastAPI wrapper that delegates to Bot.handle_serverless.
    """
    bot = Bot(
        node_id=node_id,
        playground_server=control_plane_url,
        auto_register=False,
        dev_mode=True,
        async_config=AsyncConfig(enable_async_execution=False, fallback_to_sync=True),
    )

    @bot.bot()
    async def hello(name: str = "Playground") -> dict:  # type: ignore[return-type]
        ctx = bot.ctx
        return {
            "greeting": f"Hello, {name}!",
            "run_id": getattr(ctx, "workflow_id", None),
            "execution_id": getattr(ctx, "execution_id", None),
            "parent_execution_id": getattr(ctx, "parent_execution_id", None),
        }

    @bot.bot()
    async def relay(target: str, message: str = "ping") -> dict:  # type: ignore[return-type]
        downstream = await bot.call(target, name=message)
        return {"downstream": downstream, "parent_execution_id": getattr(bot.ctx, "execution_id", None)}

    fastapi_app = FastAPI()

    @fastapi_app.get("/discover")
    async def discover():
        return await asyncio.to_thread(bot.handle_serverless, {"path": "/discover"})

    async def _handle(request: Request, override_path: Optional[str] = None):
        payload = await request.json()
        path = override_path or payload.get("path") or "/execute"
        result = await asyncio.to_thread(bot.handle_serverless, {"path": path, **payload})
        status = result.get("statusCode", 200)
        body = result.get("body", result)
        return JSONResponse(content=body, status_code=status)

    @fastapi_app.post("/execute")
    async def execute(request: Request):
        return await _handle(request, "/execute")

    @fastapi_app.post("/{full_path:path}")
    async def execute_catch_all(full_path: str, request: Request):
        return await _handle(request, f"/{full_path}")

    port = _get_free_port()
    config = uvicorn.Config(
        app=fastapi_app,
        host=TEST_BIND_HOST,
        port=port,
        log_level="warning",
        access_log=True,
    )
    server = uvicorn.Server(config)
    loop = asyncio.new_event_loop()

    def run_server():
        asyncio.set_event_loop(loop)
        loop.run_until_complete(server.serve())

    thread = threading.Thread(target=run_server, daemon=True)
    thread.start()
    await asyncio.sleep(0.5)

    try:
        yield f"http://{TEST_CALLBACK_HOST}:{port}"
    finally:
        server.should_exit = True
        if loop.is_running():
            loop.call_soon_threadsafe(lambda: None)
        thread.join(timeout=10)


@asynccontextmanager
async def run_ts_serverless_bot(node_id: str, control_plane_url: str) -> AsyncIterator[Tuple[str, asyncio.subprocess.Process]]:
    port = _get_free_port()
    env = os.environ.copy()
    env.update(
        {
            "TS_AGENT_ID": node_id,
            "TS_AGENT_PORT": str(port),
            "TS_AGENT_BIND_HOST": TEST_BIND_HOST,
            "PLAYGROUND_SERVER": control_plane_url,
            # Backward-compatible fallback
            "AGENTS_SERVER": control_plane_url,
        }
    )
    env.setdefault("NODE_PATH", "/usr/local/lib/node_modules:/usr/lib/node_modules")
    script_path = Path(__file__).resolve().parent.parent / "ts_bots" / "serverless-agent.mjs"

    process = await asyncio.create_subprocess_exec(
        "node",
        str(script_path),
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
        env=env,
    )

    try:
        await _wait_for_port("127.0.0.1", port, process=process)
        yield f"http://{TEST_CALLBACK_HOST}:{port}", process
    finally:
        if process.returncode is None:
            process.terminate()
            try:
                await asyncio.wait_for(process.wait(), timeout=10)
            except asyncio.TimeoutError:
                process.kill()
                await process.wait()


@asynccontextmanager
async def run_go_serverless_bot(node_id: str, control_plane_url: str) -> AsyncIterator[str]:
    port = _get_free_port()
    env = {
        **os.environ,
        "HANZO_NODE_ID": node_id,
        "PLAYGROUND_URL": control_plane_url,
        "PORT": str(port),
        "PLAYGROUND_TOKEN": os.environ.get("PLAYGROUND_TOKEN", os.environ.get("AGENTS_TOKEN", "")),
        # Backward-compatible fallback for older Go SDK binaries
        "AGENT_NODE_ID": node_id,
        "AGENTS_URL": control_plane_url,
        "AGENTS_TOKEN": os.environ.get("AGENTS_TOKEN", ""),
    }

    async with run_go_bot("serverless", env=env) as proc:
        await _wait_for_port("127.0.0.1", port, process=proc.process)
        yield f"http://{TEST_CALLBACK_HOST}:{port}"


@pytest.mark.functional
@pytest.mark.asyncio
async def test_python_serverless_bot_registers_and_executes(async_http_client, control_plane_url):
    node_id = unique_node_id("py-svless")

    async with run_python_serverless_bot(node_id, control_plane_url) as invocation_url:
        await _register_serverless(async_http_client, invocation_url)

        resp = await async_http_client.post(
            f"/api/v1/bots/{node_id}.hello",
            json={"input": {"name": "Lambda"}},
            timeout=30.0,
        )
        assert resp.status_code == 200, resp.text
        body = resp.json()
        result = body.get("result", {})
        assert result.get("greeting") == "Hello, Lambda!"
        assert result.get("execution_id"), "execution_id should propagate to serverless bot"


@pytest.mark.functional
@pytest.mark.asyncio
async def test_serverless_python_chain_calls(async_http_client, control_plane_url):
    child_id = unique_node_id("py-svless-child")
    parent_id = unique_node_id("py-svless-parent")

    async with run_python_serverless_bot(child_id, control_plane_url) as child_url:
        await _register_serverless(async_http_client, child_url)

        async with run_python_serverless_bot(parent_id, control_plane_url) as parent_url:
            await _register_serverless(async_http_client, parent_url)

            resp = await async_http_client.post(
                f"/api/v1/bots/{parent_id}.relay",
                json={"input": {"target": f"{child_id}.hello", "message": "hi-child"}},
                timeout=40.0,
            )
            assert resp.status_code == 200, resp.text
            result = resp.json().get("result", {})
            assert result.get("downstream", {}).get("greeting") == "Hello, hi-child!"
            assert result.get("parent_execution_id"), "parent execution id should be set on relay bot"


@pytest.mark.functional
@pytest.mark.asyncio
async def test_typescript_serverless_bot(async_http_client, control_plane_url):
    node_id = unique_node_id("ts-svless")

    async with run_ts_serverless_bot(node_id, control_plane_url) as (invocation_url, process):
        await _register_serverless(async_http_client, invocation_url)

        resp = await async_http_client.post(
            f"/api/v1/bots/{node_id}.hello",
            json={"input": {"name": "TS Lambda"}},
            timeout=30.0,
        )

        if resp.status_code != 200:
            stdout, stderr = await process.communicate()
            print("TS serverless stdout:", stdout.decode(), file=sys.stderr)
            print("TS serverless stderr:", stderr.decode(), file=sys.stderr)

        assert resp.status_code == 200, resp.text
        result = resp.json().get("result", {})
        assert result.get("greeting") == "Hello, TS Lambda!"
        exec_id = result.get("execution_id") or result.get("executionId")
        assert exec_id


@pytest.mark.functional
@pytest.mark.asyncio
async def test_typescript_serverless_chain(async_http_client, control_plane_url):
    child_id = unique_node_id("ts-svless-child")
    parent_id = unique_node_id("ts-svless-parent")

    async with run_ts_serverless_bot(child_id, control_plane_url) as (child_url, child_process):
        await _register_serverless(async_http_client, child_url)

        async with run_ts_serverless_bot(parent_id, control_plane_url) as (
            parent_url,
            parent_process,
        ):
            await _register_serverless(async_http_client, parent_url)

            resp = await async_http_client.post(
                f"/api/v1/bots/{parent_id}.relay",
                json={"input": {"target": f"{child_id}.hello", "name": "ts-child"}},
                timeout=40.0,
            )

            if resp.status_code != 200:
                # Collect logs for debugging without blocking indefinitely if the process is still alive.
                if child_process.returncode is None:
                    child_process.terminate()
                child_stdout, child_stderr = await child_process.communicate()

                if parent_process.returncode is None:
                    parent_process.terminate()
                parent_stdout, parent_stderr = await parent_process.communicate()

                print("TS child stdout:", child_stdout.decode(), file=sys.stderr)
                print("TS child stderr:", child_stderr.decode(), file=sys.stderr)
                print("TS parent stdout:", parent_stdout.decode(), file=sys.stderr)
                print("TS parent stderr:", parent_stderr.decode(), file=sys.stderr)

            assert resp.status_code == 200, resp.text
            result = resp.json().get("result", {})
            downstream = result.get("downstream", {})
            assert downstream.get("greeting") == "Hello, ts-child!"
            assert downstream.get("executionId") or downstream.get("execution_id"), "child execution id should propagate"


@pytest.mark.functional
@pytest.mark.asyncio
async def test_go_serverless_bot(async_http_client, control_plane_url):
    node_id = unique_node_id("go-svless")

    async with run_go_serverless_bot(node_id, control_plane_url) as invocation_url:
        await _register_serverless(async_http_client, invocation_url)

        resp = await async_http_client.post(
            f"/api/v1/bots/{node_id}.hello",
            json={"input": {"name": "gopher"}},
            timeout=30.0,
        )
        assert resp.status_code == 200, resp.text
        result = resp.json().get("result", {})
        assert result.get("greeting") == "Hello, gopher!"
        assert result.get("execution_id")


@pytest.mark.functional
@pytest.mark.asyncio
async def test_go_serverless_chain(async_http_client, control_plane_url):
    child_id = unique_node_id("go-svless-child")
    parent_id = unique_node_id("go-svless-parent")

    async with run_go_serverless_bot(child_id, control_plane_url) as child_url:
        await _register_serverless(async_http_client, child_url)

        async with run_go_serverless_bot(parent_id, control_plane_url) as parent_url:
            await _register_serverless(async_http_client, parent_url)

            resp = await async_http_client.post(
                f"/api/v1/bots/{parent_id}.relay",
                json={"input": {"target": f"{child_id}.hello", "message": "gopher-child"}},
                timeout=40.0,
            )
            assert resp.status_code == 200, resp.text

            result = resp.json().get("result", {})
            downstream = result.get("downstream", {})
            assert downstream.get("greeting") == "Hello, gopher-child!"
            assert downstream.get("execution_id"), "child execution id should propagate"
