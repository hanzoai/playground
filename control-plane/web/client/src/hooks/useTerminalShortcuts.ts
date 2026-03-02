/**
 * useTerminalShortcuts
 *
 * Keyboard handler for iTerm-style terminal shortcuts.
 * Listens on document with capture: true so it fires before xterm.js.
 * Cmd-modified keys don't conflict with xterm's onData (printable chars only).
 *
 * | Shortcut       | Action                           |
 * |----------------|----------------------------------|
 * | Cmd+T          | New terminal tab                 |
 * | Cmd+D          | Split focused pane horizontally  |
 * | Cmd+Opt+D      | Split focused pane vertically    |
 * | Cmd+W          | Close focused pane               |
 * | Cmd+Shift+[    | Previous tab                     |
 * | Cmd+Shift+]    | Next tab                         |
 */

import { useEffect } from 'react';
import { useTerminalManagerStore } from '@/stores/terminalManagerStore';

export function useTerminalShortcuts(enabled: boolean) {
  useEffect(() => {
    if (!enabled) return;

    const handler = (e: KeyboardEvent) => {
      // Only handle Cmd (Mac) or Ctrl (non-Mac) modified keys
      const mod = e.metaKey || e.ctrlKey;
      if (!mod) return;

      const store = useTerminalManagerStore.getState();

      // Cmd+T — new tab
      if (e.key === 't' && !e.shiftKey && !e.altKey) {
        e.preventDefault();
        e.stopPropagation();
        store.addTab();
        return;
      }

      // Cmd+D — split horizontally (side by side)
      if (e.key === 'd' && !e.shiftKey && !e.altKey) {
        e.preventDefault();
        e.stopPropagation();
        store.splitPane(store.focusedPaneId, 'horizontal');
        return;
      }

      // Cmd+Opt+D — split vertically (top/bottom)
      if (e.key === 'd' && !e.shiftKey && e.altKey) {
        e.preventDefault();
        e.stopPropagation();
        store.splitPane(store.focusedPaneId, 'vertical');
        return;
      }

      // Cmd+W — close focused pane
      if (e.key === 'w' && !e.shiftKey && !e.altKey) {
        e.preventDefault();
        e.stopPropagation();
        store.closePane(store.focusedPaneId);
        return;
      }

      // Cmd+Shift+[ — previous tab
      if (e.key === '[' && e.shiftKey && !e.altKey) {
        e.preventDefault();
        e.stopPropagation();
        store.prevTab();
        return;
      }

      // Cmd+Shift+] — next tab
      if (e.key === ']' && e.shiftKey && !e.altKey) {
        e.preventDefault();
        e.stopPropagation();
        store.nextTab();
        return;
      }
    };

    // Capture phase so we fire before xterm's event handlers
    document.addEventListener('keydown', handler, true);
    return () => document.removeEventListener('keydown', handler, true);
  }, [enabled]);
}
