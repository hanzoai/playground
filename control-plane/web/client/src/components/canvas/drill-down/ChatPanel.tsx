/**
 * ChatPanel
 *
 * Streaming markdown chat connected to a bot session.
 * Real-time message rendering with markdown support.
 */

import { useState, useCallback, useRef, useEffect } from 'react';
import { gateway } from '@/services/gatewayClient';
import { chatSend } from '@/services/gatewayApi';
import type { ChatEvent } from '@/types/gateway';
import { cn } from '@/lib/utils';
import { MicButton } from './MicButton';
import { useSpeechSynthesis } from '@/hooks/useSpeechSynthesis';
import { usePreferencesStore } from '@/stores/preferencesStore';

interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: number;
  streaming?: boolean;
}

/** Extract text from a gateway chat message payload */
function extractMessageText(message: unknown): string {
  if (typeof message === 'string') return message;
  if (!message || typeof message !== 'object') return '';
  const msg = message as Record<string, unknown>;
  if (Array.isArray(msg.content)) {
    return (msg.content as Array<Record<string, unknown>>)
      .filter((b) => b.type === 'text' && typeof b.text === 'string')
      .map((b) => b.text as string)
      .join('');
  }
  if (typeof msg.content === 'string') return msg.content;
  if (typeof msg.text === 'string') return msg.text;
  return '';
}

interface ChatPanelProps {
  agentId: string;
  sessionKey?: string;
  className?: string;
}

export function ChatPanel({ agentId, sessionKey, className }: ChatPanelProps) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [streaming, setStreaming] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  // Voice I/O
  const voiceInputEnabled = usePreferencesStore((s) => s.voiceInputEnabled);
  const voiceOutputEnabled = usePreferencesStore((s) => s.voiceOutputEnabled);
  const voiceOutputVoice = usePreferencesStore((s) => s.voiceOutputVoice);
  const { speak, isSpeaking, stop: stopSpeaking } = useSpeechSynthesis(voiceOutputVoice);

  const handleVoiceTranscript = useCallback((text: string) => {
    setInput(text);
  }, []);

  // Auto-scroll to bottom
  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' });
  }, [messages]);

  // Subscribe to chat events
  useEffect(() => {
    if (!sessionKey) return;

    const unsub = gateway.on('chat', (payload) => {
      const event = payload as ChatEvent;
      if (event.sessionKey !== sessionKey) return;

      if (event.state === 'delta') {
        setStreaming(true);
        const text = extractMessageText(event.message);
        setMessages((prev) => {
          const last = prev[prev.length - 1];
          if (last?.streaming && last.id === event.runId) {
            // CLI backends send full text each delta (not incremental), so replace
            return [
              ...prev.slice(0, -1),
              { ...last, content: text },
            ];
          }
          return [
            ...prev,
            {
              id: event.runId,
              role: 'assistant',
              content: text,
              timestamp: Date.now(),
              streaming: true,
            },
          ];
        });
      }

      if (event.state === 'final') {
        setStreaming(false);
        const finalText = extractMessageText(event.message);
        setMessages((prev) => {
          const existing = prev.find((m) => m.id === event.runId);
          if (existing) {
            // Update existing streaming message with final text and stop streaming
            return prev.map((m) =>
              m.id === event.runId
                ? { ...m, content: finalText || m.content, streaming: false }
                : m
            );
          }
          // No delta was received — add the final message directly
          if (finalText) {
            return [
              ...prev,
              {
                id: event.runId,
                role: 'assistant' as const,
                content: finalText,
                timestamp: Date.now(),
              },
            ];
          }
          return prev;
        });
      }

      if (event.state === 'error') {
        setStreaming(false);
        setMessages((prev) => [
          ...prev,
          {
            id: `err-${Date.now()}`,
            role: 'assistant',
            content: `Error: ${event.errorMessage ?? 'Unknown error'}`,
            timestamp: Date.now(),
          },
        ]);
      }
    });

    return unsub;
  }, [sessionKey]);

  const send = useCallback(async () => {
    const text = input.trim();
    if (!text || !sessionKey) return;

    setInput('');
    setMessages((prev) => [
      ...prev,
      { id: `user-${Date.now()}`, role: 'user', content: text, timestamp: Date.now() },
    ]);

    try {
      await chatSend({
        sessionKey,
        message: text,
        idempotencyKey: `chat-${Date.now()}`,
      });
    } catch (e) {
      console.error('[ChatPanel] Send failed:', e);
    }
  }, [input, sessionKey]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        send();
      }
    },
    [send]
  );

  return (
    <div className={cn('flex flex-col h-full', className)}>
      {/* Messages */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto px-3 py-2 space-y-3">
        {messages.length === 0 && (
          <div className="flex items-center justify-center h-full text-xs text-muted-foreground">
            Start chatting with {agentId}
          </div>
        )}
        {messages.map((msg) => (
          <div key={msg.id} className={cn('flex', msg.role === 'user' ? 'justify-end' : 'justify-start')}>
            <div
              className={cn(
                'max-w-[85%] rounded-lg px-3 py-2 text-xs leading-relaxed',
                msg.role === 'user'
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-accent/50 text-foreground',
                msg.streaming && 'animate-pulse'
              )}
            >
              <pre className="whitespace-pre-wrap font-sans break-words">{msg.content}</pre>
              {voiceOutputEnabled && msg.role === 'assistant' && !msg.streaming && msg.content && (
                <button
                  type="button"
                  onClick={() => isSpeaking ? stopSpeaking() : speak(msg.content)}
                  className="mt-1 text-[10px] text-muted-foreground hover:text-foreground transition-colors"
                  title={isSpeaking ? 'Stop reading' : 'Read aloud'}
                >
                  {isSpeaking ? '⏹' : '🔊'}
                </button>
              )}
            </div>
          </div>
        ))}
      </div>

      {/* Input */}
      <div className="border-t border-border/40 p-2">
        <div className="flex gap-2">
          {voiceInputEnabled && (
            <MicButton
              onTranscript={handleVoiceTranscript}
              disabled={!sessionKey || streaming}
            />
          )}
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={sessionKey ? 'Message...' : 'No session connected'}
            disabled={!sessionKey || streaming}
            className="flex-1 rounded-lg border border-border/50 bg-background/50 px-3 py-1.5 text-xs placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary/50 disabled:opacity-50"
          />
          <button
            type="button"
            onClick={send}
            disabled={!input.trim() || !sessionKey || streaming}
            className="rounded-lg bg-primary px-3 py-1.5 text-xs text-primary-foreground disabled:opacity-50 hover:bg-primary/90 active:scale-95 transition-all touch-manipulation"
          >
            Send
          </button>
        </div>
      </div>
    </div>
  );
}
