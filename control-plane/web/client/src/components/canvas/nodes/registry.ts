/**
 * Node Registry
 *
 * Single source of truth for canvas node types and default dimensions.
 * Pass nodeTypes to ReactFlow's nodeTypes prop.
 */

import { BotNodeComponent } from './Bot';
import { StarterNodeComponent } from './Starter';
import { TeamNodeComponent } from './TeamGroup';

export const nodeTypes = {
  bot: BotNodeComponent,
  starter: StarterNodeComponent,
  team: TeamNodeComponent,
} as const;

// ---------------------------------------------------------------------------
// Default Node Dimensions
// ---------------------------------------------------------------------------

/** Minimum width for bot nodes (px) */
export const BOT_NODE_MIN_WIDTH = 400;
/** Minimum height for bot nodes (px) */
export const BOT_NODE_MIN_HEIGHT = 300;
/** Default width for bot nodes when first placed (px) */
export const BOT_NODE_DEFAULT_WIDTH = 440;
/** Default height for bot nodes when first placed (px) */
export const BOT_NODE_DEFAULT_HEIGHT = 360;

/** Horizontal gap between nodes in auto-layout grid (px) */
export const GRID_GAP_X = 480;
/** Vertical gap between nodes in auto-layout grid (px) */
export const GRID_GAP_Y = 400;
/** Number of columns in auto-layout grid */
export const GRID_COLUMNS = 2;
