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

interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: number;
  streaming?: boolean;
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
        setMessages((prev) => {
          const last = prev[prev.length - 1];
          if (last?.streaming) {
            return [
              ...prev.slice(0, -1),
              { ...last, content: last.content + (event.message as string ?? '') },
            ];
          }
          return [
            ...prev,
            {
              id: event.runId,
              role: 'assistant',
              content: event.message as string ?? '',
              timestamp: Date.now(),
              streaming: true,
            },
          ];
        });
      }

      if (event.state === 'final') {
        setStreaming(false);
        setMessages((prev) =>
          prev.map((m) => m.id === event.runId ? { ...m, streaming: false } : m)
        );
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
            </div>
          </div>
        ))}
      </div>

      {/* Input */}
      <div className="border-t border-border/40 p-2">
        <div className="flex gap-2">
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
