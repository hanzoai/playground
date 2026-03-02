/**
 * Terminal Manager Store (Zustand)
 *
 * Manages tabbed + split-pane terminal layout tree.
 * Pure tree-manipulation functions: split, close, resize, add/close tab.
 * Persists layout to localStorage keyed by nodeId.
 */

import { create } from 'zustand';
import type {
  PaneNode,
  SplitBranch,
  TerminalLeaf,
  TerminalManagerState,
  TerminalTab,
} from '@/types/terminal-manager';

const STORAGE_PREFIX = 'terminal_layout:';

function uid(): string {
  return `p_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

function makeLeaf(agentId: string): TerminalLeaf {
  const id = uid();
  return {
    type: 'terminal',
    id,
    sessionKey: `agent:${agentId}:term-${id}`,
  };
}

function makeTab(agentId: string, index: number): TerminalTab {
  const leaf = makeLeaf(agentId);
  return {
    id: uid(),
    label: `Terminal ${index}`,
    root: leaf,
  };
}

/** Collect all leaf IDs in a pane tree. */
function collectLeafIds(node: PaneNode): string[] {
  if (node.type === 'terminal') return [node.id];
  return [...collectLeafIds(node.children[0]), ...collectLeafIds(node.children[1])];
}

/** Find a node by ID in the tree. */
function findNode(root: PaneNode, id: string): PaneNode | null {
  if (root.id === id) return root;
  if (root.type === 'split') {
    return findNode(root.children[0], id) ?? findNode(root.children[1], id);
  }
  return null;
}

/** Find the first leaf in a tree (for focus fallback). */
function firstLeaf(node: PaneNode): TerminalLeaf {
  if (node.type === 'terminal') return node;
  return firstLeaf(node.children[0]);
}

/**
 * Replace a target node in the tree with a replacement.
 * Returns a new tree (immutable).
 */
function replaceInTree(root: PaneNode, targetId: string, replacement: PaneNode): PaneNode {
  if (root.id === targetId) return replacement;
  if (root.type === 'split') {
    return {
      ...root,
      children: [
        replaceInTree(root.children[0], targetId, replacement),
        replaceInTree(root.children[1], targetId, replacement),
      ],
    };
  }
  return root;
}

/**
 * Remove a target leaf from the tree, promoting its sibling.
 * Returns the new root, or null if the root itself was the target.
 */
function removeFromTree(root: PaneNode, targetId: string): PaneNode | null {
  if (root.id === targetId) return null;
  if (root.type === 'split') {
    if (root.children[0].id === targetId) return root.children[1];
    if (root.children[1].id === targetId) return root.children[0];
    const left = removeFromTree(root.children[0], targetId);
    if (left !== root.children[0]) {
      return left === null
        ? root.children[1]
        : { ...root, children: [left, root.children[1]] };
    }
    const right = removeFromTree(root.children[1], targetId);
    if (right !== root.children[1]) {
      return right === null
        ? root.children[0]
        : { ...root, children: [root.children[0], right] };
    }
  }
  return root;
}

// ---------------------------------------------------------------------------
// Store
// ---------------------------------------------------------------------------

interface TerminalManagerStore extends TerminalManagerState {
  agentId: string;

  /** Initialize store for a given node. Restores from localStorage or creates default. */
  init: (agentId: string) => void;

  /** Split the focused pane. */
  splitPane: (targetId: string, orientation: 'horizontal' | 'vertical') => void;

  /** Close a pane. If it's the last pane in a tab, close the tab. Always keep >= 1 tab. */
  closePane: (targetId: string) => void;

  /** Update split ratio on drag. */
  updateRatio: (splitId: string, ratio: number) => void;

  /** Add a new tab with a single terminal. */
  addTab: () => void;

  /** Close a tab. Switch to adjacent. Always keep >= 1 tab. */
  closeTab: (tabId: string) => void;

  /** Switch active tab. */
  setActiveTab: (tabId: string) => void;

  /** Set focused pane (click handler). */
  setFocusedPane: (id: string) => void;

  /** Cycle to previous tab. */
  prevTab: () => void;

  /** Cycle to next tab. */
  nextTab: () => void;

  /** Persist to localStorage. */
  persist: () => void;
}

export const useTerminalManagerStore = create<TerminalManagerStore>((set, get) => ({
  agentId: '',
  tabs: [],
  activeTabId: '',
  focusedPaneId: '',

  init: (agentId: string) => {
    const key = `${STORAGE_PREFIX}${agentId}`;
    try {
      const saved = localStorage.getItem(key);
      if (saved) {
        const parsed = JSON.parse(saved) as TerminalManagerState;
        if (parsed.tabs?.length > 0) {
          set({ ...parsed, agentId });
          return;
        }
      }
    } catch {
      // Corrupted data — fall through to default
    }

    const tab = makeTab(agentId, 1);
    const leaf = firstLeaf(tab.root);
    set({
      agentId,
      tabs: [tab],
      activeTabId: tab.id,
      focusedPaneId: leaf.id,
    });
  },

  splitPane: (targetId, orientation) => {
    const { tabs, activeTabId, agentId } = get();
    const tabIdx = tabs.findIndex((t) => t.id === activeTabId);
    if (tabIdx < 0) return;
    const tab = tabs[tabIdx];

    const target = findNode(tab.root, targetId);
    if (!target || target.type !== 'terminal') return;

    const newLeaf = makeLeaf(agentId);
    const split: SplitBranch = {
      type: 'split',
      id: uid(),
      orientation,
      ratio: 50,
      children: [target, newLeaf],
    };

    const newRoot = replaceInTree(tab.root, targetId, split);
    const newTabs = [...tabs];
    newTabs[tabIdx] = { ...tab, root: newRoot };

    set({ tabs: newTabs, focusedPaneId: newLeaf.id });
    get().persist();
  },

  closePane: (targetId) => {
    const { tabs, activeTabId, agentId } = get();
    const tabIdx = tabs.findIndex((t) => t.id === activeTabId);
    if (tabIdx < 0) return;
    const tab = tabs[tabIdx];

    // If this is the only leaf in the only tab, do nothing
    const leaves = collectLeafIds(tab.root);
    if (leaves.length <= 1 && tabs.length <= 1) return;

    // If this is the only leaf in this tab, close the tab instead
    if (leaves.length <= 1) {
      get().closeTab(tab.id);
      return;
    }

    const newRoot = removeFromTree(tab.root, targetId);
    if (!newRoot) return;

    const newTabs = [...tabs];
    newTabs[tabIdx] = { ...tab, root: newRoot };

    // If focused pane was removed, focus the first leaf
    const focusedPaneId =
      findNode(newRoot, get().focusedPaneId) ? get().focusedPaneId : firstLeaf(newRoot).id;

    set({ tabs: newTabs, focusedPaneId });
    get().persist();
  },

  updateRatio: (splitId, ratio) => {
    const { tabs, activeTabId } = get();
    const tabIdx = tabs.findIndex((t) => t.id === activeTabId);
    if (tabIdx < 0) return;
    const tab = tabs[tabIdx];

    const updateNode = (node: PaneNode): PaneNode => {
      if (node.id === splitId && node.type === 'split') {
        return { ...node, ratio };
      }
      if (node.type === 'split') {
        return {
          ...node,
          children: [updateNode(node.children[0]), updateNode(node.children[1])],
        };
      }
      return node;
    };

    const newRoot = updateNode(tab.root);
    const newTabs = [...tabs];
    newTabs[tabIdx] = { ...tab, root: newRoot };
    set({ tabs: newTabs });
    // Persist on mouseup (debounced in component), not every move
  },

  addTab: () => {
    const { tabs, agentId } = get();
    const tab = makeTab(agentId, tabs.length + 1);
    const leaf = firstLeaf(tab.root);
    set({
      tabs: [...tabs, tab],
      activeTabId: tab.id,
      focusedPaneId: leaf.id,
    });
    get().persist();
  },

  closeTab: (tabId) => {
    const { tabs } = get();
    if (tabs.length <= 1) return;
    const idx = tabs.findIndex((t) => t.id === tabId);
    if (idx < 0) return;

    const newTabs = tabs.filter((t) => t.id !== tabId);
    const nextIdx = Math.min(idx, newTabs.length - 1);
    const nextTab = newTabs[nextIdx];
    const leaf = firstLeaf(nextTab.root);

    set({
      tabs: newTabs,
      activeTabId: nextTab.id,
      focusedPaneId: leaf.id,
    });
    get().persist();
  },

  setActiveTab: (tabId) => {
    const { tabs } = get();
    const tab = tabs.find((t) => t.id === tabId);
    if (!tab) return;
    const leaf = firstLeaf(tab.root);
    set({ activeTabId: tabId, focusedPaneId: leaf.id });
  },

  setFocusedPane: (id) => {
    set({ focusedPaneId: id });
  },

  prevTab: () => {
    const { tabs, activeTabId } = get();
    const idx = tabs.findIndex((t) => t.id === activeTabId);
    const prev = idx > 0 ? tabs[idx - 1] : tabs[tabs.length - 1];
    get().setActiveTab(prev.id);
  },

  nextTab: () => {
    const { tabs, activeTabId } = get();
    const idx = tabs.findIndex((t) => t.id === activeTabId);
    const next = idx < tabs.length - 1 ? tabs[idx + 1] : tabs[0];
    get().setActiveTab(next.id);
  },

  persist: () => {
    const { agentId, tabs, activeTabId, focusedPaneId } = get();
    if (!agentId) return;
    const key = `${STORAGE_PREFIX}${agentId}`;
    try {
      localStorage.setItem(key, JSON.stringify({ tabs, activeTabId, focusedPaneId }));
    } catch {
      // localStorage full or unavailable
    }
  },
}));
