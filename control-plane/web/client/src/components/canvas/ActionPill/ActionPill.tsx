/**
 * ActionPill
 *
 * Centralized approval UI for tool-use requests from bots.
 * Appears as a floating pill at the bottom of the canvas.
 * Keyboard: Tab (next), Shift+Tab (prev), Enter (approve), Backspace (deny).
 */

import { useEffect, useCallback } from 'react';
import { useActionPillStore } from '@/stores/actionPillStore';
import { usePermissionModeStore } from '@/stores/permissionModeStore';
import { execApprovalResolve } from '@/services/gatewayApi';
import { cn } from '@/lib/utils';

export function ActionPill() {
  const active = useActionPillStore((s) => s.active());
  const pendingCount = useActionPillStore((s) => s.pendingCount());
  const next = useActionPillStore((s) => s.next);
  const prev = useActionPillStore((s) => s.prev);
  const resolve = useActionPillStore((s) => s.resolve);
  const prune = useActionPillStore((s) => s.prune);
  const getEffective = usePermissionModeStore((s) => s.getEffective);

  const handleApprove = useCallback(async () => {
    if (!active) return;
    resolve(active.id, 'approved');
    try {
      await execApprovalResolve({
        approvalId: active.id,
        decision: 'allow',
      });
    } catch (e) {
      console.error('[ActionPill] Failed to send approval:', e);
    }
    prune();
  }, [active, resolve, prune]);

  const handleDeny = useCallback(async () => {
    if (!active) return;
    resolve(active.id, 'denied');
    try {
      await execApprovalResolve({
        approvalId: active.id,
        decision: 'deny',
      });
    } catch (e) {
      console.error('[ActionPill] Failed to send denial:', e);
    }
    prune();
  }, [active, resolve, prune]);

  // Auto-approve in auto-accept mode
  useEffect(() => {
    if (!active) return;
    const mode = getEffective(active.agentId);
    if (mode === 'auto-accept') {
      handleApprove();
    }
  }, [active, getEffective, handleApprove]);

  // Keyboard shortcuts
  useEffect(() => {
    if (pendingCount === 0) return;

    const handler = (e: KeyboardEvent) => {
      // Don't intercept when focused on input/textarea
      const tag = (e.target as HTMLElement)?.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;

      if (e.key === 'Tab' && !e.shiftKey) {
        e.preventDefault();
        next();
      } else if (e.key === 'Tab' && e.shiftKey) {
        e.preventDefault();
        prev();
      } else if (e.key === 'Enter') {
        e.preventDefault();
        handleApprove();
      } else if (e.key === 'Backspace') {
        e.preventDefault();
        handleDeny();
      }
    };

    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [pendingCount, next, prev, handleApprove, handleDeny]);

  if (!active || pendingCount === 0) return null;

  return (
    <div className="absolute bottom-4 left-1/2 -translate-x-1/2 z-30 pointer-events-auto">
      <div className="flex items-center gap-2 rounded-2xl border border-border/60 bg-card/95 px-4 py-2.5 shadow-2xl backdrop-blur-md">
        {/* Counter */}
        {pendingCount > 1 && (
          <div className="flex items-center gap-1 text-xs text-muted-foreground">
            <button
              type="button"
              onClick={prev}
              className="h-6 w-6 rounded-full hover:bg-accent flex items-center justify-center transition-colors touch-manipulation"
              title="Previous (Shift+Tab)"
            >
              ‹
            </button>
            <span className="min-w-[2ch] text-center font-mono">
              {(useActionPillStore.getState().activeIndex % pendingCount) + 1}/{pendingCount}
            </span>
            <button
              type="button"
              onClick={next}
              className="h-6 w-6 rounded-full hover:bg-accent flex items-center justify-center transition-colors touch-manipulation"
              title="Next (Tab)"
            >
              ›
            </button>
          </div>
        )}

        {/* Tool info */}
        <div className="flex items-center gap-2 border-l border-border/30 pl-3">
          <span className="text-xs text-muted-foreground truncate max-w-[100px] md:max-w-[200px]">
            {active.agentId}
          </span>
          <span className="font-mono text-sm font-medium text-foreground truncate max-w-[120px] md:max-w-[240px]">
            {active.toolName}
          </span>
        </div>

        {/* Actions */}
        <div className="flex items-center gap-1.5 border-l border-border/30 pl-3">
          <button
            type="button"
            onClick={handleDeny}
            className={cn(
              'rounded-lg px-3 py-1.5 text-xs font-medium transition-colors touch-manipulation',
              'bg-red-500/10 text-red-600 hover:bg-red-500/20',
              'dark:text-red-400 dark:hover:bg-red-500/20',
            )}
            title="Deny (Backspace)"
          >
            Deny
          </button>
          <button
            type="button"
            onClick={handleApprove}
            className={cn(
              'rounded-lg px-3 py-1.5 text-xs font-medium transition-colors touch-manipulation',
              'bg-green-500/10 text-green-600 hover:bg-green-500/20',
              'dark:text-green-400 dark:hover:bg-green-500/20',
            )}
            title="Approve (Enter)"
          >
            Approve
          </button>
        </div>

        {/* Keyboard hint (desktop only) */}
        <div className="hidden md:flex items-center gap-1 text-[10px] text-muted-foreground/60 border-l border-border/20 pl-2">
          <kbd className="rounded border border-border/40 px-1">⏎</kbd>
          <kbd className="rounded border border-border/40 px-1">⌫</kbd>
          <kbd className="rounded border border-border/40 px-1">⇥</kbd>
        </div>
      </div>
    </div>
  );
}
