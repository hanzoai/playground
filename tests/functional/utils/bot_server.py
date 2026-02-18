"""
Utilities for running test bots inside functional tests.

The `run_bot_server` async context manager handles spinning up a FastAPI
server via uvicorn, registering the bot with the control plane, and then
cleanly shutting everything down when the test completes.
"""

from __future__ import annotations

import asyncio
import os
import socket
import threading
from contextlib import asynccontextmanager
from dataclasses import dataclass
from typing import AsyncIterator

import uvicorn
from playground import Bot

BOT_BIND_HOST = os.environ.get(
    "BOT_BIND_HOST",
    os.environ.get("TEST_AGENT_BIND_HOST", "127.0.0.1"),
)
BOT_CALLBACK_HOST = os.environ.get(
    "BOT_CALLBACK_HOST",
    os.environ.get("TEST_AGENT_CALLBACK_HOST", "127.0.0.1"),
)


@dataclass
class RunningBot:
    """Metadata about a running bot server."""

    bot: Bot
    port: int
    base_url: str


# Backward-compatible alias
RunningAgent = RunningBot


@asynccontextmanager
async def run_bot_server(
    bot: Bot,
    *,
    bind_host: str = BOT_BIND_HOST,
    callback_host: str = BOT_CALLBACK_HOST,
    startup_delay: float = 2.0,
    registration_delay: float = 2.0,
) -> AsyncIterator[RunningBot]:
    """
    Start the given bot in a background uvicorn server for the duration of a test.
    """
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind((bind_host, 0))
        port = s.getsockname()[1]

    bot.base_url = f"http://{callback_host}:{port}"

    config = uvicorn.Config(
        app=bot,
        host=bind_host,
        port=port,
        log_level="error",
        access_log=False,
    )
    server = uvicorn.Server(config)
    loop = asyncio.new_event_loop()

    def run_server():
        asyncio.set_event_loop(loop)
        loop.run_until_complete(server.serve())

    thread = threading.Thread(target=run_server, daemon=True)
    thread.start()

    await asyncio.sleep(startup_delay)

    try:
        await bot.agents_handler.register_with_playground_server(port)
        bot.agents_server = None

        # Registration runs on the pytest event loop, but bots execute on the
        # uvicorn event loop inside a background thread. Reset the Playground client
        # so async HTTP clients are re-created within the uvicorn loop to avoid
        # "bound to a different event loop" errors when performing memory operations.
        try:
            await bot.client.aclose()
        except AttributeError:
            pass

        await asyncio.sleep(registration_delay)

        yield RunningBot(bot=bot, port=port, base_url=bot.base_url)
    finally:
        server.should_exit = True
        if loop.is_running():
            loop.call_soon_threadsafe(lambda: None)
        thread.join(timeout=10)
