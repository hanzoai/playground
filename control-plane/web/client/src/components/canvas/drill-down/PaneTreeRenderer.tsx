/**
 * PaneTreeRenderer
 *
 * Recursively renders a PaneNode tree:
 *   - Leaf → TerminalPanel wrapped in a focus border
 *   - Split → ResizableSplitPane with two recursive children
 */

import { useCallback } from 'react';
import { cn } from '@/lib/utils';
import { ResizableSplitPane } from '@/components/ui/ResizableSplitPane';
import { TerminalPanel, type NodeReadiness } from './TerminalPanel';
import { useTerminalManagerStore } from '@/stores/terminalManagerStore';
import type { PaneNode } from '@/types/terminal-manager';

interface PaneTreeRendererProps {
  node: PaneNode;
  agentId: string;
  nodeStatus?: NodeReadiness;
}

export function PaneTreeRenderer({ node, agentId, nodeStatus }: PaneTreeRendererProps) {
  const focusedPaneId = useTerminalManagerStore((s) => s.focusedPaneId);
  const setFocusedPane = useTerminalManagerStore((s) => s.setFocusedPane);
  const updateRatio = useTerminalManagerStore((s) => s.updateRatio);
  const persist = useTerminalManagerStore((s) => s.persist);

  const handleSizeChange = useCallback(
    (splitId: string) => (ratio: number) => {
      updateRatio(splitId, ratio);
      persist();
    },
    [updateRatio, persist],
  );

  if (node.type === 'terminal') {
    const isFocused = focusedPaneId === node.id;
    return (
      <div
        className={cn(
          'h-full w-full relative rounded',
          isFocused ? 'ring-1 ring-cyan-500/60' : 'ring-1 ring-transparent',
        )}
        onMouseDown={() => setFocusedPane(node.id)}
        onTouchStart={() => setFocusedPane(node.id)}
      >
        <TerminalPanel
          agentId={agentId}
          sessionKey={node.sessionKey}
          nodeStatus={nodeStatus}
          className="h-full"
        />
      </div>
    );
  }

  // Split node
  return (
    <ResizableSplitPane
      orientation={node.orientation}
      defaultSizePercent={node.ratio}
      minSizePercent={15}
      maxSizePercent={85}
      overflowMode="hidden"
      onSizeChange={handleSizeChange(node.id)}
    >
      <PaneTreeRenderer node={node.children[0]} agentId={agentId} nodeStatus={nodeStatus} />
      <PaneTreeRenderer node={node.children[1]} agentId={agentId} nodeStatus={nodeStatus} />
    </ResizableSplitPane>
  );
}
