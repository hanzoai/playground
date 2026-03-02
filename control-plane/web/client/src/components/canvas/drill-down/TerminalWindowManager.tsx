/**
 * TerminalWindowManager
 *
 * iTerm-style tabbed + split-pane terminal orchestrator.
 * Mounts the tab bar, renders the active tab's pane tree,
 * and activates keyboard shortcuts.
 */

import { useEffect } from 'react';
import { cn } from '@/lib/utils';
import { useTerminalManagerStore } from '@/stores/terminalManagerStore';
import { useTerminalShortcuts } from '@/hooks/useTerminalShortcuts';
import { TerminalTabBar } from './TerminalTabBar';
import { PaneTreeRenderer } from './PaneTreeRenderer';
import type { NodeReadiness } from './TerminalPanel';

interface TerminalWindowManagerProps {
  agentId: string;
  nodeStatus?: NodeReadiness;
  className?: string;
  /** Whether the terminal tab is currently visible (enables shortcuts). */
  active?: boolean;
}

export function TerminalWindowManager({
  agentId,
  nodeStatus,
  className,
  active = true,
}: TerminalWindowManagerProps) {
  const tabs = useTerminalManagerStore((s) => s.tabs);
  const activeTabId = useTerminalManagerStore((s) => s.activeTabId);
  const init = useTerminalManagerStore((s) => s.init);

  // Initialize store for this node
  useEffect(() => {
    init(agentId);
  }, [agentId, init]);

  // Keyboard shortcuts (only when terminal tab is visible)
  useTerminalShortcuts(active);

  const activeTab = tabs.find((t) => t.id === activeTabId);

  if (!activeTab) return null;

  return (
    <div className={cn('flex flex-col h-full w-full overflow-hidden', className)}>
      <TerminalTabBar />
      <div className="flex-1 min-h-0 overflow-hidden">
        {tabs.map((tab) => (
          <div
            key={tab.id}
            className={cn('h-full w-full', tab.id === activeTabId ? 'block' : 'hidden')}
          >
            <PaneTreeRenderer
              node={tab.root}
              agentId={agentId}
              nodeStatus={nodeStatus}
            />
          </div>
        ))}
      </div>
    </div>
  );
}
