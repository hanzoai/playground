/**
 * Canvas Types
 */

import type { Node, Edge, Viewport } from '@xyflow/react';
import type { BotStatus } from './gateway';

// ---------------------------------------------------------------------------
// Data Interfaces
// ---------------------------------------------------------------------------

export type BotView = 'overview' | 'terminal' | 'chat' | 'operative' | 'files';

export interface Bot {
  agentId: string;
  name: string;
  emoji?: string;
  avatar?: string;
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
