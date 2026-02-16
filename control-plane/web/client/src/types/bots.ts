export interface BotWithNode {
  // Bot identification
  bot_id: string;   // Format: "node_id.bot_id"
  name: string;          // Human-readable name
  description: string;   // Bot description

  // Node context
  node_id: string;
  node_status: 'active' | 'inactive' | 'unknown';
  node_version: string;

  // Bot details
  input_schema: any;
  output_schema: any;
  memory_config: {
    auto_inject: string[];
    memory_retention: string;
    cache_results: boolean;
  };
  tags?: string[];

  // Performance metrics (optional)
  avg_response_time_ms?: number;
  success_rate?: number;
  total_runs?: number;
  last_executed?: string;

  // Timestamps
  last_updated: string;
}

export interface BotsResponse {
  bots: BotWithNode[];
  total: number;
  online_count: number;
  offline_count: number;
  nodes_count: number;
}

export interface BotFilters {
  status?: 'all' | 'online' | 'offline';
  search?: string;
  limit?: number;
  offset?: number;
}

export type BotStatus = 'online' | 'degraded' | 'offline' | 'unknown';

export interface BotCardProps {
  bot: BotWithNode;
  onClick?: (bot: BotWithNode) => void;
}

export interface BotGridProps {
  bots: BotWithNode[];
  loading?: boolean;
  onBotClick?: (bot: BotWithNode) => void;
  viewMode?: 'grid' | 'table';
}

export interface SearchFiltersProps {
  filters: BotFilters;
  onFiltersChange: (filters: BotFilters) => void;
  totalCount: number;
  onlineCount: number;
  offlineCount: number;
}
