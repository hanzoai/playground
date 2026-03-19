/**
 * RuntimeSelector
 *
 * Shown when a bot is first created (no runtime set) or when user clicks
 * the runtime badge in the header. Lets user pick which AI agent to run.
 * hanzo-dev is default and highlighted -- uses native ZAP + IAM + payments.
 * Others require the user to configure their own API key.
 */

import { useState, useRef, useEffect, useCallback } from 'react';
import { cn } from '@/lib/utils';
import { AGENT_RUNTIMES, type AgentRuntime } from '@/types/canvas';

interface RuntimeSelectorProps {
  currentRuntime?: AgentRuntime;
  onSelect: (runtime: AgentRuntime) => void;
  compact?: boolean;
}

// ---------------------------------------------------------------------------
// Full overlay (first boot)
// ---------------------------------------------------------------------------

function RuntimeOverlay({ onSelect }: { onSelect: (runtime: AgentRuntime) => void }) {
  return (
    <div className="flex flex-col items-center justify-center h-full w-full p-4 gap-4">
      <div className="text-center mb-1">
        <h3 className="text-sm font-semibold text-foreground">Choose Agent Runtime</h3>
        <p className="text-xs text-muted-foreground mt-0.5">
          Select which AI agent powers this bot
        </p>
      </div>

      <div className="grid grid-cols-2 gap-2 w-full max-w-sm">
        {AGENT_RUNTIMES.map((rt) => {
          const isRecommended = rt.key === 'hanzo-dev';
          return (
            <button
              key={rt.key}
              type="button"
              onClick={() => onSelect(rt.key)}
              className={cn(
                'relative flex flex-col items-start gap-1 p-3 rounded-xl text-left transition-all duration-150',
                'border hover:scale-[1.02] active:scale-[0.98]',
                isRecommended
                  ? 'border-primary/40 bg-primary/5 hover:bg-primary/10 ring-1 ring-primary/20'
                  : 'border-white/[0.08] bg-zinc-800/40 hover:bg-zinc-800/70',
              )}
            >
              {isRecommended && (
                <span className="absolute -top-2 right-2 text-[9px] px-1.5 py-0.5 rounded-full bg-primary text-primary-foreground font-semibold uppercase tracking-wider">
                  Recommended
                </span>
              )}

              <div className="flex items-center gap-2">
                <span className="text-base leading-none">{rt.icon}</span>
                <span className="text-sm font-medium text-foreground">{rt.label}</span>
              </div>

              <p className="text-[10px] text-muted-foreground leading-snug">
                {rt.description}
              </p>

              {rt.auth === 'custom' && (
                <span className="text-[9px] px-1.5 py-0.5 rounded-full bg-amber-500/10 text-amber-400 font-medium mt-0.5">
                  Requires API Key
                </span>
              )}
              {rt.auth === 'hanzo' && (
                <span className="text-[9px] px-1.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 font-medium mt-0.5">
                  IAM Login
                </span>
              )}
            </button>
          );
        })}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Compact dropdown (header badge click)
// ---------------------------------------------------------------------------

function RuntimeDropdown({
  currentRuntime,
  onSelect,
  onClose,
}: {
  currentRuntime: AgentRuntime;
  onSelect: (runtime: AgentRuntime) => void;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as HTMLElement)) {
        onClose();
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [onClose]);

  return (
    <div
      ref={ref}
      className="absolute top-full right-0 mt-1 z-50 min-w-[200px] rounded-xl border border-white/[0.08] bg-zinc-900/95 backdrop-blur-lg shadow-2xl overflow-hidden"
    >
      <div className="px-3 py-2 text-[10px] text-muted-foreground uppercase tracking-wider font-medium border-b border-white/[0.06]">
        Agent Runtime
      </div>
      {AGENT_RUNTIMES.map((rt) => (
        <button
          key={rt.key}
          type="button"
          onClick={() => {
            onSelect(rt.key);
            onClose();
          }}
          className={cn(
            'flex items-center gap-2 w-full px-3 py-2 text-left transition-colors duration-100',
            rt.key === currentRuntime
              ? 'bg-primary/10 text-foreground'
              : 'text-muted-foreground hover:bg-white/[0.04] hover:text-foreground',
          )}
        >
          <span className="text-sm leading-none">{rt.icon}</span>
          <div className="flex-1 min-w-0">
            <div className="text-xs font-medium truncate">{rt.label}</div>
            <div className="text-[10px] text-muted-foreground/70 truncate">{rt.description}</div>
          </div>
          {rt.key === currentRuntime && (
            <span className="text-[10px] text-primary shrink-0">
              <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                <path d="M2 6l3 3 5-5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </span>
          )}
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Exported component
// ---------------------------------------------------------------------------

export function RuntimeSelector({ currentRuntime, onSelect, compact }: RuntimeSelectorProps) {
  const [open, setOpen] = useState(false);
  const handleSelect = useCallback(
    (runtime: AgentRuntime) => {
      onSelect(runtime);
      setOpen(false);
    },
    [onSelect],
  );

  if (!compact) {
    return <RuntimeOverlay onSelect={handleSelect} />;
  }

  const rt = AGENT_RUNTIMES.find((r) => r.key === (currentRuntime ?? 'hanzo-dev'));

  return (
    <div className="relative">
      <button
        type="button"
        onClick={(e) => {
          e.stopPropagation();
          setOpen((prev) => !prev);
        }}
        className="text-[10px] px-2 py-0.5 rounded-full shrink-0 font-medium bg-primary/10 text-primary/80 hover:bg-primary/20 transition-colors"
        title={`Running: ${rt?.label ?? 'Hanzo Dev'}`}
      >
        {rt?.icon} {rt?.label}
      </button>
      {open && (
        <RuntimeDropdown
          currentRuntime={currentRuntime ?? 'hanzo-dev'}
          onSelect={handleSelect}
          onClose={() => setOpen(false)}
        />
      )}
    </div>
  );
}
