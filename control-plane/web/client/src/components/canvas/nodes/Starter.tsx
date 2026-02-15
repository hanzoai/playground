/**
 * Starter Node
 *
 * Entry point for adding a new bot to the canvas.
 * Click to launch a bot (cloud) or connect one (local).
 */

import { type NodeProps } from '@xyflow/react';
import { cn } from '@/lib/utils';

export function StarterNodeComponent({ selected }: NodeProps) {
  return (
    <div
      className={cn(
        'flex items-center gap-3 rounded-xl border-2 border-dashed px-6 py-5',
        'bg-card/50 text-muted-foreground transition-all cursor-pointer touch-manipulation',
        'hover:border-primary/50 hover:text-foreground hover:bg-card/80 hover:shadow-md',
        'active:scale-[0.98]',
        selected ? 'border-primary text-foreground' : 'border-border/50',
      )}
    >
      <span className="text-2xl">+</span>
      <div>
        <div className="text-sm font-medium">Add a bot</div>
        <div className="text-[11px] text-muted-foreground">Launch cloud or connect local</div>
      </div>
    </div>
  );
}
