/**
 * ViewSwitcher
 *
 * Tab bar to switch between bot views: terminal, desktop, chat.
 * Renders the active view panel below the tabs.
 */

import { useState } from 'react';
import { TerminalPanel } from './TerminalPanel';
import { ChatPanel } from './ChatPanel';
import { DesktopPanel } from './DesktopPanel';
import { cn } from '@/lib/utils';

type ViewTab = 'terminal' | 'desktop' | 'chat';

interface ViewSwitcherProps {
  agentId: string;
  sessionKey?: string;
  nodeEndpoint?: string;
  os?: 'linux' | 'windows' | 'macos';
  /** Which views are available for this bot */
  availableViews?: ViewTab[];
  className?: string;
}

const viewLabels: Record<ViewTab, string> = {
  terminal: 'Terminal',
  desktop: 'Desktop',
  chat: 'Chat',
};

export function ViewSwitcher({
  agentId,
  sessionKey,
  nodeEndpoint,
  os = 'linux',
  availableViews = ['terminal', 'chat'],
  className,
}: ViewSwitcherProps) {
  const [activeView, setActiveView] = useState<ViewTab>(availableViews[0] ?? 'terminal');

  // Ensure desktop is only available when node endpoint is provided
  const views = availableViews.filter(
    (v) => v !== 'desktop' || nodeEndpoint
  );

  return (
    <div className={cn('flex flex-col h-full', className)}>
      {/* Tab bar */}
      <div className="flex border-b border-border/40 bg-card/50 px-1">
        {views.map((view) => (
          <button
            key={view}
            onClick={() => setActiveView(view)}
            className={cn(
              'px-3 py-1.5 text-xs font-medium transition-colors relative',
              activeView === view
                ? 'text-foreground'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            {viewLabels[view]}
            {activeView === view && (
              <div className="absolute bottom-0 left-1 right-1 h-0.5 bg-primary rounded-t" />
            )}
          </button>
        ))}
      </div>

      {/* View panel */}
      <div className="flex-1 min-h-0">
        {activeView === 'terminal' && (
          <TerminalPanel agentId={agentId} sessionKey={sessionKey} className="h-full" />
        )}
        {activeView === 'chat' && (
          <ChatPanel agentId={agentId} sessionKey={sessionKey} className="h-full" />
        )}
        {activeView === 'desktop' && nodeEndpoint && (
          <DesktopPanel nodeEndpoint={nodeEndpoint} os={os} className="h-full" />
        )}
      </div>
    </div>
  );
}
