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

const STATUS: Record<string, { color: string; label: string; pulse?: boolean }> = {
  idle:         { color: 'bg-blue-500',   label: 'Idle' },
  busy:         { color: 'bg-green-500',  label: 'Working', pulse: true },
  waiting:      { color: 'bg-yellow-500', label: 'Waiting' },
  error:        { color: 'bg-red-500',    label: 'Error' },
  offline:      { color: 'bg-gray-400',   label: 'Offline' },
  provisioning: { color: 'bg-purple-500', label: 'Provisioning', pulse: true },
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
        'group relative rounded-xl bg-zinc-800/90 transition-all touch-manipulation',
        'w-full h-full flex flex-col',
        selected
          ? 'ring-2 ring-primary/60 shadow-[0_0_24px_-2px] shadow-primary/30'
          : 'ring-1 ring-white/[0.10] hover:ring-white/[0.20]',
        'shadow-2xl',
      )}
      style={{
        ...(bot.color && selected ? { boxShadow: `0 0 24px -2px ${bot.color}40` } : {}),
        ...(bot.color && !selected && (bot.status === 'busy') ? { boxShadow: `0 0 16px -4px ${bot.color}30` } : {}),
        ...(bot.color ? { borderColor: selected ? `${bot.color}60` : undefined } : {}),
      }}
    >
      {/* Resize handles */}
      <NodeResizer
        minWidth={BOT_NODE_MIN_WIDTH}
        minHeight={collapsed ? 52 : BOT_NODE_MIN_HEIGHT}
        isVisible={selected ?? false}
        lineClassName="!border-primary/30"
        handleClassName="!w-2.5 !h-2.5 !bg-primary/60 !border-2 !border-zinc-800"
      />

      {/* Connection handles - hidden by default, shown on hover */}
      <Handle
        type="target"
        position={Position.Top}
        className="!w-3 !h-3 !bg-primary/60 !border-2 !border-card !opacity-0 group-hover:!opacity-100 !transition-opacity"
      />
      <Handle
        type="source"
        position={Position.Bottom}
        className="!w-3 !h-3 !bg-primary/60 !border-2 !border-card !opacity-0 group-hover:!opacity-100 !transition-opacity"
      />

      {/* Header - always visible, prominent */}
      <div
        className="flex items-center gap-3 px-4 py-3 cursor-pointer select-none shrink-0"
        onClick={handleToggleCollapse}
      >
        {/* Status dot */}
        <span className={cn(
          'h-3 w-3 rounded-full shrink-0',
          status.color,
          status.pulse && 'animate-pulse'
        )} />

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
            {bot.emoji && bot.avatar && (
              <span className="text-sm leading-none">{bot.emoji}</span>
            )}
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
        <div className="flex-1 flex flex-col border-t border-white/[0.08] min-h-0 overflow-hidden">
          {/* Toolbar: view tabs + actions */}
          <div className="flex items-center border-b border-white/[0.08] shrink-0">
            <div className="flex flex-1 overflow-x-auto scrollbar-none">
              {VIEW_TABS.map((tab) => (
                <button
                  key={tab.key}
                  type="button"
                  onClick={() => handleViewChange(tab.key)}
                  className={cn(
                    'flex items-center gap-1.5 px-3 py-2 text-xs whitespace-nowrap transition-colors shrink-0',
                    bot.activeView === tab.key
                      ? 'text-foreground border-b-2 border-primary'
                      : 'text-muted-foreground hover:text-foreground'
                  )}
                >
                  <span className="text-sm">{tab.icon}</span>
                  <span>{tab.label}</span>
                </button>
              ))}
            </div>

            {/* Mini toolbar actions */}
            <div className="flex items-center gap-0.5 px-2 shrink-0">
              {/* Collapse button */}
              <button
                type="button"
                onClick={handleToggleCollapse}
                title="Collapse"
                className="flex h-6 w-6 items-center justify-center rounded text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
              >
                <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                  <path d="M2 4l4 4 4-4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </button>
              {/* Close button */}
              <button
                type="button"
                onClick={handleClose}
                title="Remove from canvas"
                className="flex h-6 w-6 items-center justify-center rounded text-muted-foreground hover:text-red-400 hover:bg-red-500/10 transition-colors"
              >
                <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                  <path d="M3 3l6 6M9 3l-6 6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
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
