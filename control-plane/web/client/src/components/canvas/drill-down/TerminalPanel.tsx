/**
 * TerminalPanel
 *
 * xterm.js terminal connected to a bot session via gateway WebSocket.
 * Streams terminal output in real-time.
 */

import { useEffect, useRef } from 'react';
import { gateway } from '@/services/gatewayClient';

interface TerminalPanelProps {
  agentId: string;
  sessionKey?: string;
  className?: string;
}

export function TerminalPanel({ agentId, sessionKey, className }: TerminalPanelProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<import('@xterm/xterm').Terminal | null>(null);
  const fitAddonRef = useRef<import('@xterm/addon-fit').FitAddon | null>(null);
  const unsubRef = useRef<(() => void) | undefined>(undefined);
  const sessionKeyRef = useRef(sessionKey);

  // Keep sessionKey ref current without re-initializing terminal
  useEffect(() => {
    sessionKeyRef.current = sessionKey;
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

      terminal.writeln(`\x1b[36m● Connected to ${agentId}\x1b[0m`);
      terminal.writeln('');

      // Subscribe to agent stream events — filter by agentId, not sessionKey
      unsubRef.current = gateway.on('agent', (payload) => {
        const event = payload as { data?: { agentId?: string; output?: string } };
        if (event.data?.agentId === agentId && event.data?.output) {
          terminal.write(event.data.output);
        }
      });

      // Handle user input → send to bot
      terminal.onData((data: string) => {
        const key = sessionKeyRef.current;
        if (key) {
          gateway.rpc('chat.send', {
            sessionKey: key,
            message: data,
            idempotencyKey: `input-${Date.now()}`,
          }).catch(console.error);
        }
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
    <div
      ref={containerRef}
      className={`h-full w-full bg-[#0d1117] rounded-b-lg overflow-hidden ${className ?? ''}`}
    />
  );
}
