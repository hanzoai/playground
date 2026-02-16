"""
RAG Evaluation Bots

Multi-bot architectures for comprehensive RAG evaluation:
- Faithfulness: Adversarial debate (Prosecutor vs Defender)
- Relevance: Multi-jury consensus voting
- Hallucination: Hybrid ML+LLM chain-of-verification
- Constitutional: Configurable principles-based evaluation
"""

from playground import BotRouter

# Create the main router
router = BotRouter(tags=["rag-evaluation"])

# Import and register all bot modules
from .faithfulness import register_faithfulness_bots
from .relevance import register_relevance_bots
from .hallucination import register_hallucination_bots
from .constitutional import register_constitutional_bots
from .orchestrator import register_orchestrator_bots

# Register all bots with the router
register_faithfulness_bots(router)
register_relevance_bots(router)
register_hallucination_bots(router)
register_constitutional_bots(router)
register_orchestrator_bots(router)

__all__ = ["router"]
