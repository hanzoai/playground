/**
 * CanvasControls
 *
 * Floating control bar for the canvas.
 * Zoom, fit-to-view, add actions.
 */

import { useReactFlow, useViewport } from '@xyflow/react';
import { useCallback, useState, useRef, useEffect } from 'react';
import { cn } from '@/lib/utils';

interface CanvasControlsProps {
  onFitView: () => void;
  onAddBot?: (position: { x: number; y: number }) => void;
  onAddStarter?: (position: { x: number; y: number }) => void;
}

export function CanvasControls({ onFitView, onAddBot, onAddStarter }: CanvasControlsProps) {
  const { zoomIn, zoomOut } = useReactFlow();
  const { zoom } = useViewport();
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  const handleZoomIn = useCallback(() => zoomIn({ duration: 200 }), [zoomIn]);
  const handleZoomOut = useCallback(() => zoomOut({ duration: 200 }), [zoomOut]);

  const zoomPercent = Math.round(zoom * 100);

  // Close menu on outside click
  useEffect(() => {
    if (!menuOpen) return;
    const handleClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [menuOpen]);

  const center = { x: window.innerWidth / 2, y: window.innerHeight / 2 };

  return (
    <div className="absolute bottom-4 left-1/2 -translate-x-1/2 z-10 flex items-center gap-1 rounded-xl border border-border/50 bg-card/90 px-2 py-1.5 shadow-lg backdrop-blur-sm">
      <ControlButton onClick={handleZoomOut} label="Zoom out">
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
          <path d="M3 7h8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      </ControlButton>

      <span className="min-w-[3rem] text-center text-xs text-muted-foreground tabular-nums select-none">
        {zoomPercent}%
      </span>

      <ControlButton onClick={handleZoomIn} label="Zoom in">
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
          <path d="M7 3v8M3 7h8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      </ControlButton>

      <div className="mx-1 h-4 w-px bg-border/50" />

      <ControlButton onClick={onFitView} label="Fit to view">
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
          <rect x="2" y="2" width="10" height="10" rx="1.5" stroke="currentColor" strokeWidth="1.2" />
          <path d="M5 7h4M7 5v4" stroke="currentColor" strokeWidth="1" strokeLinecap="round" />
        </svg>
      </ControlButton>

      {(onAddBot || onAddStarter) && (
        <>
          <div className="mx-1 h-4 w-px bg-border/50" />
          <div className="relative" ref={menuRef}>
            <ControlButton
              onClick={() => setMenuOpen(!menuOpen)}
              label="Add"
              className={menuOpen ? 'bg-accent text-foreground' : undefined}
            >
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                <path d="M7 2v10M2 7h10" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
              </svg>
            </ControlButton>

            {menuOpen && (
              <div className="absolute bottom-full mb-2 left-1/2 -translate-x-1/2 min-w-[160px] rounded-xl border border-border/60 bg-card/95 py-1 shadow-xl backdrop-blur-sm">
                {onAddBot && (
                  <button
                    type="button"
                    onClick={() => { onAddBot(center); setMenuOpen(false); }}
                    className="flex w-full items-center gap-2.5 px-3 py-2 text-sm text-foreground/80 transition-colors hover:bg-accent hover:text-foreground"
                  >
                    <span className="text-base w-5 text-center">🤖</span>
                    <span>Add Bot</span>
                  </button>
                )}
                {onAddStarter && (
                  <button
                    type="button"
                    onClick={() => { onAddStarter(center); setMenuOpen(false); }}
                    className="flex w-full items-center gap-2.5 px-3 py-2 text-sm text-foreground/80 transition-colors hover:bg-accent hover:text-foreground"
                  >
                    <span className="text-base w-5 text-center">+</span>
                    <span>Add Starter</span>
                  </button>
                )}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// ControlButton
// ---------------------------------------------------------------------------

function ControlButton({
  onClick,
  label,
  children,
  className,
}: {
  onClick: () => void;
  label: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      title={label}
      aria-label={label}
      className={cn(
        'flex h-7 w-7 items-center justify-center rounded-lg text-muted-foreground',
        'transition-colors hover:bg-accent hover:text-foreground',
        'active:scale-95 touch-manipulation',
        className
      )}
    >
      {children}
    </button>
  );
}
