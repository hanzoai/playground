/**
 * Team Node
 *
 * Group node representing a team of bots.
 * Shows team name, emoji, and bot count.
 */

import { Handle, Position, type NodeProps } from '@xyflow/react';
import type { Team } from '@/types/canvas';
import { cn } from '@/lib/utils';

export function TeamNodeComponent({ data, selected }: NodeProps) {
  const team = data as unknown as Team;

  return (
    <div
      className={cn(
        'rounded-xl border bg-card/80 px-4 py-3 shadow-md backdrop-blur transition-all touch-manipulation',
        selected ? 'border-primary shadow-primary/20' : 'border-border/40',
      )}
    >
      <Handle type="target" position={Position.Top} className="!w-2 !h-2 !bg-border" />
      <Handle type="source" position={Position.Bottom} className="!w-2 !h-2 !bg-border" />

      <div className="flex items-center gap-2">
        <span className="text-lg">{team.emoji}</span>
        <div className="min-w-0">
          <div className="text-sm font-medium truncate">{team.name}</div>
          <div className="text-[10px] text-muted-foreground">
            {team.botIds?.length ?? 0} bots
          </div>
        </div>
      </div>
    </div>
  );
}
