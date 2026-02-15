/**
 * Bot Node
 *
 * Primary canvas node representing an agent bot.
 * Shows status, emoji/avatar/logo, name, model.
 * Click to drill down into terminal/chat/operative/files.
 * Each bot can have its own emoji, icon, or logo.
 *
 * Responsive: compact on mobile, full on desktop.
 */

import { Handle, Position, type NodeProps } from '@xyflow/react';
import { useCallback, useState, lazy, Suspense } from 'react';
import { useCanvasStore } from '@/stores/canvasStore';
import type { Bot as BotType, BotView } from '@/types/canvas';
import { cn } from '@/lib/utils';

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
  const [expanded, setExpanded] = useState(false);
  const status = STATUS[bot.status] ?? STATUS.idle;

  const handleToggleExpand = useCallback(() => {
    setExpanded((prev) => !prev);
  }, []);

  const handleViewChange = useCallback(
    (view: BotView) => {
      setBotView(bot.agentId, view);
    },
    [setBotView, bot.agentId]
  );

  return (
    <div
      className={cn(
        'group relative rounded-xl border bg-card shadow-lg backdrop-blur transition-all touch-manipulation',
        selected ? 'border-primary shadow-primary/20 ring-1 ring-primary/30' : 'border-border/60',
        expanded ? 'w-[320px] md:w-[400px]' : 'w-[160px] md:w-[200px]',
      )}
    >
      {/* Handles */}
      <Handle type="target" position={Position.Top} className="!w-2 !h-2 !bg-border" />
      <Handle type="source" position={Position.Bottom} className="!w-2 !h-2 !bg-border" />

      {/* Header */}
      <div
        className="flex items-center gap-2 px-3 py-2.5 cursor-pointer select-none"
        onClick={handleToggleExpand}
      >
        {/* Status dot */}
        <span className="relative flex h-2.5 w-2.5 shrink-0">
          {status.pulse && (
            <span className={cn('absolute inline-flex h-full w-full animate-ping rounded-full opacity-75', status.color)} />
          )}
          <span className={cn('relative inline-flex h-2.5 w-2.5 rounded-full', status.color)} />
        </span>

        {/* Avatar / Emoji / Logo */}
        {bot.avatar ? (
          <img src={bot.avatar} alt={bot.name} className="h-6 w-6 rounded-full object-cover shrink-0" />
        ) : (
          <span className="text-lg leading-none shrink-0">{bot.emoji ?? '🤖'}</span>
        )}

        {/* Name + Status */}
        <div className="flex-1 min-w-0">
          <div className="text-sm font-medium truncate">{bot.name}</div>
          <div className="text-[10px] text-muted-foreground">{status.label}</div>
        </div>

        {/* Source badge */}
        <span className={cn(
          'text-[9px] px-1.5 py-0.5 rounded-full border shrink-0',
          bot.source === 'local'
            ? 'border-green-500/30 text-green-600 bg-green-500/10'
            : 'border-blue-500/30 text-blue-600 bg-blue-500/10'
        )}>
          {bot.source === 'local' ? 'local' : 'cloud'}
        </span>
      </div>

      {/* Expanded content */}
      {expanded && (
        <div className="border-t border-border/40">
          {/* View tabs */}
          <div className="flex border-b border-border/30 overflow-x-auto scrollbar-none">
            {VIEW_TABS.map((tab) => (
              <button
                key={tab.key}
                type="button"
                onClick={() => handleViewChange(tab.key)}
                className={cn(
                  'flex items-center gap-1 px-2.5 py-1.5 text-[11px] whitespace-nowrap transition-colors shrink-0',
                  bot.activeView === tab.key
                    ? 'text-foreground border-b-2 border-primary'
                    : 'text-muted-foreground hover:text-foreground'
                )}
              >
                <span className="text-xs">{tab.icon}</span>
                <span className="hidden sm:inline">{tab.label}</span>
              </button>
            ))}
          </div>

          {/* Drill-down views */}
          <div className="h-[200px] md:h-[300px] overflow-hidden">
            <Suspense fallback={<Loading />}>
              {bot.activeView === 'overview' && <BotOverview bot={bot} />}
              {bot.activeView === 'terminal' && (
                <TerminalPanel agentId={bot.agentId} sessionKey={bot.sessionKey} />
              )}
              {bot.activeView === 'chat' && (
                <ChatPanel agentId={bot.agentId} sessionKey={bot.sessionKey} />
              )}
              {bot.activeView === 'operative' && (
                <OperativePanel agentId={bot.agentId} />
              )}
              {bot.activeView === 'files' && (
                <FileViewer agentId={bot.agentId} />
              )}
            </Suspense>
          </div>
        </div>
      )}

      {/* Model badge */}
      {bot.model && !expanded && (
        <div className="px-3 pb-2 text-[10px] text-muted-foreground truncate">
          {bot.model}
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
    <div className="w-full px-3 py-2 space-y-2 text-xs">
      <Row label="Agent ID" value={bot.agentId} />
      {bot.model && <Row label="Model" value={bot.model} />}
      {bot.workspace && <Row label="Workspace" value={bot.workspace} />}
      {bot.sessionKey && <Row label="Session" value={bot.sessionKey} />}
      {bot.lastActivity && (
        <Row label="Last Activity" value={new Date(bot.lastActivity).toLocaleTimeString()} />
      )}
    </div>
  );
}

function Loading() {
  return (
    <div className="flex items-center justify-center h-full text-xs text-muted-foreground">
      Loading...
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-2">
      <span className="text-muted-foreground shrink-0">{label}</span>
      <span className="truncate text-foreground font-mono text-[10px]">{value}</span>
    </div>
  );
}
