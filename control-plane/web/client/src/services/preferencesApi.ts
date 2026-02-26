/**
 * Preferences API
 *
 * Client for the user preferences backend endpoint.
 * GET /v1/preferences — fetch current preferences
 * PUT /v1/preferences — upsert preferences
 */

import type { SoundName } from './audioService';
import { getGlobalApiKey } from './api';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/v1';

export interface UserPreferencesDTO {
  notification_sound: SoundName;
  notification_volume: number;
  sound_on_task_complete: boolean;
  sound_on_approval_needed: boolean;
  voice_input_enabled: boolean;
  voice_output_enabled: boolean;
  voice_output_voice: string;
  onboarding_complete: boolean;
}

function authHeaders(): Headers {
  const headers = new Headers({ 'Content-Type': 'application/json' });
  const apiKey = getGlobalApiKey();
  if (apiKey) {
    headers.set('X-API-Key', apiKey);
  }
  return headers;
}

export async function getPreferences(): Promise<UserPreferencesDTO | null> {
  try {
    const res = await fetch(`${API_BASE_URL}/preferences`, { headers: authHeaders() });
    if (res.status === 404) return null;
    if (!res.ok) throw new Error(`GET /preferences failed: ${res.status}`);
    return res.json() as Promise<UserPreferencesDTO>;
  } catch {
    return null;
  }
}

export async function putPreferences(prefs: UserPreferencesDTO): Promise<void> {
  const res = await fetch(`${API_BASE_URL}/preferences`, {
    method: 'PUT',
    headers: authHeaders(),
    body: JSON.stringify(prefs),
  });
  if (!res.ok) {
    throw new Error(`PUT /preferences failed: ${res.status}`);
  }
}
