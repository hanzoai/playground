/**
 * Terminal Window Manager Types
 *
 * Data model for iTerm-style tabbed + split-pane terminal layout.
 * PaneNode is a recursive tree: leaves are terminals, branches are splits.
 */

/** Leaf node — a single terminal session. */
export interface TerminalLeaf {
  type: 'terminal';
  id: string;
  sessionKey: string;
}

/** Branch node — a horizontal or vertical split with two children. */
export interface SplitBranch {
  type: 'split';
  id: string;
  orientation: 'horizontal' | 'vertical';
  /** 0–100 percentage allocated to the first child. */
  ratio: number;
  children: [PaneNode, PaneNode];
}

export type PaneNode = TerminalLeaf | SplitBranch;

export interface TerminalTab {
  id: string;
  label: string;
  root: PaneNode;
}

export interface TerminalManagerState {
  tabs: TerminalTab[];
  activeTabId: string;
  focusedPaneId: string;
}
