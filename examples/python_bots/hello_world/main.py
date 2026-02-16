"""
Hello World Bot - Minimal Playground Example

Demonstrates:
- One skill (deterministic function)
- Two bots (AI-powered functions)
- Call graph: say_hello → get_greeting (skill) + add_emoji (bot)
"""

from playground import Bot
from playground import AIConfig
from pydantic import BaseModel
import os

# Initialize bot
app = Bot(
    node_id="hello-world",
    playground_server=os.getenv("PLAYGROUND_URL", "http://localhost:8080"),
    ai_config=AIConfig(
        model=os.getenv("SMALL_MODEL", "openai/gpt-4o-mini"), temperature=0.7
    ),
)

# ============= SKILL (DETERMINISTIC) =============


@app.skill()
def get_greeting(name: str) -> dict:
    """Returns a greeting template (deterministic - no AI)"""
    return {"message": f"Hello, {name}! Welcome to Playground."}


# ============= REASONERS (AI-POWERED) =============


class EmojiResult(BaseModel):
    """Simple schema for emoji addition"""

    text: str
    emoji: str


@app.bot()
async def add_emoji(text: str) -> EmojiResult:
    """Uses AI to add an appropriate emoji to text"""
    return await app.ai(
        user=f"Add one appropriate emoji to this greeting: {text}", schema=EmojiResult
    )


@app.bot()
async def say_hello(name: str) -> dict:
    """
    Main entry point - orchestrates skill and bot.

    Call graph:
    say_hello (entry point)
    ├─→ get_greeting (skill)
    └─→ add_emoji (bot)
    """
    # Step 1: Get greeting from skill (deterministic)
    greeting = get_greeting(name)

    # Step 2: Add emoji using AI (bot)
    result = await add_emoji(greeting["message"])

    return {"greeting": result.text, "emoji": result.emoji, "name": name}


# ============= START SERVER OR CLI =============

if __name__ == "__main__":
    print("🚀 Hello World Agent")
    print("📍 Node: hello-world")
    print("🌐 Control Plane: http://localhost:8080")

    # Universal entry point - auto-detects CLI vs server mode
    app.run(auto_port=True)
