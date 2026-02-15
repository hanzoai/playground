/**
 * CommandPalette
 *
 * Cmd+K palette for quick actions on the canvas.
 * Search bots, switch views, toggle permissions, navigate.
 */

import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { useAgentStore } from '@/stores/agentStore';
import { useCanvasStore } from '@/stores/canvasStore';
import { usePermissionModeStore } from '@/stores/permissionModeStore';
import { cn } from '@/lib/utils';

interface Command {
  id: string;
  label: string;
  description?: string;
  icon: string;
  action: () => void;
  keywords?: string[];
}

export function CommandPalette() {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  const agents = useAgentStore((s) => s.agents);
  const selectNode = useCanvasStore((s) => s.selectNode);
  const nodes = useCanvasStore((s) => s.nodes);
  const cycleGlobal = usePermissionModeStore((s) => s.cycleGlobal);
  const globalMode = usePermissionModeStore((s) => s.global);

  // Build commands
  const commands = useMemo<Command[]>(() => {
    const cmds: Command[] = [];

    // Bot commands — navigate to each bot
    for (const [id, agent] of agents) {
      const node = nodes.find((n) => n.data && (n.data as Record<string, unknown>).agentId === id);
      if (!node) continue;
      cmds.push({
        id: `bot-${id}`,
        label: agent.name,
        description: `${agent.status} — ${agent.model ?? 'unknown model'}`,
        icon: agent.emoji ?? '🤖',
        action: () => selectNode(node.id),
        keywords: [agent.name, id, agent.model ?? ''].map((s) => s.toLowerCase()),
      });
    }

    // Global actions
    cmds.push({
      id: 'cycle-permission',
      label: `Permission: ${globalMode}`,
      description: 'Cycle global permission mode (plan → auto-accept → ask)',
      icon: '🔒',
      action: cycleGlobal,
      keywords: ['permission', 'mode', 'plan', 'auto', 'ask'],
    });

    return cmds;
  }, [agents, nodes, selectNode, cycleGlobal, globalMode]);

  // Filter
  const filtered = useMemo(() => {
    if (!query) return commands;
    const q = query.toLowerCase();
    return commands.filter((cmd) => {
      if (cmd.label.toLowerCase().includes(q)) return true;
      if (cmd.description?.toLowerCase().includes(q)) return true;
      return cmd.keywords?.some((k) => k.includes(q)) ?? false;
    });
  }, [commands, query]);

  // Clamp selection
  useEffect(() => {
    setSelectedIndex(0);
  }, [query]);

  // Open/close with Cmd+K
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setOpen((prev) => !prev);
        setQuery('');
        setSelectedIndex(0);
      }
      if (e.key === 'Escape') {
        setOpen(false);
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, []);

  // Focus input on open
  useEffect(() => {
    if (open) {
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [open]);

  // Navigate & execute
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setSelectedIndex((i) => Math.min(i + 1, filtered.length - 1));
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        setSelectedIndex((i) => Math.max(i - 1, 0));
      } else if (e.key === 'Enter') {
        e.preventDefault();
        const cmd = filtered[selectedIndex];
        if (cmd) {
          cmd.action();
          setOpen(false);
        }
      }
    },
    [filtered, selectedIndex]
  );

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center pt-[15vh]">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/40 backdrop-blur-sm"
        onClick={() => setOpen(false)}
      />

      {/* Palette */}
      <div className="relative w-[90vw] max-w-md rounded-xl border border-border/60 bg-card shadow-2xl overflow-hidden">
        {/* Search */}
        <div className="flex items-center gap-2 border-b border-border/40 px-4 py-3">
          <span className="text-muted-foreground text-sm">⌘</span>
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Search bots, actions..."
            className="flex-1 bg-transparent text-sm text-foreground placeholder:text-muted-foreground/60 outline-none"
          />
          <kbd className="text-[10px] text-muted-foreground/60 border border-border/40 rounded px-1.5 py-0.5">
            esc
          </kbd>
        </div>

        {/* Results */}
        <div className="max-h-[300px] overflow-y-auto py-1">
          {filtered.length === 0 ? (
            <div className="px-4 py-6 text-center text-sm text-muted-foreground">
              No results
            </div>
          ) : (
            filtered.map((cmd, i) => (
              <button
                key={cmd.id}
                type="button"
                onClick={() => {
                  cmd.action();
                  setOpen(false);
                }}
                className={cn(
                  'flex items-center gap-3 w-full px-4 py-2.5 text-left transition-colors',
                  i === selectedIndex
                    ? 'bg-primary/10 text-foreground'
                    : 'text-muted-foreground hover:text-foreground hover:bg-accent/30',
                )}
              >
                <span className="text-base shrink-0">{cmd.icon}</span>
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium truncate">{cmd.label}</div>
                  {cmd.description && (
                    <div className="text-[11px] text-muted-foreground truncate">{cmd.description}</div>
                  )}
                </div>
              </button>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
