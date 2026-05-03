/**
 * Canvas Types
 */

import type { Node, Edge, Viewport } from '@xyflow/react';
import type { BotStatus } from './gateway';

// ---------------------------------------------------------------------------
// Agent Runtime
// ---------------------------------------------------------------------------

export type AgentRuntime = 'hanzo-dev' | 'claude' | 'gemini' | 'qwen' | 'grok' | 'terminal';

export const AGENT_RUNTIMES: { key: AgentRuntime; label: string; icon: string; description: string; auth: 'hanzo' | 'custom' | 'none' }[] = [
  { key: 'hanzo-dev', label: 'Hanzo Dev',   icon: '\u26A1',     description: 'Native ZAP protocol, IAM login, integrated payments', auth: 'hanzo' },
  { key: 'claude',    label: 'Claude Code',  icon: '\uD83D\uDFE3', description: 'Anthropic Claude with hanzo-mcp',                    auth: 'custom' },
  { key: 'gemini',    label: 'Gemini CLI',   icon: '\uD83D\uDD35', description: 'Google Gemini agent',                                auth: 'custom' },
  { key: 'qwen',      label: 'Qwen Agent',   icon: '\uD83D\uDFE2', description: 'Qwen3 with tool use',                                auth: 'custom' },
  { key: 'grok',      label: 'Grok',         icon: '\u26AA',     description: 'xAI Grok agent',                                     auth: 'custom' },
  { key: 'terminal',  label: 'Terminal',      icon: '\u2328\uFE0F', description: 'Plain zsh/bash shell',                               auth: 'none' },
];

// ---------------------------------------------------------------------------
// Data Interfaces
// ---------------------------------------------------------------------------

export type BotView = 'overview' | 'terminal' | 'chat' | 'operative' | 'files';

export interface Bot {
  agentId: string;
  name: string;
  emoji?: string;
  avatar?: string;
  role?: string;
  color?: string;
  personality?: string;
  status: BotStatus;
  sessionKey?: string;
  model?: string;
  workspace?: string;
  lastActivity?: string;
  activeView: BotView;
  source: 'local' | 'cloud';
  teamId?: string;
  /** Cloud node fields — only set when source === 'cloud' */
  podName?: string;
  namespace?: string;
  endpoint?: string;
  image?: string;
  owner?: string;
  org?: string;
  skills?: string[];
  channels?: string[];

  // Agent runtime configuration
  runtime?: AgentRuntime;
  runtimes?: AgentRuntime[];
  authMode?: 'hanzo' | 'custom' | 'none';
  apiKeyConfigured?: boolean;
}

export interface Starter {
  placeholder?: string;
}

export interface Team {
  teamId: string;
  presetId: string;
  name: string;
  emoji: string;
  botIds: string[];
}

// ---------------------------------------------------------------------------
// Canvas uses untyped Node at the flow level.
// Components narrow via node.type + cast.
// ---------------------------------------------------------------------------

export type CanvasNode = Node;

export interface Canvas {
  nodes: CanvasNode[];
  edges: Edge[];
  viewport: Viewport;
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

export const NODE_TYPES = {
  bot: 'bot',
  starter: 'starter',
  team: 'team',
} as const;
