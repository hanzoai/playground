from playground.bot_registry import (
    set_current_bot,
    get_current_bot_instance,
    clear_current_bot,
)


class DummyAgent:
    pass


def test_agent_registry_roundtrip():
    clear_current_bot()
    assert get_current_bot_instance() is None

    agent = DummyAgent()
    set_current_bot(agent)
    assert get_current_bot_instance() is agent

    clear_current_bot()
    assert get_current_bot_instance() is None
