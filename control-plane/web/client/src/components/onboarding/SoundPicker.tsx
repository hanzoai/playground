/**
 * SoundPicker
 *
 * A grid of notification sound tiles. Click to preview + select.
 */

import { useCallback } from 'react';
import { audioService, ALL_SOUNDS, SOUND_LABELS } from '@/services/audioService';
import type { SoundName } from '@/services/audioService';
import { cn } from '@/lib/utils';

interface SoundPickerProps {
  selected: SoundName;
  volume: number;
  onSelect: (name: SoundName) => void;
}

const SOUND_ICONS: Record<SoundName, string> = {
  none: '\u{1F507}',
  chaching: '\u{1F4B0}',
  synth: '\u{1F916}',
  jazz: '\u{1F3B7}',
  chime: '\u266A',
  ding: '\u{1F514}',
  droplet: '\u{1F4A7}',
  pulse: '\u{1F4AB}',
  bell: '\u{1F514}',
  pop: '\u{1F4AC}',
  whoosh: '\u{1F4A8}',
  tap: '\u{1F449}',
};

export function SoundPicker({ selected, volume, onSelect }: SoundPickerProps) {
  const handleClick = useCallback(
    (name: SoundName) => {
      audioService.play(name, volume).catch(() => {});
      onSelect(name);
    },
    [volume, onSelect],
  );

  return (
    <div className="grid grid-cols-4 gap-2">
      {ALL_SOUNDS.map((name) => (
        <button
          key={name}
          type="button"
          onClick={() => handleClick(name)}
          className={cn(
            'flex flex-col items-center gap-1 rounded-lg border px-3 py-2.5 text-xs transition-all',
            'hover:bg-accent/50 active:scale-95',
            selected === name
              ? 'border-primary bg-primary/10 ring-1 ring-primary/40'
              : 'border-border/50 bg-background/50',
          )}
        >
          <span className="text-base">{SOUND_ICONS[name]}</span>
          <span className="font-medium">{SOUND_LABELS[name]}</span>
        </button>
      ))}
    </div>
  );
}
