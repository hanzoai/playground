/**
 * SpaceChatSidebar
 *
 * Slide-out panel on the right side of the canvas for space-wide chat.
 * Messages from all users and agents in the active space.
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import { gateway } from '@/services/gatewayClient';
import { cn } from '@/lib/utils';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface ChatMessage {
  id: string;
  userId: string;
  displayName: string;
  avatar?: string;
  text: string;
  timestamp: number;
}

interface SpaceChatSidebarProps {
  open: boolean;
  onClose: () => void;
  onUnreadChange?: (count: number) => void;
}

// ---------------------------------------------------------------------------
// SpaceChatSidebar
// ---------------------------------------------------------------------------

export function SpaceChatSidebar({ open, onClose, onUnreadChange }: SpaceChatSidebarProps) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState('');
  const bottomRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Track unread when sidebar is closed
  const [unreadCount, setUnreadCount] = useState(0);

  // Listen for incoming chat messages from gateway
  useEffect(() => {
    const unsub = gateway.on('chat.room.message', (payload) => {
      const msg = payload as ChatMessage;
      setMessages((prev) => [...prev, msg]);
      if (!open) {
        setUnreadCount((c) => {
          const next = c + 1;
          onUnreadChange?.(next);
          return next;
        });
      }
    });
    return unsub;
  }, [open, onUnreadChange]);

  // Clear unread when opening
  useEffect(() => {
    if (open) {
      setUnreadCount(0);
      onUnreadChange?.(0);
      // Focus input when sidebar opens
      setTimeout(() => inputRef.current?.focus(), 100);
    }
  }, [open, onUnreadChange]);

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const sendMessage = useCallback(() => {
    const text = input.trim();
    if (!text) return;

    const msg: ChatMessage = {
      id: `local-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
      userId: 'self',
      displayName: 'You',
      text,
      timestamp: Date.now(),
    };

    // Optimistic local append
    setMessages((prev) => [...prev, msg]);
    setInput('');

    // Send via gateway (fire-and-forget for now)
    if (gateway.isConnected) {
      gateway.rpc('chat.room.send', { message: text }).catch((err) => {
        console.warn('[chat] Failed to send message via gateway:', err);
      });
    }
  }, [input]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        sendMessage();
      }
    },
    [sendMessage]
  );

  return (
    <div
      className={cn(
        'fixed right-0 top-0 z-40 flex h-full w-80 flex-col border-l border-border bg-card shadow-2xl',
        'transition-transform duration-200 ease-in-out',
        open ? 'translate-x-0' : 'translate-x-full'
      )}
    >
      {/* Header */}
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <h3 className="text-sm font-semibold text-foreground">Space Chat</h3>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close chat"
          className="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
        >
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
            <path d="M3 3l8 8M11 3l-8 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          </svg>
        </button>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto px-3 py-2 space-y-3">
        {messages.length === 0 && (
          <div className="flex h-full items-center justify-center">
            <p className="text-xs text-muted-foreground">No messages yet. Say something!</p>
          </div>
        )}
        {messages.map((msg) => (
          <div key={msg.id} className="group">
            <div className="flex items-start gap-2">
              {/* Avatar */}
              <div className="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full bg-muted text-[10px] font-bold text-muted-foreground">
                {msg.avatar ? (
                  <img src={msg.avatar} alt="" className="h-6 w-6 rounded-full" />
                ) : (
                  msg.displayName.charAt(0).toUpperCase()
                )}
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-baseline gap-2">
                  <span className="text-xs font-medium text-foreground">{msg.displayName}</span>
                  <span className="text-[10px] text-muted-foreground">
                    {new Date(msg.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                  </span>
                </div>
                <p className="text-sm text-foreground/90 break-words">{msg.text}</p>
              </div>
            </div>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <div className="border-t border-border p-3">
        <div className="flex items-center gap-2">
          <input
            ref={inputRef}
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type a message..."
            className="flex-1 rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          />
          <button
            type="button"
            onClick={sendMessage}
            disabled={!input.trim()}
            aria-label="Send message"
            className={cn(
              'flex h-8 w-8 items-center justify-center rounded-lg transition-colors',
              input.trim()
                ? 'bg-primary text-primary-foreground hover:bg-primary/90'
                : 'bg-muted text-muted-foreground cursor-not-allowed'
            )}
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
              <path d="M1 7l5-5v3.5h6v3H6V12L1 7z" fill="currentColor" transform="rotate(-90 7 7)" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  );
}
