/**
 * PreferencesSettings
 *
 * Full preferences panel for notification sounds and voice I/O.
 * Reachable from the settings route.
 */

import { useCallback } from 'react';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import { SoundPicker } from '@/components/onboarding/SoundPicker';
import { VoiceSettings } from '@/components/onboarding/VoiceSettings';
import { usePreferencesStore } from '@/stores/preferencesStore';
import { audioService } from '@/services/audioService';

export function PreferencesSettings() {
  const sound = usePreferencesStore((s) => s.notificationSound);
  const volume = usePreferencesStore((s) => s.notificationVolume);
  const soundOnTaskComplete = usePreferencesStore((s) => s.soundOnTaskComplete);
  const soundOnApprovalNeeded = usePreferencesStore((s) => s.soundOnApprovalNeeded);
  const voiceInputEnabled = usePreferencesStore((s) => s.voiceInputEnabled);
  const voiceOutputEnabled = usePreferencesStore((s) => s.voiceOutputEnabled);

  const setSound = usePreferencesStore((s) => s.setNotificationSound);
  const setVolume = usePreferencesStore((s) => s.setNotificationVolume);
  const setSoundOnTaskComplete = usePreferencesStore((s) => s.setSoundOnTaskComplete);
  const setSoundOnApprovalNeeded = usePreferencesStore((s) => s.setSoundOnApprovalNeeded);
  const setVoiceInputEnabled = usePreferencesStore((s) => s.setVoiceInputEnabled);
  const setVoiceOutputEnabled = usePreferencesStore((s) => s.setVoiceOutputEnabled);

  const handleVolumeChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const v = Number(e.target.value);
      setVolume(v);
    },
    [setVolume],
  );

  const handleTestSound = useCallback(() => {
    audioService.play(sound, volume).catch(() => {});
  }, [sound, volume]);

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-heading-1 mb-1">Preferences</h2>
        <p className="text-body text-muted-foreground">
          Notification sounds, voice input, and voice output settings.
        </p>
      </div>

      {/* Notification Sound */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Notification Sound</CardTitle>
          <CardDescription>
            Plays when a task completes or when agent needs your approval.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <SoundPicker selected={sound} volume={volume} onSelect={setSound} />

          <div className="space-y-2">
            <Label htmlFor="pref-volume" className="text-xs font-medium">
              Volume
            </Label>
            <div className="flex items-center gap-3">
              <span className="text-xs text-muted-foreground">{volume === 0 ? '\u{1F507}' : '\u{1F508}'}</span>
              <input
                id="pref-volume"
                type="range"
                min={0}
                max={1}
                step={0.05}
                value={volume}
                onChange={handleVolumeChange}
                className="flex-1 h-1.5 accent-primary cursor-pointer"
              />
              <span className="text-xs text-muted-foreground">🔊</span>
              <button
                type="button"
                onClick={handleTestSound}
                className="text-xs text-primary hover:underline"
              >
                Test
              </button>
            </div>
          </div>

          <div className="space-y-3 pt-2">
            <div className="flex items-center justify-between">
              <Label htmlFor="sound-complete" className="text-xs">
                Sound on task complete
              </Label>
              <Switch
                id="sound-complete"
                checked={soundOnTaskComplete}
                onCheckedChange={setSoundOnTaskComplete}
              />
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="sound-approval" className="text-xs">
                Sound on approval needed
              </Label>
              <Switch
                id="sound-approval"
                checked={soundOnApprovalNeeded}
                onCheckedChange={setSoundOnApprovalNeeded}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Voice I/O */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Voice</CardTitle>
          <CardDescription>
            Speak to your agent and have responses read aloud.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <VoiceSettings
            voiceInputEnabled={voiceInputEnabled}
            voiceOutputEnabled={voiceOutputEnabled}
            onVoiceInputChange={setVoiceInputEnabled}
            onVoiceOutputChange={setVoiceOutputEnabled}
          />
        </CardContent>
      </Card>
    </div>
  );
}
