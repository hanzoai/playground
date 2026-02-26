/**
 * MicButton
 *
 * Toggle button for voice input in the chat panel.
 * Starts/stops browser speech recognition and passes transcript to parent.
 */

import { useEffect, useCallback } from 'react';
import { useSpeechRecognition } from '@/hooks/useSpeechRecognition';
import { cn } from '@/lib/utils';

interface MicButtonProps {
  onTranscript: (text: string) => void;
  disabled?: boolean;
  className?: string;
}

export function MicButton({ onTranscript, disabled, className }: MicButtonProps) {
  const { transcript, isListening, supported, start, stop } = useSpeechRecognition();

  // Push transcript to parent when recognition produces text
  useEffect(() => {
    if (transcript) {
      onTranscript(transcript);
    }
  }, [transcript, onTranscript]);

  const toggle = useCallback(() => {
    if (isListening) {
      stop();
    } else {
      start();
    }
  }, [isListening, start, stop]);

  if (!supported) return null;

  return (
    <button
      type="button"
      onClick={toggle}
      disabled={disabled}
      title={isListening ? 'Stop listening' : 'Voice input'}
      className={cn(
        'rounded-lg px-2 py-1.5 text-xs transition-all active:scale-95 touch-manipulation',
        isListening
          ? 'bg-red-500/20 text-red-500 animate-pulse'
          : 'bg-muted/50 text-muted-foreground hover:bg-muted hover:text-foreground',
        'disabled:opacity-50',
        className,
      )}
    >
      {isListening ? '🔴' : '🎤'}
    </button>
  );
}
