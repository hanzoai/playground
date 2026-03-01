/**
 * BotContextMenu
 *
 * Right-click context menu for bot actions.
 * Works from both canvas nodes and sidebar items.
 */

import { useCallback, useEffect, useRef } from 'react';
import { cn } from '@/lib/utils';
import { useCanvasStore } from '@/stores/canvasStore';
import { useBotStore } from '@/stores/botStore';
import { agentsDelete, chatAbort } from '@/services/gatewayApi';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface BotContextMenuState {
  x: number;
  y: number;
  agentId: string;
  sessionKey?: string;
  status?: string;
}

interface BotContextMenuProps {
  state: BotContextMenuState | null;
  onClose: () => void;
}

// ---------------------------------------------------------------------------
// Menu Items
// ---------------------------------------------------------------------------

interface MenuItem {
  key: string;
  label: string;
  icon: string;
  danger?: boolean;
  separator?: boolean;
  disabled?: (status?: string) => boolean;
}

const MENU_ITEMS: MenuItem[] = [
  { key: 'stop',     label: 'Stop Bot',        icon: '⏹', disabled: (s) => s === 'idle' || s === 'offline' },
  { key: 'restart',  label: 'Restart Bot',      icon: '🔄' },
  { key: 'sep1',     label: '',                 icon: '', separator: true },
  { key: 'terminal', label: 'Open Terminal',    icon: '⌨️' },
  { key: 'chat',     label: 'Open Chat',        icon: '💬' },
  { key: 'desktop',  label: 'Open Desktop',     icon: '🖥️' },
  { key: 'sep2',     label: '',                 icon: '', separator: true },
  { key: 'copyId',   label: 'Copy Bot ID',      icon: '📋' },
  { key: 'sep3',     label: '',                 icon: '', separator: true },
  { key: 'remove',   label: 'Remove from Canvas', icon: '✕' },
  { key: 'delete',   label: 'Delete Bot',       icon: '🗑️', danger: true },
];

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function BotContextMenu({ state, onClose }: BotContextMenuProps) {
  const ref = useRef<HTMLDivElement>(null);
  const removeBot = useCanvasStore((s) => s.removeBot);
  const setBotView = useCanvasStore((s) => s.setBotView);
  const selectNode = useCanvasStore((s) => s.selectNode);
  const nodes = useCanvasStore((s) => s.nodes);

  // Close on click outside or Escape
  useEffect(() => {
    if (!state) return;
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
  }, [state, onClose]);

  const handleAction = useCallback(
    async (key: string) => {
      if (!state) return;
      const { agentId, sessionKey } = state;

      switch (key) {
        case 'stop':
          if (sessionKey) {
            try { await chatAbort({ sessionKey }); } catch { /* ignore */ }
          }
          break;

        case 'restart':
          if (sessionKey) {
            try { await chatAbort({ sessionKey }); } catch { /* ignore */ }
          }
          break;

        case 'terminal':
        case 'chat':
        case 'desktop': {
          const viewMap: Record<string, string> = {
            terminal: 'terminal',
            chat: 'chat',
            desktop: 'operative',
          };
          setBotView(agentId, viewMap[key] as 'terminal' | 'chat' | 'operative');
          const node = nodes.find(
            (n) => n.type === 'bot' && (n.data as Record<string, unknown>).agentId === agentId
          );
          if (node) selectNode(node.id);
          break;
        }

        case 'copyId':
          try { await navigator.clipboard.writeText(agentId); } catch { /* ignore */ }
          break;

        case 'remove':
          removeBot(agentId);
          break;

        case 'delete':
          try { await agentsDelete(agentId); } catch { /* ignore */ }
          removeBot(agentId);
          useBotStore.getState().removeAgent(agentId);
          break;
      }

      onClose();
    },
    [state, removeBot, setBotView, selectNode, nodes, onClose]
  );

  if (!state) return null;

  // Adjust position to stay within viewport
  const menuWidth = 200;
  const menuHeight = MENU_ITEMS.length * 32;
  const x = Math.min(state.x, window.innerWidth - menuWidth - 8);
  const y = Math.min(state.y, window.innerHeight - menuHeight - 8);

  return (
    <div
      ref={ref}
      className="fixed z-[100] min-w-[200px] rounded-xl border border-border/60 bg-card/95 py-1 shadow-xl backdrop-blur-sm"
      style={{ left: x, top: y }}
    >
      {MENU_ITEMS.map((item) => {
        if (item.separator) {
          return <div key={item.key} className="my-1 border-t border-border/30" />;
        }

        const isDisabled = item.disabled?.(state.status);

        return (
          <button
            key={item.key}
            type="button"
            disabled={isDisabled}
            onClick={() => handleAction(item.key)}
            className={cn(
              'flex w-full items-center gap-2.5 px-3 py-1.5 text-sm transition-colors touch-manipulation',
              item.danger
                ? 'text-red-400 hover:bg-red-500/10 hover:text-red-300'
                : 'text-foreground/80 hover:bg-accent hover:text-foreground',
              isDisabled && 'opacity-40 cursor-not-allowed hover:bg-transparent'
            )}
          >
            <span className="text-sm w-5 text-center">{item.icon}</span>
            <span>{item.label}</span>
          </button>
        );
      })}
    </div>
  );
}
