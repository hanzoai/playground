#!/usr/bin/env zsh
# Agent Runtime Selector
# Reads AGENT_RUNTIME env var to determine which AI agent to launch.

set -euo pipefail

RUNTIME="${AGENT_RUNTIME:-hanzo-dev}"

case "$RUNTIME" in
  hanzo-dev)
    echo "[agent] Starting Hanzo Dev (native ZAP)..."
    if [ -n "${HANZO_API_KEY:-}" ]; then
      export HANZO_API_KEY
    fi
    exec hanzo-dev --server "${PLAYGROUND_SERVER:-}" --node-id "${AGENT_NODE_ID:-agent}" --security full --ask off
    ;;
  claude)
    echo "[agent] Starting Claude Code..."
    if [ -z "${ANTHROPIC_API_KEY:-}" ]; then
      echo "[agent] ANTHROPIC_API_KEY not set. Falling back to terminal."
      exec zsh
    fi
    exec claude --dangerously-skip-permissions
    ;;
  gemini)
    echo "[agent] Starting Gemini CLI..."
    if [ -z "${GOOGLE_API_KEY:-}" ]; then
      echo "[agent] GOOGLE_API_KEY not set. Falling back to terminal."
      exec zsh
    fi
    exec gemini
    ;;
  qwen)
    echo "[agent] Qwen agent mode. Configure QWEN_API_KEY to enable."
    exec zsh
    ;;
  grok)
    echo "[agent] Grok mode. Configure XAI_API_KEY to enable."
    exec zsh
    ;;
  terminal|*)
    echo "[agent] Starting terminal..."
    exec zsh
    ;;
esac
