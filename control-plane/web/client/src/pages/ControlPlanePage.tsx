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

type ConnectivityState = 'connected' | 'heartbeat-delayed' | 'degraded' | 'unreachable' | 'updating';

interface AgentGroup {
  nodeId: string;
  nodeName: string;
  nodeType: 'local' | 'cloud';
  location: string;
  status: ConnectivityState;
  bots: BotWithNode[];
  metrics: {
    cpu: number;
    memory: number;
    latency: number;
  };
}

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

function groupBotsByAgent(bots: BotWithNode[]): AgentGroup[] {
  const groups = new Map<string, AgentGroup>();

  for (const bot of bots) {
    const nodeId = bot.node_id || 'unknown';
    if (!groups.has(nodeId)) {
      // Determine node type from bot_id pattern or node_id
      const isLocal = nodeId.includes('local') || nodeId.startsWith('dev-');
      groups.set(nodeId, {
        nodeId,
        nodeName: nodeId,
        nodeType: isLocal ? 'local' : 'cloud',
        location: isLocal ? 'Local' : 'Cloud',
        status: bot.node_status === 'active' ? 'connected' : bot.node_status === 'inactive' ? 'unreachable' : 'degraded',
        bots: [],
        metrics: { cpu: 0, memory: 0, latency: 0 },
      });
    }
    groups.get(nodeId)!.bots.push(bot);
  }

  return Array.from(groups.values());
}

function connectivityLabel(state: ConnectivityState): { text: string; color: string } {
  switch (state) {
    case 'connected':         return { text: 'Connected',         color: 'text-green-400' };
    case 'heartbeat-delayed': return { text: 'Heartbeat Delayed', color: 'text-yellow-400' };
    case 'degraded':          return { text: 'Degraded',          color: 'text-orange-400' };
    case 'unreachable':       return { text: 'Unreachable',       color: 'text-red-400' };
    case 'updating':          return { text: 'Updating',          color: 'text-blue-400' };
  }
}

function statusDotColor(state: ConnectivityState): string {
  switch (state) {
    case 'connected':         return 'bg-green-400';
    case 'heartbeat-delayed': return 'bg-yellow-400';
    case 'degraded':          return 'bg-orange-400';
    case 'unreachable':       return 'bg-red-400';
    case 'updating':          return 'bg-blue-400';
  }
}

function botStatusColor(nodeStatus: string): string {
  switch (nodeStatus) {
    case 'active':   return 'text-green-400';
    case 'inactive': return 'text-muted-foreground';
    case 'unknown':  return 'text-yellow-400';
    default:         return 'text-muted-foreground';
  }
}

function botStatusLabel(nodeStatus: string): string {
  switch (nodeStatus) {
    case 'active':   return 'Running';
    case 'inactive': return 'Offline';
    case 'unknown':  return 'Unknown';
    default:         return nodeStatus;
  }
}

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
  const [expandedAgents, setExpandedAgents] = useState<Set<string>>(new Set());
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
              addActivity('agent', `Bot came online: ${event.data?.bot_id ?? 'unknown'}`, 'success');
              break;
            case 'bot_offline':
              addActivity('agent', `Bot went offline: ${event.data?.bot_id ?? 'unknown'}`, 'warn');
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
  const agents = groupBotsByAgent(safeData.bots);
  const cloudCount = agents.filter((a) => a.nodeType === 'cloud').length;
  const localCount = agents.filter((a) => a.nodeType === 'local').length;
  const healthyCount = agents.filter((a) => a.status === 'connected').length;

  const toggleAgent = useCallback((nodeId: string) => {
    setExpandedAgents((prev) => {
      const next = new Set(prev);
      if (next.has(nodeId)) next.delete(nodeId);
      else next.add(nodeId);
      return next;
    });
  }, []);

  // -----------------------------------------------------------------------
  // Render
  // -----------------------------------------------------------------------

  return (
    <div className="space-y-0">
      {/* ─── Top Bar ─── */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-4">
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

          {/* Register Agent */}
          <button
            type="button"
            onClick={() => navigate('/nodes')}
            className="h-7 px-3 flex items-center gap-1.5 rounded-md bg-primary text-primary-foreground text-xs font-medium hover:bg-primary/90 transition-colors"
          >
            <span>+</span>
            <span>Register Agent</span>
          </button>
        </div>
      </div>

      {/* ─── Metrics Strip ─── */}
      <div className="flex flex-wrap items-center gap-2 sm:gap-4 py-2 px-0 mb-4 border-y border-border/30 text-xs font-mono text-muted-foreground">
        <MetricCell label="Bots" value={agents.length} />
        <Sep />
        <MetricCell label="Cloud" value={cloudCount} />
        <Sep />
        <MetricCell label="Local" value={localCount} />
        <Sep />
        <MetricCell label="Bots" value={safeData.total} />
        <Sep />
        <MetricCell label="Healthy" value={agents.length > 0 ? `${Math.round((healthyCount / agents.length) * 100)}%` : '\u2014'} />
      </div>

      {/* ─── Error ─── */}
      {error && (
        <div className="mb-4 px-3 py-2 border border-red-500/30 bg-red-500/5 rounded-md text-xs text-red-400">
          {error}
        </div>
      )}

      {/* ─── Main 60/40 Split ─── */}
      <div className="flex flex-col lg:flex-row gap-4 min-h-[400px]">
        {/* LEFT: Agent Network State (60%) */}
        <div className="flex-[3] min-w-0">
          <div className="text-xs font-medium text-muted-foreground mb-3 uppercase tracking-wider">
            Agent Network
          </div>

          {loading && !data ? (
            <div className="flex items-center gap-2 text-xs text-muted-foreground py-8">
              <span className="h-3 w-3 animate-spin rounded-full border border-muted-foreground border-t-transparent" />
              Connecting to agent network...
            </div>
          ) : agents.length === 0 ? (
            <EmptyNetwork onRegisterLocal={() => navigate('/nodes')} onDeployCloud={() => navigate('/nodes')} />
          ) : (
            <div className="space-y-1">
              {agents.map((agent) => (
                <AgentCard
                  key={agent.nodeId}
                  agent={agent}
                  expanded={expandedAgents.has(agent.nodeId)}
                  onToggle={() => toggleAgent(agent.nodeId)}
                  onBotClick={(bot) => navigate(`/bots/${encodeURIComponent(bot.bot_id)}`)}
                />
              ))}
            </div>
          )}
        </div>

        {/* RIGHT: Activity Stream (40%) */}
        <div className="flex-[2] min-w-0 border-t lg:border-t-0 lg:border-l border-border/30 pt-4 lg:pt-0 lg:pl-4">
          <div className="text-xs font-medium text-muted-foreground mb-3 uppercase tracking-wider">
            Activity
          </div>
          <ActivityStream events={activity} />
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function MetricCell({ label, value }: { label: string; value: number | string }) {
  return (
    <div className="flex items-baseline gap-1.5">
      <span className="text-foreground font-medium">{typeof value === 'number' ? value : value}</span>
      <span>{label}</span>
    </div>
  );
}

function Sep() {
  return <span className="text-border/50">|</span>;
}

function EmptyNetwork({ onRegisterLocal, onDeployCloud }: { onRegisterLocal: () => void; onDeployCloud: () => void }) {
  return (
    <div className="py-12">
      <p className="text-sm text-muted-foreground mb-6">No agents connected.</p>
      <div className="flex gap-3">
        <button
          type="button"
          onClick={onRegisterLocal}
          className="h-8 px-4 flex items-center gap-2 rounded-md border border-border/50 text-xs font-medium text-foreground hover:bg-accent transition-colors"
        >
          <span className="text-green-400">+</span>
          Register Local Agent
        </button>
        <button
          type="button"
          onClick={onDeployCloud}
          className="h-8 px-4 flex items-center gap-2 rounded-md border border-border/50 text-xs font-medium text-foreground hover:bg-accent transition-colors"
        >
          <span className="text-blue-400">+</span>
          Deploy Cloud Agent
        </button>
      </div>
    </div>
  );
}

function AgentCard({
  agent,
  expanded,
  onToggle,
  onBotClick,
}: {
  agent: AgentGroup;
  expanded: boolean;
  onToggle: () => void;
  onBotClick: (bot: BotWithNode) => void;
}) {
  const conn = connectivityLabel(agent.status);
  const dotColor = statusDotColor(agent.status);

  return (
    <div className="border border-border/30 rounded-md overflow-hidden">
      {/* Agent header */}
      <button
        type="button"
        onClick={onToggle}
        className="w-full flex items-center gap-3 px-3 py-2 text-left hover:bg-accent/30 transition-colors"
      >
        {/* Expand chevron */}
        <svg
          width="10"
          height="10"
          viewBox="0 0 10 10"
          fill="none"
          className={cn('shrink-0 transition-transform text-muted-foreground', expanded && 'rotate-90')}
        >
          <path d="M3 1l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>

        {/* Status dot */}
        <span className={cn('h-2 w-2 rounded-full shrink-0', dotColor)} />

        {/* Name */}
        <span className="text-sm font-medium truncate">{agent.nodeName}</span>

        {/* Location badge */}
        <span className={cn(
          'text-[10px] px-1.5 py-0.5 rounded font-mono shrink-0',
          agent.nodeType === 'local'
            ? 'text-green-400 bg-green-500/10'
            : 'text-blue-400 bg-blue-500/10'
        )}>
          {agent.location}
        </span>

        {/* Status */}
        <span className={cn('text-[10px] font-mono ml-auto shrink-0', conn.color)}>
          {conn.text}
        </span>

        {/* Bot count */}
        <span className="text-[10px] text-muted-foreground font-mono shrink-0">
          {agent.bots.length} {agent.bots.length === 1 ? 'bot' : 'bots'}
        </span>
      </button>

      {/* Expanded: bot list */}
      {expanded && (
        <div className="border-t border-border/20">
          {agent.bots.map((bot) => (
            <button
              key={bot.bot_id}
              type="button"
              onClick={() => onBotClick(bot)}
              className="w-full flex items-center gap-3 px-3 py-1.5 pl-9 text-left hover:bg-accent/20 transition-colors text-xs"
            >
              <span className={cn('font-mono', botStatusColor(bot.node_status))}>
                {botStatusLabel(bot.node_status)}
              </span>
              <span className="text-foreground truncate">{bot.name || bot.bot_id}</span>
              {bot.node_version && (
                <span className="text-muted-foreground font-mono text-[10px] ml-auto truncate max-w-[120px]">
                  {bot.node_version}
                </span>
              )}
            </button>
          ))}
        </div>
      )}
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
