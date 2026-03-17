/**
 * Control Plane
 *
 * Distributed agent orchestration interface.
 * Shows agent network state, activity stream, and system topology.
 * Dense, infrastructure-style — not a SaaS CRUD dashboard.
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { botsApi, BotsApiError } from '@/services/botsApi';
import { useNodeEventsSSE, useUnifiedStatusSSE } from '@/hooks/useSSE';
import { cn } from '@/lib/utils';
import type { BotsResponse, BotWithNode } from '@/types/bots';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface ActivityEvent {
  id: string;
  ts: Date;
  type: string;
  message: string;
  level: 'info' | 'warn' | 'error' | 'success';
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function relativeTime(ts: Date): string {
  const ms = Date.now() - ts.getTime();
  if (ms < 60_000) return 'now';
  if (ms < 3_600_000) return `${Math.floor(ms / 60_000)}m ago`;
  if (ms < 86_400_000) return `${Math.floor(ms / 3_600_000)}h ago`;
  return `${Math.floor(ms / 86_400_000)}d ago`;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function ControlPlanePage() {
  const navigate = useNavigate();
  const [data, setData] = useState<BotsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [sseConnected, setSseConnected] = useState(false);
  const [activity, setActivity] = useState<ActivityEvent[]>([]);
  const [showActivity, setShowActivity] = useState(false);
  const eventSourceRef = useRef<EventSource | null>(null);
  const activityIdRef = useRef(0);

  const nodeEventsSSE = useNodeEventsSSE();
  const unifiedStatusSSE = useUnifiedStatusSSE();

  // -----------------------------------------------------------------------
  // Data fetch
  // -----------------------------------------------------------------------

  const fetchBots = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await botsApi.getAllBots({ status: 'all', limit: 200, offset: 0 });
      setData(response);
    } catch (err) {
      if (err instanceof BotsApiError && (err.status === 404 || err.message?.includes('no bots found'))) {
        setData({ bots: [], total: 0, online_count: 0, offline_count: 0, nodes_count: 0 });
      } else {
        setError(err instanceof BotsApiError ? err.message : 'Failed to connect to agent network');
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchBots(); }, [fetchBots]);

  // -----------------------------------------------------------------------
  // Activity log
  // -----------------------------------------------------------------------

  const addActivity = useCallback((type: string, message: string, level: ActivityEvent['level'] = 'info') => {
    setActivity((prev) => [
      { id: String(++activityIdRef.current), ts: new Date(), type, message, level },
      ...prev,
    ].slice(0, 50));
  }, []);

  // -----------------------------------------------------------------------
  // SSE
  // -----------------------------------------------------------------------

  useEffect(() => {
    try {
      const eventSource = botsApi.createEventStream(
        (event) => {
          switch (event.type) {
            case 'connected':
              setSseConnected(true);
              addActivity('system', 'Live connection established', 'success');
              break;
            case 'heartbeat':
              break;
            case 'bot_online':
              addActivity('bot', `Bot came online: ${event.data?.bot_id ?? 'unknown'}`, 'success');
              break;
            case 'bot_offline':
              addActivity('bot', `Bot went offline: ${event.data?.bot_id ?? 'unknown'}`, 'warn');
              break;
            case 'bot_status_changed':
            case 'node_status_changed':
              addActivity('status', `Status changed: ${event.data?.node_id ?? event.data?.bot_id ?? 'unknown'}`, 'info');
              break;
            default:
              if (event.type !== 'heartbeat') {
                addActivity('event', `${event.type}`, 'info');
              }
          }
        },
        () => {
          setSseConnected(false);
          addActivity('system', 'Live connection lost', 'error');
        },
        () => {
          setSseConnected(true);
        }
      );
      eventSourceRef.current = eventSource;
    } catch {
      setSseConnected(false);
    }

    return () => {
      if (eventSourceRef.current) {
        botsApi.closeEventStream(eventSourceRef.current);
        eventSourceRef.current = null;
      }
    };
  }, [addActivity]);

  // Handle node events
  useEffect(() => {
    const event = unifiedStatusSSE.latestEvent || nodeEventsSSE.latestEvent;
    if (!event) return;
    addActivity('node', `${event.type}: ${event.data?.node_id ?? ''}`, 'info');
  }, [unifiedStatusSSE.latestEvent, nodeEventsSSE.latestEvent, addActivity]);

  // -----------------------------------------------------------------------
  // Derived state
  // -----------------------------------------------------------------------

  const safeData = data ?? { bots: [], total: 0, online_count: 0, offline_count: 0, nodes_count: 0 };

  // -----------------------------------------------------------------------
  // Render
  // -----------------------------------------------------------------------

  return (
    <div className="space-y-0">
      {/* ─── Top Bar ─── */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-6">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">Control Plane</h1>
          <p className="text-xs text-muted-foreground mt-0.5">
            Orchestrate local and cloud agents from a unified control surface.
          </p>
        </div>
        <div className="flex items-center gap-3">
          {/* Live indicator */}
          <div className="flex items-center gap-1.5 text-xs">
            <span className={cn(
              'h-1.5 w-1.5 rounded-full',
              sseConnected ? 'bg-green-400' : 'bg-red-400'
            )} />
            <span className={cn(
              'font-mono',
              sseConnected ? 'text-green-400' : 'text-red-400'
            )}>
              {sseConnected ? 'Live' : 'Offline'}
            </span>
          </div>

          {/* Refresh */}
          <button
            type="button"
            onClick={fetchBots}
            disabled={loading}
            className="h-7 w-7 flex items-center justify-center rounded-md border border-border/50 text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
            title="Refresh"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className={loading ? 'animate-spin' : ''}>
              <path d="M21 2v6h-6" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M3 22v-6h6" /><path d="M21 12a9 9 0 0 1-15 6.7L3 16" />
            </svg>
          </button>

          {/* Activity toggle */}
          <button
            type="button"
            onClick={() => setShowActivity((v) => !v)}
            className={cn(
              'h-7 px-2.5 flex items-center gap-1.5 rounded-md border border-border/50 text-xs font-medium transition-colors',
              showActivity ? 'bg-accent text-foreground' : 'text-muted-foreground hover:text-foreground hover:bg-accent'
            )}
          >
            Activity
            {activity.length > 0 && (
              <span className="h-4 min-w-[16px] px-1 flex items-center justify-center rounded-full bg-muted text-[10px] font-mono">
                {activity.length}
              </span>
            )}
          </button>
        </div>
      </div>

      {/* ─── Error ─── */}
      {error && (
        <div className="mb-4 px-3 py-2 border border-red-500/30 bg-red-500/5 rounded-md text-xs text-red-400">
          {error}
        </div>
      )}

      {/* ─── Bot Grid (full width) ─── */}
      {loading && !data ? (
        <div className="flex items-center gap-2 text-xs text-muted-foreground py-12">
          <span className="h-3 w-3 animate-spin rounded-full border border-muted-foreground border-t-transparent" />
          Connecting to bot network...
        </div>
      ) : safeData.bots.length === 0 ? (
        <EmptyNetwork onCreate={() => navigate('/playground')} />
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {safeData.bots.map((bot) => (
            <BotCard
              key={bot.bot_id}
              bot={bot}
              onOpen={() => navigate(`/playground?bot=${encodeURIComponent(bot.bot_id)}`)}
              onDetail={() => navigate(`/bots/${encodeURIComponent(bot.bot_id)}`)}
            />
          ))}
        </div>
      )}

      {/* ─── Collapsible Activity Stream ─── */}
      {showActivity && (
        <div className="mt-6 border-t border-border/30 pt-4">
          <div className="text-xs font-medium text-muted-foreground mb-3 uppercase tracking-wider">
            Activity
          </div>
          <ActivityStream events={activity} />
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function EmptyNetwork({ onCreate }: { onCreate: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center py-20">
      <div className="h-12 w-12 rounded-full bg-muted flex items-center justify-center mb-4">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="text-muted-foreground">
          <circle cx="12" cy="12" r="10" />
          <path d="M12 8v8" /><path d="M8 12h8" />
        </svg>
      </div>
      <p className="text-sm font-medium text-foreground mb-1">No bots yet</p>
      <p className="text-xs text-muted-foreground mb-6">Create your first bot to get started.</p>
      <button
        type="button"
        onClick={onCreate}
        className="h-8 px-4 flex items-center gap-2 rounded-md bg-primary text-primary-foreground text-xs font-medium hover:bg-primary/90 transition-colors"
      >
        Create your first bot
      </button>
    </div>
  );
}

function BotCard({
  bot,
  onOpen,
  onDetail,
}: {
  bot: BotWithNode;
  onOpen: () => void;
  onDetail: () => void;
}) {
  const dotColor =
    bot.node_status === 'active' ? 'bg-green-400' :
    bot.node_status === 'inactive' ? 'bg-red-400' :
    'bg-yellow-400';

  const lastActive = bot.last_executed
    ? relativeTime(new Date(bot.last_executed))
    : bot.last_updated
    ? relativeTime(new Date(bot.last_updated))
    : null;

  return (
    <div
      className="border border-border/30 rounded-lg p-4 hover:border-border/60 transition-colors cursor-pointer"
      onClick={onDetail}
    >
      {/* Name + status */}
      <div className="flex items-center gap-2 mb-2">
        <span className={cn('h-2 w-2 rounded-full shrink-0', dotColor)} />
        <span className="text-sm font-medium truncate">{bot.name || bot.bot_id}</span>
      </div>

      {/* Last active */}
      {lastActive && (
        <p className="text-xs text-muted-foreground mb-3">
          Active {lastActive}
        </p>
      )}

      {/* Open in Playground */}
      <button
        type="button"
        onClick={(e) => { e.stopPropagation(); onOpen(); }}
        className="h-7 px-3 w-full flex items-center justify-center gap-1.5 rounded-md border border-border/50 text-xs font-medium text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
      >
        Open in Playground
      </button>
    </div>
  );
}

function ActivityStream({ events }: { events: ActivityEvent[] }) {
  if (events.length === 0) {
    return (
      <div className="text-xs text-muted-foreground/60 font-mono py-4">
        Awaiting agent connections...
      </div>
    );
  }

  return (
    <div className="space-y-0 max-h-[500px] overflow-y-auto">
      {events.map((event) => (
        <div
          key={event.id}
          className="flex items-start gap-2 py-1.5 border-b border-border/10 text-xs"
        >
          <span className={cn(
            'h-1 w-1 rounded-full mt-1.5 shrink-0',
            event.level === 'success' ? 'bg-green-400' :
            event.level === 'warn' ? 'bg-yellow-400' :
            event.level === 'error' ? 'bg-red-400' :
            'bg-muted-foreground/40'
          )} />
          <span className="text-muted-foreground font-mono shrink-0 w-10">
            {relativeTime(event.ts)}
          </span>
          <span className="text-foreground/80 truncate">{event.message}</span>
        </div>
      ))}
    </div>
  );
}
