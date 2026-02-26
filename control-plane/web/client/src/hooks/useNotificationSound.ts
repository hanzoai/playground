/**
 * useNotificationSound
 *
 * Subscribes to gateway events and plays the user's chosen notification sound
 * when a task completes or when approval is needed.
 */

import { useEffect } from 'react';
import { gateway } from '@/services/gatewayClient';
import { audioService } from '@/services/audioService';
import { usePreferencesStore } from '@/stores/preferencesStore';
import type { ChatEvent } from '@/types/gateway';

export function useNotificationSound() {
  const sound = usePreferencesStore((s) => s.notificationSound);
  const volume = usePreferencesStore((s) => s.notificationVolume);
  const onComplete = usePreferencesStore((s) => s.soundOnTaskComplete);
  const onApproval = usePreferencesStore((s) => s.soundOnApprovalNeeded);

  useEffect(() => {
    const unsubs: Array<() => void> = [];

    if (onComplete) {
      const unsub = gateway.on('chat', (payload) => {
        const event = payload as ChatEvent;
        if (event.state === 'final' || event.state === 'error') {
          audioService.play(sound, volume).catch(() => {});
        }
      });
      unsubs.push(unsub);
    }

    if (onApproval) {
      const unsub = gateway.on('exec.approval.requested', () => {
        audioService.play(sound, volume).catch(() => {});
      });
      unsubs.push(unsub);
    }

    return () => unsubs.forEach((fn) => fn());
  }, [sound, volume, onComplete, onApproval]);
}
