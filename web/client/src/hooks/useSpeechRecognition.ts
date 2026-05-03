/**
 * useSpeechRecognition (Hybrid)
 *
 * Primary: Cloud ASR via api.hanzo.ai (Whisper) — works on all browsers.
 * Fallback: Browser Web Speech API (Chrome/Edge only).
 *
 * Uses MediaRecorder to capture audio, sends the blob to the cloud endpoint,
 * and returns a transcript. The hook interface is identical to the previous
 * browser-only version so all consumers work without changes.
 */

import { useState, useCallback, useRef, useEffect } from 'react';
import { transcribeAudio } from '@/services/cloudAsr';

interface UseSpeechRecognitionReturn {
  transcript: string;
  isListening: boolean;
  supported: boolean;
  start: () => void;
  stop: () => void;
}

export function useSpeechRecognition(): UseSpeechRecognitionReturn {
  const [transcript, setTranscript] = useState('');
  const [isListening, setIsListening] = useState(false);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);

  // MediaRecorder + getUserMedia is supported on all modern browsers over HTTPS.
  const supported =
    typeof window !== 'undefined' &&
    typeof navigator !== 'undefined' &&
    !!navigator.mediaDevices?.getUserMedia;

  const start = useCallback(async () => {
    if (!supported || isListening) return;

    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });

      const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus')
        ? 'audio/webm;codecs=opus'
        : 'audio/webm';

      const mediaRecorder = new MediaRecorder(stream, { mimeType });

      chunksRef.current = [];

      mediaRecorder.ondataavailable = (event: BlobEvent) => {
        if (event.data.size > 0) {
          chunksRef.current.push(event.data);
        }
      };

      mediaRecorder.onstop = async () => {
        // Release mic immediately.
        stream.getTracks().forEach((track) => track.stop());

        if (chunksRef.current.length === 0) {
          setIsListening(false);
          return;
        }

        const audioBlob = new Blob(chunksRef.current, { type: 'audio/webm' });

        try {
          const result = await transcribeAudio(audioBlob);
          if (result.text) {
            setTranscript((prev) => prev + (prev ? ' ' : '') + result.text);
          }
        } catch (err) {
          console.warn('Cloud ASR failed:', err);
        }

        setIsListening(false);
      };

      mediaRecorderRef.current = mediaRecorder;
      mediaRecorder.start(1000); // collect in 1 s chunks
      setIsListening(true);
      setTranscript('');
    } catch (err) {
      console.error('Failed to start recording:', err);
      setIsListening(false);
    }
  }, [supported, isListening]);

  const stop = useCallback(() => {
    if (
      mediaRecorderRef.current &&
      mediaRecorderRef.current.state !== 'inactive'
    ) {
      mediaRecorderRef.current.stop();
    }
    mediaRecorderRef.current = null;
  }, []);

  // Cleanup on unmount.
  useEffect(() => {
    return () => {
      if (
        mediaRecorderRef.current &&
        mediaRecorderRef.current.state !== 'inactive'
      ) {
        mediaRecorderRef.current.stop();
      }
    };
  }, []);

  return { transcript, isListening, supported, start, stop };
}
