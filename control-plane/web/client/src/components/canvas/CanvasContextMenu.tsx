/**
 * CanvasContextMenu
 *
 * Right-click menu on the canvas for adding nodes.
 */

import { useCallback, useEffect, useRef } from 'react';
import { cn } from '@/lib/utils';

interface ContextMenuPosition {
  x: number;
  y: number;
}

interface CanvasContextMenuProps {
  position: ContextMenuPosition | null;
  onClose: () => void;
  onAddBot: (position: ContextMenuPosition) => void;
  onAddStarter: (position: ContextMenuPosition) => void;
}

const ITEMS = [
  { key: 'bot', label: 'Add Bot', icon: '🤖', action: 'addBot' },
  { key: 'starter', label: 'Add Starter', icon: '+', action: 'addStarter' },
] as const;

export function CanvasContextMenu({ position, onClose, onAddBot, onAddStarter }: CanvasContextMenuProps) {
  const ref = useRef<HTMLDivElement>(null);

  // Close on click outside
  useEffect(() => {
    if (!position) return;
    const handleClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        onClose();
      }
    };
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('mousedown', handleClick);
    document.addEventListener('keydown', handleEscape);
    return () => {
      document.removeEventListener('mousedown', handleClick);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [position, onClose]);

  const handleAction = useCallback(
    (action: string) => {
      if (!position) return;
      if (action === 'addBot') onAddBot(position);
      if (action === 'addStarter') onAddStarter(position);
      onClose();
    },
    [position, onAddBot, onAddStarter, onClose]
  );

  if (!position) return null;

  return (
    <div
      ref={ref}
      className="fixed z-50 min-w-[160px] rounded-xl border border-border/60 bg-card/95 py-1 shadow-xl backdrop-blur-sm"
      style={{ left: position.x, top: position.y }}
    >
      {ITEMS.map((item) => (
        <button
          key={item.key}
          type="button"
          onClick={() => handleAction(item.action)}
          className={cn(
            'flex w-full items-center gap-2.5 px-3 py-2 text-sm text-foreground/80',
            'transition-colors hover:bg-accent hover:text-foreground',
            'touch-manipulation'
          )}
        >
          <span className="text-base w-5 text-center">{item.icon}</span>
          <span>{item.label}</span>
        </button>
      ))}
    </div>
  );
}
