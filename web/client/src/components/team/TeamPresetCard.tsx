/**
 * TeamPresetCard
 *
 * Card for a single team preset showing emoji, name,
 * description, bot count, and provision button.
 */

import type { TeamPreset } from '@/types/team';
import { cn } from '@/lib/utils';

interface TeamPresetCardProps {
  preset: TeamPreset;
  onProvision: (preset: TeamPreset) => void;
  disabled?: boolean;
  className?: string;
}

export function TeamPresetCard({ preset, onProvision, disabled, className }: TeamPresetCardProps) {
  return (
    <div
      className={cn(
        'group rounded-xl border border-border/60 bg-card p-4 transition-all hover:border-primary/30 hover:shadow-md',
        disabled && 'opacity-50 pointer-events-none',
        className,
      )}
    >
      {/* Header */}
      <div className="flex items-start gap-3 mb-3">
        <span className="text-3xl">{preset.emoji}</span>
        <div className="flex-1 min-w-0">
          <h3 className="text-sm font-semibold truncate">{preset.name}</h3>
          <p className="text-xs text-muted-foreground mt-0.5 line-clamp-2">
            {preset.description}
          </p>
        </div>
      </div>

      {/* Bots */}
      <div className="flex flex-wrap gap-1 mb-3">
        {preset.bots.map((bot) => (
          <span
            key={bot.role}
            className="inline-flex items-center gap-1 rounded-full bg-accent/50 px-2 py-0.5 text-[10px] text-muted-foreground"
          >
            {bot.name}
            {bot.model && (
              <span className="text-muted-foreground/60">({bot.model})</span>
            )}
          </span>
        ))}
      </div>

      {/* Tags */}
      {preset.tags && preset.tags.length > 0 && (
        <div className="flex flex-wrap gap-1 mb-3">
          {preset.tags.map((tag) => (
            <span
              key={tag}
              className="rounded-full border border-border/40 px-1.5 py-0.5 text-[9px] text-muted-foreground/70"
            >
              {tag}
            </span>
          ))}
        </div>
      )}

      {/* Action */}
      <button
        type="button"
        onClick={() => onProvision(preset)}
        disabled={disabled}
        className={cn(
          'w-full rounded-lg py-2 text-xs font-medium transition-colors touch-manipulation',
          'bg-primary/10 text-primary hover:bg-primary/20',
        )}
      >
        Provision ({preset.bots.length} bots)
      </button>
    </div>
  );
}
