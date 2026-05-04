/**
 * useSpeechSynthesis
 *
 * Wraps browser SpeechSynthesis API for TTS.
 * Used as local fallback — cloud TTS via bot gateway is preferred when available.
 */

import { useState, useCallback, useRef, useEffect } from 'react';

interface SpeechSynthesisResult {
  speak: (text: string) => void;
  stop: () => void;
  isSpeaking: boolean;
  voices: SpeechSynthesisVoice[];
  supported: boolean;
}

export function useSpeechSynthesis(voiceName?: string): SpeechSynthesisResult {
  const [isSpeaking, setIsSpeaking] = useState(false);
  const [voices, setVoices] = useState<SpeechSynthesisVoice[]>([]);
  const utteranceRef = useRef<SpeechSynthesisUtterance | null>(null);

  const supported =
    typeof window !== 'undefined' && 'speechSynthesis' in window;

  // Load available voices
  useEffect(() => {
    if (!supported) return;

    const loadVoices = () => {
      const available = speechSynthesis.getVoices();
      if (available.length > 0) {
        setVoices(available);
      }
    };

    loadVoices();
    speechSynthesis.addEventListener('voiceschanged', loadVoices);
    return () => speechSynthesis.removeEventListener('voiceschanged', loadVoices);
  }, [supported]);

  const speak = useCallback(
    (text: string) => {
      if (!supported || !text.trim()) return;

      // Cancel any current speech
      speechSynthesis.cancel();

      const utterance = new SpeechSynthesisUtterance(text);

      // Find requested voice
      if (voiceName) {
        const match = voices.find((v) => v.name === voiceName);
        if (match) utterance.voice = match;
      }

      utterance.rate = 1.0;
      utterance.pitch = 1.0;

      utterance.onstart = () => setIsSpeaking(true);
      utterance.onend = () => setIsSpeaking(false);
      utterance.onerror = () => setIsSpeaking(false);

      utteranceRef.current = utterance;
      speechSynthesis.speak(utterance);
    },
    [supported, voiceName, voices],
  );

  const stop = useCallback(() => {
    if (!supported) return;
    speechSynthesis.cancel();
    setIsSpeaking(false);
  }, [supported]);

  // Cleanup
  useEffect(() => {
    return () => {
      if (supported) speechSynthesis.cancel();
    };
  }, [supported]);

  return { speak, stop, isSpeaking, voices, supported };
}
