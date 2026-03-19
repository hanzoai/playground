/**
 * Bot Node
 *
 * Primary canvas node representing an agent bot.
 * Large card showing name, status, and live terminal/desktop/chat preview.
 * Always shows content (no collapsed tiny state).
 * Resizable via corner handles.
 * Handles hidden by default, shown on hover for connecting.
 */

import { Handle, Position, NodeResizer, type NodeProps } from '@xyflow/react';
import { useCallback, useState, lazy, Suspense } from 'react';
import { useCanvasStore } from '@/stores/canvasStore';
import type { Bot as BotType, BotView } from '@/types/canvas';
import { cn } from '@/lib/utils';
import { BOT_NODE_MIN_WIDTH, BOT_NODE_MIN_HEIGHT } from './registry';

// Lazy load drill-down panels for code splitting
const TerminalPanel = lazy(() => import('../drill-down/TerminalPanel').then(m => ({ default: m.TerminalPanel })));
const ChatPanel = lazy(() => import('../drill-down/ChatPanel').then(m => ({ default: m.ChatPanel })));
const OperativePanel = lazy(() => import('../drill-down/OperativePanel').then(m => ({ default: m.OperativePanel })));
const FileViewer = lazy(() => import('../drill-down/FileViewer').then(m => ({ default: m.FileViewer })));

// ---------------------------------------------------------------------------
// Status Config
// ---------------------------------------------------------------------------

const STATUS: Record<string, { color: string; ring: string; label: string; pulse?: boolean }> = {
  idle:         { color: 'bg-blue-500',    ring: 'ring-blue-500/30',    label: 'Idle' },
  busy:         { color: 'bg-emerald-400', ring: 'ring-emerald-400/30', label: 'Working', pulse: true },
  waiting:      { color: 'bg-amber-400',   ring: 'ring-amber-400/30',  label: 'Waiting' },
  error:        { color: 'bg-red-500',     ring: 'ring-red-500/30',    label: 'Error' },
  offline:      { color: 'bg-zinc-500',    ring: 'ring-zinc-500/20',   label: 'Offline' },
  provisioning: { color: 'bg-purple-500',  ring: 'ring-purple-500/30', label: 'Provisioning', pulse: true },
};

const VIEW_TABS: { key: BotView; label: string; icon: string }[] = [
  { key: 'overview',  label: 'Overview',  icon: '📊' },
  { key: 'terminal',  label: 'Terminal',  icon: '⌨️' },
  { key: 'chat',      label: 'Chat',      icon: '💬' },
  { key: 'operative', label: 'Desktop',   icon: '🖥️' },
  { key: 'files',     label: 'Files',     icon: '📁' },
];

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function BotNodeComponent({ data, selected }: NodeProps) {
  const bot = data as unknown as BotType;
  const setBotView = useCanvasStore((s) => s.setBotView);
  const removeBot = useCanvasStore((s) => s.removeBot);
  const [collapsed, setCollapsed] = useState(false);
  const status = STATUS[bot.status] ?? STATUS.idle;

  const handleToggleCollapse = useCallback(() => {
    setCollapsed((prev) => !prev);
  }, []);

  const handleViewChange = useCallback(
    (view: BotView) => {
      setBotView(bot.agentId, view);
    },
    [setBotView, bot.agentId]
  );

  const handleClose = useCallback(() => {
    removeBot(bot.agentId);
  }, [removeBot, bot.agentId]);

  return (
    <div
      className={cn(
        'group relative rounded-2xl bg-zinc-900/95 transition-all duration-200 touch-manipulation',
        'w-full h-full flex flex-col',
        selected
          ? 'ring-2 ring-primary/50 shadow-[0_0_32px_-4px] shadow-primary/20'
          : 'ring-1 ring-white/[0.08] hover:ring-white/[0.16] hover:shadow-[0_8px_32px_-8px] hover:shadow-black/40',
        'shadow-2xl',
      )}
      style={{
        borderRadius: '16px',
        ...(bot.color && selected ? { boxShadow: `0 0 32px -4px ${bot.color}30, 0 0 64px -8px ${bot.color}15` } : {}),
        ...(bot.color && !selected && (bot.status === 'busy') ? { boxShadow: `0 0 20px -6px ${bot.color}25` } : {}),
        ...(bot.color ? { borderColor: selected ? `${bot.color}50` : undefined } : {}),
      }}
    >
      {/* Resize handles */}
      <NodeResizer
        minWidth={BOT_NODE_MIN_WIDTH}
        minHeight={collapsed ? 52 : BOT_NODE_MIN_HEIGHT}
        isVisible={selected ?? false}
        lineClassName="!border-primary/20"
        handleClassName="!w-2 !h-2 !bg-primary/50 !border-[1.5px] !border-zinc-900 !rounded-full"
      />

      {/* Connection handles - hidden by default, shown on hover */}
      <Handle
        type="target"
        position={Position.Top}
        className="!w-2.5 !h-2.5 !bg-primary/50 !border-[1.5px] !border-zinc-900 !rounded-full !opacity-0 group-hover:!opacity-100 !transition-all !duration-200"
      />
      <Handle
        type="source"
        position={Position.Bottom}
        className="!w-2.5 !h-2.5 !bg-primary/50 !border-[1.5px] !border-zinc-900 !rounded-full !opacity-0 group-hover:!opacity-100 !transition-all !duration-200"
      />

      {/* Header - always visible, prominent */}
      <div
        className="flex items-center gap-2.5 px-4 py-3 cursor-pointer select-none shrink-0"
        onClick={handleToggleCollapse}
      >
        {/* Status indicator */}
        <span className={cn(
          'relative h-2.5 w-2.5 rounded-full shrink-0',
          status.color,
          status.pulse && 'animate-pulse',
          'ring-2',
          status.ring,
        )}>
          {status.pulse && (
            <span className={cn(
              'absolute inset-0 rounded-full animate-ping',
              status.color,
              'opacity-40',
            )} />
          )}
        </span>

        {/* Avatar / Emoji */}
        {bot.avatar ? (
          <img
            src={bot.avatar}
            alt={bot.name}
            className="h-7 w-7 rounded-full object-cover shrink-0"
            style={{ outline: `2px solid ${bot.color ?? 'transparent'}`, outlineOffset: '1px' }}
            onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; (e.target as HTMLImageElement).nextElementSibling?.classList.remove('hidden'); }}
          />
        ) : null}
        {!bot.avatar ? (
          <span className="text-lg leading-none shrink-0">{bot.emoji ?? '\u{1F916}'}</span>
        ) : (
          <span className="text-lg leading-none shrink-0 hidden">{bot.emoji ?? '\u{1F916}'}</span>
        )}

        {/* Name + Role + Status */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-1.5">
            <span className="text-base font-semibold truncate">{bot.name}</span>
          </div>
          <div className="text-xs text-muted-foreground leading-tight flex items-center gap-1.5">
            {bot.role && (
              <>
                <span style={{ color: bot.color }}>{bot.role}</span>
                <span className="text-muted-foreground/40">&middot;</span>
              </>
            )}
            <span>{status.label}</span>
          </div>
        </div>

        {/* Source badge */}
        <span className={cn(
          'text-[10px] px-2 py-0.5 rounded-full shrink-0 font-medium',
          bot.source === 'local'
            ? 'text-emerald-400 bg-emerald-500/10'
            : 'text-blue-400 bg-blue-500/10'
        )}>
          {bot.source === 'local' ? 'local' : 'cloud'}
        </span>

        {/* Model tag */}
        {bot.model && (
          <span className="text-[10px] px-2 py-0.5 rounded-full bg-muted/50 text-muted-foreground shrink-0">
            {bot.model}
          </span>
        )}
      </div>

      {/* Content area (collapsible) */}
      {!collapsed && (
        <div className="flex-1 flex flex-col border-t border-white/[0.06] min-h-0 overflow-hidden transition-all duration-200">
          {/* Tab bar — window-style header */}
          <div className="flex items-center bg-zinc-800/50 shrink-0">
            <div className="flex flex-1 overflow-x-auto scrollbar-none">
              {VIEW_TABS.map((tab) => (
                <button
                  key={tab.key}
                  type="button"
                  onClick={() => handleViewChange(tab.key)}
                  className={cn(
                    'flex items-center gap-1.5 px-3 py-2 text-[11px] whitespace-nowrap transition-all duration-150 shrink-0 relative',
                    bot.activeView === tab.key
                      ? 'text-foreground bg-zinc-900/80'
                      : 'text-muted-foreground/70 hover:text-muted-foreground hover:bg-zinc-800/60'
                  )}
                >
                  <span className="text-xs">{tab.icon}</span>
                  <span className="font-medium">{tab.label}</span>
                  {bot.activeView === tab.key && (
                    <div className="absolute bottom-0 left-2 right-2 h-[2px] bg-primary rounded-full" />
                  )}
                </button>
              ))}
            </div>

            {/* Window controls */}
            <div className="flex items-center gap-1 px-2 shrink-0">
              {/* Collapse */}
              <button
                type="button"
                onClick={handleToggleCollapse}
                title="Collapse"
                className="flex h-5 w-5 items-center justify-center rounded-full text-muted-foreground/50 hover:text-foreground hover:bg-white/[0.08] transition-all duration-150"
              >
                <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
                  <path d="M2 4l3 3 3-3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </button>
              {/* Close */}
              <button
                type="button"
                onClick={handleClose}
                title="Remove from canvas"
                className="flex h-5 w-5 items-center justify-center rounded-full text-muted-foreground/50 hover:text-red-400 hover:bg-red-500/10 transition-all duration-150"
              >
                <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
                  <path d="M2.5 2.5l5 5M7.5 2.5l-5 5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                </svg>
              </button>
            </div>
          </div>

          {/* Drill-down content - fills remaining space */}
          <div className="flex-1 min-h-0 overflow-hidden">
            <Suspense fallback={<Loading />}>
              {bot.activeView === 'overview' && <BotOverview bot={bot} />}
              {bot.activeView === 'terminal' && (
                <TerminalPanel agentId={bot.agentId} sessionKey={bot.sessionKey} />
              )}
              {bot.activeView === 'chat' && (
                <ChatPanel agentId={bot.agentId} sessionKey={bot.sessionKey} />
              )}
              {bot.activeView === 'operative' && (
                <OperativePanel agentId={bot.agentId} nodeId={bot.agentId} />
              )}
              {bot.activeView === 'files' && (
                <FileViewer agentId={bot.agentId} />
              )}
            </Suspense>
          </div>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Overview sub-view
// ---------------------------------------------------------------------------

function BotOverview({ bot }: { bot: BotType }) {
  return (
    <div className="w-full px-4 py-3 space-y-3 text-sm overflow-y-auto h-full">
      {/* Personality */}
      {bot.personality && (
        <div className="text-xs text-muted-foreground italic border-l-2 pl-2.5" style={{ borderColor: bot.color ?? 'var(--border)' }}>
          {bot.personality}
        </div>
      )}

      {/* Info rows */}
      <div className="space-y-2">
        <Row label="Agent ID" value={bot.agentId} />
        {bot.role && <Row label="Role" value={bot.role} />}
        {bot.model && <Row label="Model" value={bot.model} />}
        {bot.workspace && <Row label="Workspace" value={bot.workspace} />}
        {bot.sessionKey && <Row label="Session" value={bot.sessionKey} />}
        {bot.owner && <Row label="Owner" value={bot.owner} />}
        {bot.lastActivity && (
          <Row label="Last Active" value={new Date(bot.lastActivity).toLocaleTimeString()} />
        )}
      </div>

      {/* Skills */}
      {bot.skills && bot.skills.length > 0 && (
        <div>
          <div className="text-xs text-muted-foreground mb-1.5 uppercase tracking-wider font-medium">Skills</div>
          <div className="flex flex-wrap gap-1.5">
            {bot.skills.map((skill) => (
              <span key={skill} className="text-xs px-2.5 py-1 rounded-md bg-muted/50 text-muted-foreground">
                {skill}
              </span>
            ))}
          </div>
        </div>
      )}

      {/* Channels */}
      {bot.channels && bot.channels.length > 0 && (
        <div>
          <div className="text-xs text-muted-foreground mb-1.5 uppercase tracking-wider font-medium">Channels</div>
          <div className="flex flex-wrap gap-1.5">
            {bot.channels.map((ch) => (
              <span key={ch} className="text-xs px-2.5 py-1 rounded-md bg-primary/10 text-primary/80">
                {ch}
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function Loading() {
  return (
    <div className="flex items-center justify-center h-full text-sm text-muted-foreground">
      Loading...
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-3">
      <span className="text-muted-foreground shrink-0">{label}</span>
      <span className="truncate text-foreground font-mono text-xs">{value}</span>
    </div>
  );
}
