/**
 * TerminalTabBar
 *
 * Dark tab strip for the terminal window manager.
 * Shows tab labels, close buttons, and a "+" button to add new tabs.
 */

import { cn } from '@/lib/utils';
import { useTerminalManagerStore } from '@/stores/terminalManagerStore';

export function TerminalTabBar() {
  const tabs = useTerminalManagerStore((s) => s.tabs);
  const activeTabId = useTerminalManagerStore((s) => s.activeTabId);
  const setActiveTab = useTerminalManagerStore((s) => s.setActiveTab);
  const addTab = useTerminalManagerStore((s) => s.addTab);
  const closeTab = useTerminalManagerStore((s) => s.closeTab);

  return (
    <div className="flex items-center h-9 sm:h-8 bg-[#161b22] border-b border-[#30363d] overflow-x-auto scrollbar-none select-none">
      {tabs.map((tab) => {
        const isActive = tab.id === activeTabId;
        return (
          <button
            key={tab.id}
            className={cn(
              'group relative flex items-center gap-1 sm:gap-1.5 px-2.5 sm:px-3 h-full text-[11px] font-medium whitespace-nowrap transition-colors',
              isActive
                ? 'bg-[#0d1117] text-zinc-200 border-b border-cyan-500'
                : 'text-zinc-500 hover:text-zinc-300 hover:bg-[#1c2128]',
            )}
            onClick={() => setActiveTab(tab.id)}
          >
            <span>{tab.label}</span>
            {/* Close button — only show if more than one tab */}
            {tabs.length > 1 && (
              <span
                className={cn(
                  'ml-1 inline-flex items-center justify-center w-4 h-4 rounded-sm text-[10px] leading-none',
                  'opacity-0 group-hover:opacity-100 hover:bg-zinc-600/40 transition-opacity',
                  isActive && 'opacity-60',
                )}
                onClick={(e) => {
                  e.stopPropagation();
                  closeTab(tab.id);
                }}
                title="Close tab"
              >
                x
              </span>
            )}
          </button>
        );
      })}

      {/* Add tab button */}
      <button
        className="flex items-center justify-center w-8 sm:w-7 h-full text-zinc-500 hover:text-zinc-300 hover:bg-[#1c2128] transition-colors text-sm"
        onClick={addTab}
        title="New terminal tab (Cmd+T)"
      >
        +
      </button>
    </div>
  );
}
