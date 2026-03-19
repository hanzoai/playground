/**
 * Cloud ASR Service
 *
 * Uses Hanzo Cloud API (api.hanzo.ai) for server-side speech-to-text.
 * Works on all browsers that support MediaRecorder (which is all modern browsers).
 */

const ASR_ENDPOINT =
  import.meta.env.VITE_ASR_ENDPOINT || 'https://api.hanzo.ai/v1/audio/transcriptions';

export interface AsrResult {
  text: string;
  language?: string;
  duration?: number;
}

/**
 * Transcribe an audio blob via the cloud API.
 */
export async function transcribeAudio(
  audioBlob: Blob,
  language?: string,
): Promise<AsrResult> {
  const formData = new FormData();
  formData.append('file', audioBlob, 'recording.webm');
  formData.append('model', 'whisper-1');
  if (language) formData.append('language', language);

  const apiKey =
    localStorage.getItem('hanzo_api_key') ||
    import.meta.env.VITE_HANZO_API_KEY ||
    '';

  const response = await fetch(ASR_ENDPOINT, {
    method: 'POST',
    headers: {
      ...(apiKey ? { Authorization: `Bearer ${apiKey}` } : {}),
    },
    body: formData,
  });

  if (!response.ok) {
    throw new Error(`ASR failed: ${response.status} ${response.statusText}`);
  }

  const data: Record<string, unknown> = await response.json();
  return {
    text: (data.text as string) || '',
    language: data.language as string | undefined,
    duration: data.duration as number | undefined,
  };
}
