/**
 * VoiceSettings
 *
 * Toggle cards for voice input (STT) and voice output (TTS).
 * Used in onboarding and settings page.
 */

import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';

interface VoiceSettingsProps {
  voiceInputEnabled: boolean;
  voiceOutputEnabled: boolean;
  onVoiceInputChange: (enabled: boolean) => void;
  onVoiceOutputChange: (enabled: boolean) => void;
}

const STT_SUPPORTED =
  typeof window !== 'undefined' &&
  ('SpeechRecognition' in window || 'webkitSpeechRecognition' in window);

const TTS_SUPPORTED =
  typeof window !== 'undefined' && 'speechSynthesis' in window;

export function VoiceSettings({
  voiceInputEnabled,
  voiceOutputEnabled,
  onVoiceInputChange,
  onVoiceOutputChange,
}: VoiceSettingsProps) {
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between rounded-lg border border-border/50 bg-background/50 px-4 py-3">
        <div className="flex items-center gap-3">
          <span className="text-base">🎤</span>
          <div>
            <Label htmlFor="voice-input" className="text-xs font-medium">
              Voice input
            </Label>
            <p className="text-[10px] text-muted-foreground">
              {STT_SUPPORTED ? 'Speak to your agent' : 'Not supported in this browser'}
            </p>
          </div>
        </div>
        <Switch
          id="voice-input"
          checked={voiceInputEnabled}
          onCheckedChange={onVoiceInputChange}
          disabled={!STT_SUPPORTED}
        />
      </div>

      <div className="flex items-center justify-between rounded-lg border border-border/50 bg-background/50 px-4 py-3">
        <div className="flex items-center gap-3">
          <span className="text-base">🔊</span>
          <div>
            <Label htmlFor="voice-output" className="text-xs font-medium">
              Voice output
            </Label>
            <p className="text-[10px] text-muted-foreground">
              {TTS_SUPPORTED ? 'Agent reads responses aloud' : 'Not supported in this browser'}
            </p>
          </div>
        </div>
        <Switch
          id="voice-output"
          checked={voiceOutputEnabled}
          onCheckedChange={onVoiceOutputChange}
          disabled={!TTS_SUPPORTED}
        />
      </div>
    </div>
  );
}
