/**
 * Preferences Store (Zustand)
 *
 * Manages user notification sound and voice I/O preferences.
 * Reads from localStorage on init, syncs to backend on change.
 */

import { create } from 'zustand';
import type { SoundName } from '@/services/audioService';
import { getPreferences, putPreferences } from '@/services/preferencesApi';
import type { UserPreferencesDTO } from '@/services/preferencesApi';

const STORAGE_KEY = 'hanzo_user_preferences';

export interface UserPreferences {
  onboardingComplete: boolean;
  notificationSound: SoundName;
  notificationVolume: number;
  soundOnTaskComplete: boolean;
  soundOnApprovalNeeded: boolean;
  voiceInputEnabled: boolean;
  voiceOutputEnabled: boolean;
  voiceOutputVoice: string;
}

interface PreferencesActions {
  setNotificationSound: (sound: SoundName) => void;
  setNotificationVolume: (volume: number) => void;
  setSoundOnTaskComplete: (enabled: boolean) => void;
  setSoundOnApprovalNeeded: (enabled: boolean) => void;
  setVoiceInputEnabled: (enabled: boolean) => void;
  setVoiceOutputEnabled: (enabled: boolean) => void;
  setVoiceOutputVoice: (voice: string) => void;
  setOnboardingComplete: (complete: boolean) => void;
  /** Bulk update from onboarding. */
  applyOnboarding: (partial: Partial<UserPreferences>) => void;
  /** Pull from backend (called after auth). */
  syncFromBackend: () => Promise<void>;
}

type PreferencesState = UserPreferences & PreferencesActions;

const DEFAULTS: UserPreferences = {
  onboardingComplete: false,
  notificationSound: 'chaching',
  notificationVolume: 0.7,
  soundOnTaskComplete: true,
  soundOnApprovalNeeded: true,
  voiceInputEnabled: false,
  voiceOutputEnabled: false,
  voiceOutputVoice: '',
};

function loadFromStorage(): Partial<UserPreferences> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return {};
    return JSON.parse(raw) as Partial<UserPreferences>;
  } catch {
    return {};
  }
}

function persistToStorage(prefs: UserPreferences) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs));
  } catch { /* ok */ }
}

function toDTO(prefs: UserPreferences): UserPreferencesDTO {
  return {
    notification_sound: prefs.notificationSound,
    notification_volume: prefs.notificationVolume,
    sound_on_task_complete: prefs.soundOnTaskComplete,
    sound_on_approval_needed: prefs.soundOnApprovalNeeded,
    voice_input_enabled: prefs.voiceInputEnabled,
    voice_output_enabled: prefs.voiceOutputEnabled,
    voice_output_voice: prefs.voiceOutputVoice,
    onboarding_complete: prefs.onboardingComplete,
  };
}

function fromDTO(dto: UserPreferencesDTO): UserPreferences {
  return {
    notificationSound: dto.notification_sound || DEFAULTS.notificationSound,
    notificationVolume: dto.notification_volume ?? DEFAULTS.notificationVolume,
    soundOnTaskComplete: dto.sound_on_task_complete ?? DEFAULTS.soundOnTaskComplete,
    soundOnApprovalNeeded: dto.sound_on_approval_needed ?? DEFAULTS.soundOnApprovalNeeded,
    voiceInputEnabled: dto.voice_input_enabled ?? DEFAULTS.voiceInputEnabled,
    voiceOutputEnabled: dto.voice_output_enabled ?? DEFAULTS.voiceOutputEnabled,
    voiceOutputVoice: dto.voice_output_voice ?? DEFAULTS.voiceOutputVoice,
    onboardingComplete: dto.onboarding_complete ?? DEFAULTS.onboardingComplete,
  };
}

let syncTimer: ReturnType<typeof setTimeout> | null = null;

/** Debounced backend sync — collapses rapid changes into one PUT. */
function debouncedSync(prefs: UserPreferences) {
  persistToStorage(prefs);
  if (syncTimer) clearTimeout(syncTimer);
  syncTimer = setTimeout(() => {
    putPreferences(toDTO(prefs)).catch(() => { /* best effort */ });
  }, 800);
}

function snapshot(state: PreferencesState): UserPreferences {
  return {
    onboardingComplete: state.onboardingComplete,
    notificationSound: state.notificationSound,
    notificationVolume: state.notificationVolume,
    soundOnTaskComplete: state.soundOnTaskComplete,
    soundOnApprovalNeeded: state.soundOnApprovalNeeded,
    voiceInputEnabled: state.voiceInputEnabled,
    voiceOutputEnabled: state.voiceOutputEnabled,
    voiceOutputVoice: state.voiceOutputVoice,
  };
}

const stored = loadFromStorage();
const initial: UserPreferences = { ...DEFAULTS, ...stored };

export const usePreferencesStore = create<PreferencesState>((set, get) => ({
  ...initial,

  setNotificationSound: (sound) => {
    set({ notificationSound: sound });
    debouncedSync(snapshot(get()));
  },
  setNotificationVolume: (volume) => {
    set({ notificationVolume: Math.max(0, Math.min(1, volume)) });
    debouncedSync(snapshot(get()));
  },
  setSoundOnTaskComplete: (enabled) => {
    set({ soundOnTaskComplete: enabled });
    debouncedSync(snapshot(get()));
  },
  setSoundOnApprovalNeeded: (enabled) => {
    set({ soundOnApprovalNeeded: enabled });
    debouncedSync(snapshot(get()));
  },
  setVoiceInputEnabled: (enabled) => {
    set({ voiceInputEnabled: enabled });
    debouncedSync(snapshot(get()));
  },
  setVoiceOutputEnabled: (enabled) => {
    set({ voiceOutputEnabled: enabled });
    debouncedSync(snapshot(get()));
  },
  setVoiceOutputVoice: (voice) => {
    set({ voiceOutputVoice: voice });
    debouncedSync(snapshot(get()));
  },
  setOnboardingComplete: (complete) => {
    set({ onboardingComplete: complete });
    debouncedSync(snapshot(get()));
  },

  applyOnboarding: (partial) => {
    const merged = { ...snapshot(get()), ...partial, onboardingComplete: true };
    set(merged);
    debouncedSync(merged);
  },

  syncFromBackend: async () => {
    const dto = await getPreferences();
    if (dto) {
      const prefs = fromDTO(dto);
      set(prefs);
      persistToStorage(prefs);
    }
  },
}));
