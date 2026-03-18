/**
 * CanvasControls
 *
 * Floating control bar for the canvas.
 * Zoom, fit-to-view, add actions (including cloud provisioning).
 */

import { useReactFlow, useViewport } from '@xyflow/react';
import { useCallback, useState, useRef, useEffect } from 'react';
import { cn } from '@/lib/utils';

interface CanvasControlsProps {
  onFitView: () => void;
  onAutoLayout?: () => void;
  onAddBot?: (position: { x: number; y: number }) => void;
  onAddStarter?: (position: { x: number; y: number }) => void;
  onLaunchCloud?: (type: 'linux' | 'terminal' | 'desktop') => void;
  onToggleChat?: () => void;
  chatUnread?: number;
}

export function CanvasControls({ onFitView, onAutoLayout, onAddBot, onAddStarter, onLaunchCloud, onToggleChat, chatUnread }: CanvasControlsProps) {
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
    <>
      {/* Zoom controls — bottom center */}
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

        {onAutoLayout && (
          <>
            <div className="mx-1 h-4 w-px bg-border/50" />
            <ControlButton onClick={onAutoLayout} label="Auto layout">
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                <rect x="1" y="1" width="5" height="5" rx="1" stroke="currentColor" strokeWidth="1.2" />
                <rect x="8" y="1" width="5" height="5" rx="1" stroke="currentColor" strokeWidth="1.2" />
                <rect x="1" y="8" width="5" height="5" rx="1" stroke="currentColor" strokeWidth="1.2" />
                <rect x="8" y="8" width="5" height="5" rx="1" stroke="currentColor" strokeWidth="1.2" />
              </svg>
            </ControlButton>
          </>
        )}

        {onToggleChat && (
          <>
            <div className="mx-1 h-4 w-px bg-border/50" />
            <div className="relative">
              <ControlButton onClick={onToggleChat} label="Toggle chat">
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                  <path d="M2 2h10a1 1 0 011 1v6a1 1 0 01-1 1H5l-3 3V3a1 1 0 011-1z" stroke="currentColor" strokeWidth="1.2" strokeLinejoin="round" />
                </svg>
              </ControlButton>
              {(chatUnread ?? 0) > 0 && (
                <span className="absolute -top-1 -right-1 flex h-4 min-w-[1rem] items-center justify-center rounded-full bg-destructive px-0.5 text-[9px] font-bold text-destructive-foreground">
                  {chatUnread! > 99 ? '99+' : chatUnread}
                </span>
              )}
            </div>
          </>
        )}
      </div>

      {/* FAB — bottom left */}
      <div className="absolute bottom-4 left-4 z-10" ref={menuRef}>
        <button
          type="button"
          onClick={() => setMenuOpen(!menuOpen)}
          title="Add"
          aria-label="Add bot or service"
          className={cn(
            'flex h-12 w-12 items-center justify-center rounded-2xl shadow-lg',
            'bg-primary text-primary-foreground',
            'transition-all hover:scale-105 hover:shadow-xl active:scale-95',
            menuOpen && 'rotate-45'
          )}
        >
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
            <path d="M10 3v14M3 10h14" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" />
          </svg>
        </button>

        {menuOpen && (
          <div className="absolute bottom-full mb-2 left-0 min-w-[200px] rounded-xl border border-border/60 bg-card/95 py-1.5 shadow-xl backdrop-blur-sm">
            {onLaunchCloud && (
              <>
                <MenuItem
                  icon="⌨️"
                  label="Launch Terminal"
                  description="Lightweight shell (512Mi)"
                  onClick={() => { onLaunchCloud('terminal'); setMenuOpen(false); }}
                />
                <MenuItem
                  icon="🖥️"
                  label="Launch Desktop"
                  description="Linux + VNC desktop (512Mi+)"
                  onClick={() => { onLaunchCloud('desktop'); setMenuOpen(false); }}
                />
                <MenuItem
                  icon="🤖"
                  label="Launch Cloud Bot"
                  description="Full bot runtime (512Mi+)"
                  onClick={() => { onLaunchCloud('linux'); setMenuOpen(false); }}
                />
                <div className="mx-2 my-1 h-px bg-border/40" />
              </>
            )}
            {onAddBot && (
              <MenuItem
                icon="+"
                label="Add to Canvas"
                description="Empty bot node"
                onClick={() => { onAddBot(center); setMenuOpen(false); }}
              />
            )}
            {onAddStarter && (
              <MenuItem
                icon="▶"
                label="Add Starter"
                description="Workflow trigger"
                onClick={() => { onAddStarter(center); setMenuOpen(false); }}
              />
            )}
          </div>
        )}
      </div>
    </>
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

// ---------------------------------------------------------------------------
// MenuItem
// ---------------------------------------------------------------------------

function MenuItem({
  icon,
  label,
  description,
  onClick,
}: {
  icon: string;
  label: string;
  description: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex w-full items-start gap-2.5 px-3 py-2 text-left transition-colors hover:bg-accent"
    >
      <span className="text-base w-5 text-center flex-shrink-0 mt-0.5">{icon}</span>
      <div className="min-w-0">
        <div className="text-sm font-medium text-foreground">{label}</div>
        <div className="text-xs text-muted-foreground">{description}</div>
      </div>
    </button>
  );
}
