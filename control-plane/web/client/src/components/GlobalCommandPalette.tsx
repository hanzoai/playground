/**
 * GlobalCommandPalette
 *
 * Cmd+K command palette accessible from any page.
 * Infrastructure-oriented: navigate, deploy, restart, view logs.
 */

import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useBotStore } from '@/stores/botStore';
import { cn } from '@/lib/utils';

interface Command {
  id: string;
  label: string;
  description?: string;
  icon: string;
  section: string;
  action: () => void;
  keywords?: string[];
}

export function GlobalCommandPalette() {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const navigate = useNavigate();
  const location = useLocation();

  const agents = useBotStore((s) => s.agents);

  // Build commands
  const commands = useMemo<Command[]>(() => {
    const cmds: Command[] = [];

    // Navigation commands
    const nav = [
      { id: 'nav-bots',          label: 'My Bots',        icon: '🖥️', href: '/nodes',          desc: 'Running bots and connected nodes' },
      { id: 'nav-playground',    label: 'Playground',     icon: '🤖', href: '/playground',     desc: 'Visual canvas' },
      { id: 'nav-control-plane', label: 'Control Plane',  icon: '🎛️', href: '/bots/all',      desc: 'Bot orchestration' },
      { id: 'nav-executions',    label: 'Executions',     icon: '▶️', href: '/executions',     desc: 'Execution history' },
      { id: 'nav-metrics',       label: 'Metrics',        icon: '📊', href: '/metrics',        desc: 'System overview' },
      { id: 'nav-settings',      label: 'Settings',       icon: '⚙️', href: '/settings',       desc: 'Connection config' },
    ];

    for (const item of nav) {
      cmds.push({
        id: item.id,
        label: item.label,
        description: item.desc,
        icon: item.icon,
        section: 'Navigate',
        action: () => navigate(item.href),
        keywords: [item.label.toLowerCase(), item.desc.toLowerCase()],
      });
    }

    // Bot commands
    for (const [id, agent] of agents) {
      cmds.push({
        id: `bot-${id}`,
        label: agent.name,
        description: `${agent.status} bot`,
        icon: agent.emoji ?? '🤖',
        section: 'Bots',
        action: () => navigate('/playground'),
        keywords: [agent.name.toLowerCase(), id.toLowerCase()],
      });
    }

    // Infrastructure actions
    cmds.push({
      id: 'action-register',
      label: 'Register Local Bot',
      description: 'Connect a local machine as a bot node',
      icon: '+',
      section: 'Actions',
      action: () => navigate('/nodes'),
      keywords: ['register', 'local', 'bot', 'connect', 'add'],
    });

    cmds.push({
      id: 'action-deploy-cloud',
      label: 'Deploy Cloud Bot',
      description: 'Provision a cloud bot',
      icon: '☁️',
      section: 'Actions',
      action: () => navigate('/nodes'),
      keywords: ['deploy', 'cloud', 'provision', 'launch'],
    });

    return cmds;
  }, [agents, navigate]);

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

  // Group by section
  const grouped = useMemo(() => {
    const sections = new Map<string, Command[]>();
    for (const cmd of filtered) {
      const list = sections.get(cmd.section) ?? [];
      list.push(cmd);
      sections.set(cmd.section, list);
    }
    return sections;
  }, [filtered]);

  // Clamp selection
  useEffect(() => { setSelectedIndex(0); }, [query]);

  // Open/close with Cmd+K — only if no canvas palette is active
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        // Don't hijack on playground — it has its own palette
        if (location.pathname === '/playground') return;
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
  }, [location.pathname]);

  // Focus input on open
  useEffect(() => {
    if (open) requestAnimationFrame(() => inputRef.current?.focus());
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
        if (cmd) { cmd.action(); setOpen(false); }
      }
    },
    [filtered, selectedIndex]
  );

  if (!open) return null;

  let flatIndex = 0;

  return (
    <div className="fixed inset-0 z-[200] flex items-start justify-center pt-[15vh]">
      <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={() => setOpen(false)} />
      <div className="relative w-[90vw] max-w-lg rounded-xl border border-border/60 bg-card shadow-2xl overflow-hidden">
        {/* Search */}
        <div className="flex items-center gap-2 border-b border-border/40 px-4 py-3">
          <span className="text-muted-foreground text-sm">⌘K</span>
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Navigate, deploy, search..."
            className="flex-1 bg-transparent text-sm text-foreground placeholder:text-muted-foreground/60 outline-none"
          />
          <kbd className="text-[10px] text-muted-foreground/60 border border-border/40 rounded px-1.5 py-0.5">esc</kbd>
        </div>

        {/* Results */}
        <div className="max-h-[400px] overflow-y-auto py-1">
          {filtered.length === 0 ? (
            <div className="px-4 py-6 text-center text-sm text-muted-foreground">No results</div>
          ) : (
            Array.from(grouped.entries()).map(([section, cmds]) => (
              <div key={section}>
                <div className="px-4 pt-2 pb-1 text-[10px] uppercase tracking-wider text-muted-foreground/60 font-medium">
                  {section}
                </div>
                {cmds.map((cmd) => {
                  const idx = flatIndex++;
                  return (
                    <button
                      key={cmd.id}
                      type="button"
                      onClick={() => { cmd.action(); setOpen(false); }}
                      className={cn(
                        'flex items-center gap-3 w-full px-4 py-2 text-left transition-colors',
                        idx === selectedIndex
                          ? 'bg-primary/10 text-foreground'
                          : 'text-muted-foreground hover:text-foreground hover:bg-accent/30',
                      )}
                    >
                      <span className="text-sm shrink-0 w-5 text-center">{cmd.icon}</span>
                      <div className="flex-1 min-w-0">
                        <div className="text-sm font-medium truncate">{cmd.label}</div>
                        {cmd.description && (
                          <div className="text-[10px] text-muted-foreground truncate">{cmd.description}</div>
                        )}
                      </div>
                    </button>
                  );
                })}
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
