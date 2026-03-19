/**
 * SubAgentCard
 *
 * Compact card for a sub-agent spawned by a parent bot.
 * Shows: icon, name, status, quick action buttons.
 * Animates in from the parent bot (slide down + fade in).
 * Renders at 60% parent width, connected by a thin line/arrow.
 * Click to expand into full terminal view.
 */

import { cn } from '@/lib/utils';
import { AGENT_RUNTIMES, type AgentRuntime } from '@/types/canvas';
import type { BotStatus } from '@/types/gateway';

interface SubAgentCardProps {
  name: string;
  runtime: AgentRuntime;
  status: BotStatus;
  onClick?: () => void;
  onStop?: () => void;
}

const STATUS_DOT: Record<string, { color: string; pulse?: boolean }> = {
  idle:         { color: 'bg-blue-500' },
  busy:         { color: 'bg-emerald-400', pulse: true },
  waiting:      { color: 'bg-amber-400' },
  error:        { color: 'bg-red-500' },
  offline:      { color: 'bg-zinc-500' },
  provisioning: { color: 'bg-purple-500', pulse: true },
};

export function SubAgentCard({ name, runtime, status, onClick, onStop }: SubAgentCardProps) {
  const rt = AGENT_RUNTIMES.find((r) => r.key === runtime);
  const dot = STATUS_DOT[status] ?? STATUS_DOT.idle;

  return (
    <div className="flex flex-col items-center">
      {/* Connector line from parent */}
      <div className="w-px h-4 bg-white/[0.12]" />

      {/* Arrow tip */}
      <div className="w-0 h-0 border-l-[4px] border-l-transparent border-r-[4px] border-r-transparent border-t-[5px] border-t-white/[0.12] mb-1" />

      {/* Card */}
      <button
        type="button"
        onClick={onClick}
        className={cn(
          'group relative flex items-center gap-2 px-3 py-2 rounded-xl',
          'bg-zinc-800/80 border border-white/[0.08] hover:border-white/[0.16]',
          'shadow-lg hover:shadow-xl transition-all duration-200',
          'animate-in slide-in-from-top-2 fade-in duration-300',
          'w-[60%] min-w-[200px]',
        )}
      >
        {/* Status dot */}
        <span className={cn('h-2 w-2 rounded-full shrink-0', dot.color, dot.pulse && 'animate-pulse')} />

        {/* Runtime icon */}
        <span className="text-sm leading-none shrink-0">{rt?.icon ?? '\u26A1'}</span>

        {/* Name + runtime label */}
        <div className="flex-1 min-w-0 text-left">
          <div className="text-xs font-medium text-foreground truncate">{name}</div>
          <div className="text-[10px] text-muted-foreground truncate">{rt?.label ?? 'Unknown'}</div>
        </div>

        {/* Stop button */}
        {onStop && status !== 'offline' && (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onStop();
            }}
            className="flex h-5 w-5 items-center justify-center rounded-full text-muted-foreground/50 hover:text-red-400 hover:bg-red-500/10 transition-all duration-150 opacity-0 group-hover:opacity-100"
            title="Stop sub-agent"
          >
            <svg width="8" height="8" viewBox="0 0 8 8" fill="none">
              <rect x="1" y="1" width="6" height="6" rx="1" fill="currentColor" />
            </svg>
          </button>
        )}
      </button>
    </div>
  );
}
