/**
 * TerminalPanel
 *
 * xterm.js terminal connected to a bot session via gateway WebSocket.
 * Streams terminal output in real-time.
 */

import { useEffect, useRef, useState } from 'react';
import { gateway } from '@/services/gatewayClient';

interface TerminalPanelProps {
  agentId: string;
  sessionKey?: string;
  className?: string;
}

type ConnectionState = 'connecting' | 'connected' | 'disconnected' | 'error';

export function TerminalPanel({ agentId, sessionKey, className }: TerminalPanelProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<import('@xterm/xterm').Terminal | null>(null);
  const fitAddonRef = useRef<import('@xterm/addon-fit').FitAddon | null>(null);
  const unsubRef = useRef<(() => void) | undefined>(undefined);
  const sessionKeyRef = useRef(sessionKey);
  const [connState, setConnState] = useState<ConnectionState>('connecting');

  // Keep sessionKey ref current without re-initializing terminal
  useEffect(() => {
    sessionKeyRef.current = sessionKey;
    if (sessionKey) {
      setConnState('connected');
    } else {
      setConnState('disconnected');
    }
  }, [sessionKey]);

  // Initialize terminal once per agentId — never re-init on sessionKey change
  useEffect(() => {
    if (!containerRef.current) return;

    let terminal: import('@xterm/xterm').Terminal;
    let disposed = false;

    const init = async () => {
      const { Terminal } = await import('@xterm/xterm');
      const { FitAddon } = await import('@xterm/addon-fit');
      await import('@xterm/xterm/css/xterm.css');

      if (disposed) return;

      terminal = new Terminal({
        cursorBlink: true,
        fontSize: 13,
        fontFamily: 'Menlo, Monaco, "Courier New", monospace',
        theme: {
          background: '#0d1117',
          foreground: '#c9d1d9',
          cursor: '#58a6ff',
          selectionBackground: '#264f78',
        },
        allowProposedApi: true,
        scrollback: 10000,
      });

      const fitAddon = new FitAddon();
      terminal.loadAddon(fitAddon);

      // Try WebGL addon for performance
      try {
        const { WebglAddon } = await import('@xterm/addon-webgl');
        terminal.loadAddon(new WebglAddon());
      } catch {
        // WebGL not supported, falls back to canvas
      }

      if (disposed) return;

      terminal.open(containerRef.current!);
      fitAddon.fit();
      termRef.current = terminal;
      fitAddonRef.current = fitAddon;

      if (sessionKeyRef.current) {
        terminal.writeln(`\x1b[36m● Connected to ${agentId}\x1b[0m`);
        terminal.writeln('');
        setConnState('connected');
      } else {
        terminal.writeln(`\x1b[33m● Waiting for session with ${agentId}...\x1b[0m`);
        terminal.writeln('\x1b[90mNo active session. Start the agent to connect.\x1b[0m');
        terminal.writeln('');
        setConnState('disconnected');
      }

      // Subscribe to agent stream events — filter by agentId, not sessionKey
      unsubRef.current = gateway.on('agent', (payload) => {
        const event = payload as { data?: { agentId?: string; output?: string } };
        if (event.data?.agentId === agentId && event.data?.output) {
          terminal.write(event.data.output);
          setConnState('connected');
        }
      });

      // Handle user input → send to bot
      terminal.onData((data: string) => {
        const key = sessionKeyRef.current;
        if (!key) {
          terminal.write('\x1b[31mNo active session. Start the agent first.\x1b[0m\r\n');
          return;
        }
        gateway.rpc('chat.send', {
          sessionKey: key,
          message: data,
          idempotencyKey: `input-${Date.now()}`,
        }).catch((err: Error) => {
          terminal.write(`\x1b[31mSend error: ${err.message}\x1b[0m\r\n`);
          setConnState('error');
        });
      });
    };

    init();

    // Handle resize
    const resizeObserver = new ResizeObserver(() => {
      fitAddonRef.current?.fit();
    });
    resizeObserver.observe(containerRef.current);

    return () => {
      disposed = true;
      resizeObserver.disconnect();
      unsubRef.current?.();
      termRef.current?.dispose();
      termRef.current = null;
      fitAddonRef.current = null;
    };
  }, [agentId]);

  return (
    <div className={`relative h-full w-full ${className ?? ''}`}>
      {/* Connection status indicator */}
      <div className="absolute top-2 right-2 z-10 flex items-center gap-1.5 px-2 py-0.5 rounded bg-black/60 text-[10px]">
        <span className={`inline-block w-1.5 h-1.5 rounded-full ${
          connState === 'connected' ? 'bg-green-400' :
          connState === 'connecting' ? 'bg-yellow-400 animate-pulse' :
          connState === 'error' ? 'bg-red-400' :
          'bg-zinc-500'
        }`} />
        <span className="text-zinc-400">
          {connState === 'connected' ? 'Connected' :
           connState === 'connecting' ? 'Connecting...' :
           connState === 'error' ? 'Error' :
           'Disconnected'}
        </span>
      </div>
      <div
        ref={containerRef}
        className="h-full w-full bg-[#0d1117] rounded-b-lg overflow-hidden"
      />
    </div>
  );
}
