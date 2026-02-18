import pytest

from playground.router import BotRouter


class DummyAgent:
    def __init__(self):
        self.calls = []

    async def ai(self, *args, **kwargs):
        self.calls.append(("ai", args, kwargs))
        return "ai-called"

    async def call(self, target, *args, **kwargs):
        self.calls.append((target, args, kwargs))
        return "call-result"

    def note(self, message: str, tags=None):
        self.calls.append(("note", (message,), {"tags": tags}))
        return "note-logged"

    def discover(self, **kwargs):
        self.calls.append(("discover", (), kwargs))
        return "discovery-result"

    @property
    def memory(self):
        return "memory-client"


@pytest.mark.asyncio
async def test_router_requires_agent_before_use():
    router = BotRouter()

    with pytest.raises(RuntimeError):
        await router.call("node.skill")

    agent = DummyAgent()
    router._attach_bot(agent)

    result = await router.call("node.skill", 1, mode="fast")
    assert result == "call-result"
    assert agent.calls == [("node.skill", (1,), {"mode": "fast"})]

    ai_result = await router.ai("gpt")
    assert ai_result == "ai-called"

    assert router.memory == "memory-client"


def test_bot_and_skill_registration():
    router = BotRouter(prefix="/api/v1", tags=["base"])

    @router.bot(path="/foo")
    def sample_bot():
        return "bot"

    @router.skill(tags=["extra"], path="tool")
    def sample_skill():
        return "skill"

    # The decorator returns a wrapper; original func is stored in entry["func"]
    # and also accessible via wrapper._original_func
    assert router.bots[0]["func"] is sample_bot._original_func
    assert router.bots[0]["wrapper"] is sample_bot
    assert router.bots[0]["path"] == "/foo"
    assert router.bots[0]["tags"] == ["base"]

    skill_entry = router.skills[0]
    assert skill_entry["func"] is sample_skill
    assert skill_entry["tags"] == ["base", "extra"]
    assert skill_entry["path"] == "tool"


def test_router_supports_parentheses_free_decorators():
    router = BotRouter()

    @router.bot
    def inline_bot():
        return "ok"

    @router.skill
    def inline_skill():
        return "ok"

    # The decorator returns a wrapper for bots
    assert router.bots[0]["func"] is inline_bot._original_func
    assert router.bots[0]["wrapper"] is inline_bot
    assert router.bots[0]["path"] is None
    assert router.skills[0]["func"] is inline_skill
    assert router.skills[0]["path"] is None


@pytest.mark.parametrize(
    "prefix,default,custom,expected",
    [
        ("", None, None, None),
        ("/api", "/items", None, "/api/items"),
        ("api/", None, "detail", "/api/detail"),
        ("/root/", "default", "custom", "/root/custom"),
        ("", "default", None, "/default"),
        ("group", "/bots/foo", None, "/bots/group/foo"),
    ],
)
def test_combine_path(prefix, default, custom, expected):
    router = BotRouter(prefix=prefix)
    assert router._combine_path(default, custom) == expected


def test_router_automatic_delegation():
    """Test that BotRouter automatically delegates all Agent methods via __getattr__."""
    router = BotRouter()
    agent = DummyAgent()
    router._attach_bot(agent)

    # Test note() delegation (the original issue)
    note_result = router.note("Test message", tags=["debug"])
    assert note_result == "note-logged"
    assert agent.calls[-1] == ("note", ("Test message",), {"tags": ["debug"]})

    # Test discover() delegation (future-proofing)
    discover_result = router.discover(agent="test_agent", tags=["api"])
    assert discover_result == "discovery-result"
    assert agent.calls[-1] == ("discover", (), {"agent": "test_agent", "tags": ["api"]})

    # Test property access (memory)
    assert router.memory == "memory-client"

    # Test app property
    assert router.app is agent


def test_router_delegation_without_agent_raises_error():
    """Test that accessing delegated methods without an attached agent raises RuntimeError."""
    router = BotRouter()

    # Test that note() raises RuntimeError when no agent is attached
    with pytest.raises(RuntimeError, match="Router not attached to a bot"):
        router.note("Test message")

    # Test that discover() raises RuntimeError when no agent is attached
    with pytest.raises(RuntimeError, match="Router not attached to a bot"):
        router.discover()

    # Test that memory raises RuntimeError when no agent is attached
    with pytest.raises(RuntimeError, match="Router not attached to a bot"):
        _ = router.memory
