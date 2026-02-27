/**
 * PreferencesOnboarding
 *
 * Single-card modal shown on first login.
 * User picks a notification sound, adjusts volume, and optionally enables voice.
 * Sensible defaults pre-selected — just click Continue.
 */

import { useState, useCallback } from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { SoundPicker } from './SoundPicker';
import { VoiceSettings } from './VoiceSettings';
import { usePreferencesStore } from '@/stores/preferencesStore';
import { audioService } from '@/services/audioService';
import type { SoundName } from '@/services/audioService';

interface PreferencesOnboardingProps {
  onComplete: () => void;
}

export function PreferencesOnboarding({ onComplete }: PreferencesOnboardingProps) {
  const store = usePreferencesStore();

  const [sound, setSound] = useState<SoundName>(store.notificationSound);
  const [volume, setVolume] = useState(store.notificationVolume);
  const [voiceIn, setVoiceIn] = useState(store.voiceInputEnabled);
  const [voiceOut, setVoiceOut] = useState(store.voiceOutputEnabled);

  const handleContinue = useCallback(() => {
    // Ensure AudioContext is unlocked from this user gesture
    audioService.resume().catch(() => {});

    store.applyOnboarding({
      notificationSound: sound,
      notificationVolume: volume,
      voiceInputEnabled: voiceIn,
      voiceOutputEnabled: voiceOut,
    });
    onComplete();
  }, [sound, volume, voiceIn, voiceOut, store, onComplete]);

  return (
    <Dialog open modal>
      <DialogContent
        className="sm:max-w-md"
        onPointerDownOutside={(e) => e.preventDefault()}
        onEscapeKeyDown={(e) => e.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle>Set Up Your Experience</DialogTitle>
          <DialogDescription>
            Pick a notification sound and configure voice. You can change these anytime in Settings.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-5 py-2">
          {/* Sound Picker */}
          <div className="space-y-2">
            <Label className="text-xs font-medium">Notification sound</Label>
            <SoundPicker selected={sound} volume={volume} onSelect={setSound} />
          </div>

          {/* Volume Slider */}
          <div className="space-y-2">
            <Label htmlFor="volume-slider" className="text-xs font-medium">
              Volume
            </Label>
            <div className="flex items-center gap-3">
              <span className="text-xs text-muted-foreground">{volume === 0 ? '\u{1F507}' : '\u{1F508}'}</span>
              <input
                id="volume-slider"
                type="range"
                min={0}
                max={1}
                step={0.05}
                value={volume}
                onChange={(e) => setVolume(Number(e.target.value))}
                className="flex-1 h-1.5 accent-primary cursor-pointer"
              />
              <span className="text-xs text-muted-foreground">🔊</span>
            </div>
            <p className="text-[10px] text-muted-foreground">
              Plays when tasks complete and when agent needs your approval.
            </p>
          </div>

          {/* Voice Toggles */}
          <div className="space-y-2">
            <Label className="text-xs font-medium">Voice (optional)</Label>
            <VoiceSettings
              voiceInputEnabled={voiceIn}
              voiceOutputEnabled={voiceOut}
              onVoiceInputChange={setVoiceIn}
              onVoiceOutputChange={setVoiceOut}
            />
          </div>
        </div>

        <DialogFooter>
          <Button onClick={handleContinue} className="w-full">
            Continue
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
