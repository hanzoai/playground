/**
 * TeamGrid
 *
 * Grid of team preset cards. Responsive: 1 col mobile, 2 tablet, 3 desktop.
 */

import type { TeamPreset } from '@/types/team';
import { TeamPresetCard } from './TeamPresetCard';

interface TeamGridProps {
  presets: TeamPreset[];
  onProvision: (preset: TeamPreset) => void;
  disabled?: boolean;
}

export function TeamGrid({ presets, onProvision, disabled }: TeamGridProps) {
  if (presets.length === 0) {
    return (
      <div className="flex items-center justify-center py-16 text-center">
        <div>
          <div className="text-3xl mb-2">🤝</div>
          <h3 className="text-sm font-medium mb-1">No Team Presets</h3>
          <p className="text-xs text-muted-foreground max-w-xs">
            Team presets will appear here once configured in the gateway.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      {presets.map((preset) => (
        <TeamPresetCard
          key={preset.id}
          preset={preset}
          onProvision={onProvision}
          disabled={disabled}
        />
      ))}
    </div>
  );
}
