/**
 * CanvasSidebar
 *
 * Agent list panel on the left of the canvas.
 * Search, filter by status, click to select on canvas.
 * Responsive: drawer on mobile, panel on desktop.
 */

import { useState, useMemo, useCallback } from 'react';
import { useBotStore } from '@/stores/botStore';
import { useCanvasStore } from '@/stores/canvasStore';
import type { BotStatus } from '@/types/gateway';
import { cn } from '@/lib/utils';

const STATUS_FILTERS: { key: BotStatus | 'all'; label: string }[] = [
  { key: 'all',    label: 'All' },
  { key: 'busy',   label: 'Working' },
  { key: 'idle',   label: 'Idle' },
  { key: 'error',  label: 'Error' },
  { key: 'offline', label: 'Offline' },
];

interface CanvasSidebarProps {
  open: boolean;
  onClose: () => void;
}

export function CanvasSidebar({ open, onClose }: CanvasSidebarProps) {
  const agentsMap = useBotStore((s) => s.agents);
  const agents = useMemo(() => Array.from(agentsMap.values()), [agentsMap]);
  const selectNode = useCanvasStore((s) => s.selectNode);
  const nodes = useCanvasStore((s) => s.nodes);
  const selectedNodeId = useCanvasStore((s) => s.selectedNodeId);

  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState<BotStatus | 'all'>('all');

  const filtered = useMemo(() => {
    return agents.filter((agent) => {
      if (statusFilter !== 'all' && agent.status !== statusFilter) return false;
      if (search && !agent.name.toLowerCase().includes(search.toLowerCase())) return false;
      return true;
    });
  }, [agents, statusFilter, search]);

  const handleSelect = useCallback(
    (agentId: string) => {
      const node = nodes.find(
        (n) => n.type === 'bot' && (n.data as Record<string, unknown>).agentId === agentId
      );
      if (node) selectNode(node.id);
    },
    [nodes, selectNode]
  );

  const selectedAgentId = useMemo(() => {
    if (!selectedNodeId) return null;
    const node = nodes.find((n) => n.id === selectedNodeId);
    if (!node || node.type !== 'bot') return null;
    return (node.data as Record<string, unknown>).agentId as string;
  }, [selectedNodeId, nodes]);

  return (
    <>
      {/* Backdrop on mobile */}
      {open && (
        <div
          className="fixed inset-0 z-30 bg-black/30 md:hidden"
          onClick={onClose}
        />
      )}

      {/* Panel */}
      <div
        className={cn(
          'fixed left-0 top-0 z-40 h-full w-72 border-r border-border/50 bg-card/95 backdrop-blur-sm shadow-xl transition-transform duration-200',
          'md:relative md:z-auto md:shadow-none md:border-r-0',
          open ? 'translate-x-0' : '-translate-x-full md:translate-x-0 md:w-0 md:overflow-hidden md:border-0',
        )}
      >
        <div className="flex h-full flex-col">
          {/* Header */}
          <div className="flex items-center justify-between px-3 py-3 border-b border-border/40">
            <span className="text-sm font-semibold">Bots</span>
            <span className="text-xs text-muted-foreground">{agents.length}</span>
          </div>

          {/* Search */}
          <div className="px-3 py-2">
            <input
              type="text"
              placeholder="Search bots..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full rounded-lg border border-border/50 bg-background/50 px-3 py-1.5 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary/50"
            />
          </div>

          {/* Status filters */}
          <div className="flex gap-1 px-3 pb-2 overflow-x-auto scrollbar-none">
            {STATUS_FILTERS.map((f) => (
              <button
                key={f.key}
                type="button"
                onClick={() => setStatusFilter(f.key)}
                className={cn(
                  'px-2 py-0.5 rounded-full text-[10px] whitespace-nowrap transition-colors shrink-0',
                  statusFilter === f.key
                    ? 'bg-primary text-primary-foreground'
                    : 'bg-accent/50 text-muted-foreground hover:text-foreground'
                )}
              >
                {f.label}
              </button>
            ))}
          </div>

          {/* Agent list */}
          <div className="flex-1 overflow-y-auto px-2 pb-2 space-y-0.5">
            {filtered.length === 0 ? (
              <div className="flex items-center justify-center h-20 text-xs text-muted-foreground">
                {agents.length === 0 ? 'No bots connected' : 'No matches'}
              </div>
            ) : (
              filtered.map((agent) => (
                <button
                  key={agent.id}
                  type="button"
                  onClick={() => handleSelect(agent.id)}
                  className={cn(
                    'flex items-center gap-2 w-full rounded-lg px-2.5 py-2 text-left transition-colors',
                    selectedAgentId === agent.id
                      ? 'bg-primary/10 text-foreground'
                      : 'hover:bg-accent/50 text-foreground/80'
                  )}
                >
                  {agent.avatar ? (
                    <img src={agent.avatar} alt={agent.name} className="h-5 w-5 rounded-full object-cover" />
                  ) : (
                    <span className="text-base">{agent.emoji ?? '🤖'}</span>
                  )}
                  <div className="flex-1 min-w-0">
                    <div className="text-xs font-medium truncate">{agent.name}</div>
                    <div className="text-[10px] text-muted-foreground capitalize">{agent.status}</div>
                  </div>
                  <StatusDot status={agent.status} />
                </button>
              ))
            )}
          </div>
        </div>
      </div>
    </>
  );
}

function StatusDot({ status }: { status: string }) {
  const color =
    status === 'busy' ? 'bg-green-500' :
    status === 'error' ? 'bg-red-500' :
    status === 'offline' ? 'bg-gray-400' :
    status === 'waiting' ? 'bg-yellow-500' :
    'bg-blue-500';

  return <span className={cn('h-1.5 w-1.5 rounded-full shrink-0', color)} />;
}
