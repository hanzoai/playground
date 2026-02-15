/**
 * Node Registry
 *
 * Single source of truth for canvas node types.
 * Pass to ReactFlow's nodeTypes prop.
 */

import { BotNodeComponent } from './Bot';
import { StarterNodeComponent } from './Starter';
import { TeamNodeComponent } from './TeamGroup';

export const nodeTypes = {
  bot: BotNodeComponent,
  starter: StarterNodeComponent,
  team: TeamNodeComponent,
} as const;
