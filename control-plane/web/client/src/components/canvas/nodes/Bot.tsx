/**
 * Bot Node
 *
 * Primary canvas node representing an agent bot.
 * Clean card with status, name, and source badge.
 * Click to expand into terminal/chat/desktop/files.
 * Handles hidden by default, shown on hover for connecting.
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
        'group relative rounded-xl bg-zinc-800/90 transition-all touch-manipulation',
        selected
          ? 'ring-1 ring-primary/50 shadow-[0_0_16px_-2px] shadow-primary/25'
          : 'ring-1 ring-white/[0.10] hover:ring-white/[0.20]',
        expanded ? 'w-[320px] md:w-[400px] shadow-2xl' : 'w-[180px] md:w-[200px] shadow-lg',
      )}
    >
      {/* Handles - hidden by default, shown on hover */}
      <Handle
        type="target"
        position={Position.Top}
        className="!w-2.5 !h-2.5 !bg-primary/60 !border-2 !border-card !opacity-0 group-hover:!opacity-100 !transition-opacity"
      />
      <Handle
        type="source"
        position={Position.Bottom}
        className="!w-2.5 !h-2.5 !bg-primary/60 !border-2 !border-card !opacity-0 group-hover:!opacity-100 !transition-opacity"
      />

      {/* Header */}
      <div
        className="flex items-center gap-2.5 px-3 py-2.5 cursor-pointer select-none"
        onClick={handleToggleExpand}
      >
        {/* Status dot */}
        <span className={cn(
          'h-2 w-2 rounded-full shrink-0',
          status.color,
          status.pulse && 'animate-pulse'
        )} />

        {/* Avatar / Emoji */}
        {bot.avatar ? (
          <img src={bot.avatar} alt={bot.name} className="h-6 w-6 rounded-full object-cover shrink-0" />
        ) : (
          <span className="text-base leading-none shrink-0">{bot.emoji ?? '🤖'}</span>
        )}

        {/* Name + Status */}
        <div className="flex-1 min-w-0">
          <div className="text-sm font-medium truncate">{bot.name}</div>
          <div className="text-[10px] text-muted-foreground leading-tight">{status.label}</div>
        </div>

        {/* Source badge */}
        <span className={cn(
          'text-[9px] px-1.5 py-0.5 rounded-full shrink-0 font-medium',
          bot.source === 'local'
            ? 'text-emerald-400 bg-emerald-500/10'
            : 'text-blue-400 bg-blue-500/10'
        )}>
          {bot.source === 'local' ? 'local' : 'cloud'}
        </span>
      </div>

      {/* Compact info - model + skills preview */}
      {!expanded && (
        <div className="px-3 pb-2.5 space-y-1">
          {bot.model && (
            <div className="text-[10px] text-muted-foreground truncate">{bot.model}</div>
          )}
          {bot.skills && bot.skills.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {bot.skills.slice(0, 3).map((skill) => (
                <span key={skill} className="text-[9px] px-1.5 py-0.5 rounded bg-muted/50 text-muted-foreground">
                  {skill}
                </span>
              ))}
              {bot.skills.length > 3 && (
                <span className="text-[9px] text-muted-foreground">+{bot.skills.length - 3}</span>
              )}
            </div>
          )}
        </div>
      )}

      {/* Expanded content */}
      {expanded && (
        <div className="border-t border-white/[0.08]">
          {/* View tabs */}
          <div className="flex border-b border-white/[0.08] overflow-x-auto scrollbar-none">
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
    </div>
  );
}

// ---------------------------------------------------------------------------
// Overview sub-view
// ---------------------------------------------------------------------------

function BotOverview({ bot }: { bot: BotType }) {
  return (
    <div className="w-full px-3 py-2.5 space-y-3 text-xs overflow-y-auto h-full">
      {/* Info rows */}
      <div className="space-y-1.5">
        <Row label="Agent ID" value={bot.agentId} />
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
          <div className="text-[10px] text-muted-foreground mb-1.5 uppercase tracking-wider font-medium">Skills</div>
          <div className="flex flex-wrap gap-1">
            {bot.skills.map((skill) => (
              <span key={skill} className="text-[10px] px-2 py-0.5 rounded-md bg-muted/50 text-muted-foreground">
                {skill}
              </span>
            ))}
          </div>
        </div>
      )}

      {/* Channels */}
      {bot.channels && bot.channels.length > 0 && (
        <div>
          <div className="text-[10px] text-muted-foreground mb-1.5 uppercase tracking-wider font-medium">Channels</div>
          <div className="flex flex-wrap gap-1">
            {bot.channels.map((ch) => (
              <span key={ch} className="text-[10px] px-2 py-0.5 rounded-md bg-primary/10 text-primary/80">
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
