import type { KeyboardEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { cn } from '@/lib/utils';
import type { BotCardProps } from '../../types/bots';
import { StatusIndicator } from './StatusIndicator';
import { CompositeDIDStatus } from '../did/DIDStatusBadge';
import { useDIDStatus } from '../../hooks/useDIDInfo';
import { Bot, Layers, Timer, Tag, Flash, CheckCircle, BarChart3, Identification } from '@/components/ui/icon-bridge';
import { Card } from '@/components/ui/card';

export function BotCard({ bot, onClick }: BotCardProps) {
  const navigate = useNavigate();

  // Get DID status for the bot
  const { status: didStatus } = useDIDStatus(bot.bot_id);

  const handleClick = () => {
    if (onClick) {
      onClick(bot);
    } else {
      // Navigate to bot detail page
      // bot_id already contains the full format: "node_id.bot_name"
      navigate(`/bots/${encodeURIComponent(bot.bot_id)}`);
    }
  };

  const getStatusFromNodeStatus = (nodeStatus: string) => {
    switch (nodeStatus) {
      case 'active':
        return 'online';
      case 'inactive':
        return 'offline';
      default:
        return 'unknown';
    }
  };

  const status = getStatusFromNodeStatus(bot.node_status);
  const isOffline = status === 'offline';

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      handleClick();
    }
  };

  const formatTimeAgo = (dateString: string) => {
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / (1000 * 60));

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins}m ago`;

    const diffHours = Math.floor(diffMins / 60);
    if (diffHours < 24) return `${diffHours}h ago`;

    const diffDays = Math.floor(diffHours / 24);
    return `${diffDays}d ago`;
  };

  return (
    <Card
      variant="default"
      interactive={true}
      role="button"
      tabIndex={0}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      aria-label={`View bot ${bot.name}`}
      className={cn(
        "group flex h-full flex-col gap-4 p-4 transition-transform duration-200 cursor-pointer",
        "hover:-translate-y-[1px]",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
        isOffline && "opacity-75"
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 flex-1 items-start gap-3">
          <div className="mt-0.5 flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
            <Bot className="h-4 w-4" weight="fill" />
          </div>
          <div className="min-w-0 space-y-1">
            <h3
              className="line-clamp-2 text-sm font-medium leading-tight text-foreground transition-colors group-hover:text-foreground"
              title={bot.name}
            >
              {bot.name}
            </h3>
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-body-small">
              <span className="truncate">{bot.node_id}</span>
              <span className="text-muted-foreground/50">•</span>
              <span className="whitespace-nowrap">
                Updated {formatTimeAgo(bot.last_updated)}
              </span>
            </div>
          </div>
        </div>
        <div className="mt-0.5 flex flex-shrink-0 items-center gap-2">
          <StatusIndicator status={status} size="sm" />
          {didStatus && didStatus.has_did && (
            <div className="flex items-center gap-1 text-body-small">
              <Identification className="h-3 w-3" weight="bold" />
              <CompositeDIDStatus
                status={didStatus.did_status}
                botCount={didStatus.bot_count}
                skillCount={didStatus.skill_count}
                compact={true}
                className="text-xs"
              />
            </div>
          )}
        </div>
      </div>

      <p className="min-h-[2.5rem] text-xs leading-relaxed text-muted-foreground line-clamp-3">
        {bot.description}
      </p>

      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-body-small">
        <div className="flex items-center gap-1.5">
          <Layers
            className="h-3 w-3 flex-shrink-0"
            weight={bot.memory_config?.cache_results ? "fill" : "regular"}
          />
          <span className="whitespace-nowrap">
            {bot.memory_config?.cache_results ? "Cached" : "No cache"}
          </span>
        </div>
        {bot.memory_config?.memory_retention && (
          <div className="flex items-center gap-1.5">
            <Timer className="h-3 w-3 flex-shrink-0" weight="regular" />
            <span className="whitespace-nowrap">
              {bot.memory_config.memory_retention}
            </span>
          </div>
        )}
        <div className="flex items-center gap-1.5">
          <Tag className="h-3 w-3 flex-shrink-0" weight="regular" />
          <span className="whitespace-nowrap">v{bot.node_version}</span>
        </div>
      </div>

      {(bot.avg_response_time_ms ||
        bot.success_rate ||
        bot.total_runs) && (
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-body-small">
          {bot.avg_response_time_ms && (
            <div className="flex items-center gap-1">
              <Flash className="h-3 w-3 flex-shrink-0" weight="bold" />
              <span className="whitespace-nowrap">
                {bot.avg_response_time_ms}ms avg
              </span>
            </div>
          )}
          {bot.success_rate && (
            <div className="flex items-center gap-1">
              <CheckCircle
                className="h-3 w-3 flex-shrink-0 text-status-success"
                weight="fill"
              />
              <span className="whitespace-nowrap">
                {(bot.success_rate * 100).toFixed(1)}% success
              </span>
            </div>
          )}
          {bot.total_runs && (
            <div className="flex items-center gap-1">
              <BarChart3 className="h-3 w-3 flex-shrink-0" weight="bold" />
              <span className="whitespace-nowrap">
                {bot.total_runs.toLocaleString()} runs
              </span>
            </div>
          )}
        </div>
      )}
    </Card>
  );
}
