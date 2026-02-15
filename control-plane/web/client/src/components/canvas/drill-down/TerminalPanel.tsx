/**
 * TerminalPanel
 *
 * xterm.js terminal connected to a bot session via gateway WebSocket.
 * Streams terminal output in real-time.
 */

import { useEffect, useRef, useCallback } from 'react';
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

  // Initialize terminal
  useEffect(() => {
    if (!containerRef.current) return;

    let terminal: import('@xterm/xterm').Terminal;
    let fitAddon: import('@xterm/addon-fit').FitAddon;

    const init = async () => {
      const { Terminal } = await import('@xterm/xterm');
      const { FitAddon } = await import('@xterm/addon-fit');
      await import('@xterm/xterm/css/xterm.css');

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

      fitAddon = new FitAddon();
      terminal.loadAddon(fitAddon);

      // Try WebGL addon for performance
      try {
        const { WebglAddon } = await import('@xterm/addon-webgl');
        terminal.loadAddon(new WebglAddon());
      } catch {
        // WebGL not supported, falls back to canvas
      }

      terminal.open(containerRef.current!);
      fitAddon.fit();
      termRef.current = terminal;
      fitAddonRef.current = fitAddon;

      terminal.writeln(`\x1b[36m● Connected to ${agentId}\x1b[0m`);
      terminal.writeln('');

      // Subscribe to agent stream events
      if (sessionKey) {
        const unsub = gateway.on('agent', (payload) => {
          const event = payload as { data?: { agentId?: string; output?: string } };
          if (event.data?.agentId === agentId && event.data?.output) {
            terminal.write(event.data.output);
          }
        });
        return unsub;
      }
    };

    let unsubscribe: (() => void) | undefined;
    init().then((unsub) => { unsubscribe = unsub; });

    // Handle resize
    const resizeObserver = new ResizeObserver(() => {
      fitAddonRef.current?.fit();
    });
    resizeObserver.observe(containerRef.current);

    return () => {
      resizeObserver.disconnect();
      unsubscribe?.();
      terminal?.dispose();
    };
  }, [agentId, sessionKey]);

  // Handle user input → send to bot
  const handleInput = useCallback(
    (data: string) => {
      if (sessionKey) {
        gateway.rpc('chat.send', {
          sessionKey,
          message: data,
          idempotencyKey: `input-${Date.now()}`,
        }).catch(console.error);
      }
    },
    [sessionKey]
  );

  useEffect(() => {
    const term = termRef.current;
    if (!term) return;
    const disposable = term.onData(handleInput);
    return () => disposable.dispose();
  }, [handleInput]);

  return (
    <div
      ref={containerRef}
      className={`h-full w-full bg-[#0d1117] rounded-b-lg overflow-hidden ${className ?? ''}`}
    />
  );
}
