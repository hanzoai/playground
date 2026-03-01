/**
 * TerminalPanel
 *
 * xterm.js terminal that executes commands on a connected node via
 * gateway node.invoke → system.run. Line-buffered input with shell prompt.
 */

import { useEffect, useRef, useState } from 'react';
import { gateway } from '@/services/gatewayClient';

interface TerminalPanelProps {
  agentId: string;
  sessionKey?: string;
  className?: string;
}

type ConnectionState = 'connecting' | 'connected' | 'disconnected' | 'error';

export function TerminalPanel({ agentId, sessionKey: _sessionKey, className }: TerminalPanelProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<import('@xterm/xterm').Terminal | null>(null);
  const fitAddonRef = useRef<import('@xterm/addon-fit').FitAddon | null>(null);
  const unsubRef = useRef<(() => void) | undefined>(undefined);
  const lineBufferRef = useRef('');
  const busyRef = useRef(false);
  const [connState, setConnState] = useState<ConnectionState>('connecting');

  const PROMPT = '\x1b[36m$\x1b[0m ';

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

      try {
        const { WebglAddon } = await import('@xterm/addon-webgl');
        terminal.loadAddon(new WebglAddon());
      } catch {
        // WebGL not supported
      }

      if (disposed) return;

      terminal.open(containerRef.current!);
      fitAddon.fit();
      termRef.current = terminal;
      fitAddonRef.current = fitAddon;

      terminal.writeln(`\x1b[36m● Connected to ${agentId}\x1b[0m`);
      terminal.writeln('\x1b[90mType commands to execute on the remote node via system.run\x1b[0m');
      terminal.writeln('');
      terminal.write(PROMPT);
      setConnState('connected');

      // Subscribe to agent stream events for background output
      unsubRef.current = gateway.on('agent', (payload) => {
        const event = payload as { data?: { agentId?: string; output?: string } };
        if (event.data?.agentId === agentId && event.data?.output) {
          terminal.write(event.data.output);
        }
      });

      // Line-buffered input: accumulate chars, execute on Enter
      terminal.onData((data: string) => {
        if (busyRef.current) return;

        for (const ch of data) {
          if (ch === '\r' || ch === '\n') {
            // Enter pressed — execute command
            terminal.write('\r\n');
            const cmd = lineBufferRef.current.trim();
            lineBufferRef.current = '';

            if (!cmd) {
              terminal.write(PROMPT);
              continue;
            }

            busyRef.current = true;
            gateway.rpc<{ ok?: boolean; payloadJSON?: string; error?: { code?: string; message?: string } }>(
              'node.invoke',
              {
                nodeId: agentId,
                command: 'system.run',
                params: { command: ['sh', '-c', cmd] },
                timeoutMs: 30000,
                idempotencyKey: `term-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
              },
            ).then((result) => {
              if (result.payloadJSON) {
                try {
                  const payload = JSON.parse(result.payloadJSON);
                  if (payload.stdout) terminal.write(payload.stdout.replace(/\n/g, '\r\n'));
                  if (payload.stderr) terminal.write(`\x1b[31m${payload.stderr.replace(/\n/g, '\r\n')}\x1b[0m`);
                  if (payload.exitCode !== undefined && payload.exitCode !== 0) {
                    terminal.write(`\x1b[90m[exit ${payload.exitCode}]\x1b[0m\r\n`);
                  }
                } catch {
                  terminal.write(result.payloadJSON + '\r\n');
                }
              } else if (result.error) {
                terminal.write(`\x1b[31m${result.error.message || 'command failed'}\x1b[0m\r\n`);
              }
            }).catch((err: Error) => {
              terminal.write(`\x1b[31mError: ${err.message}\x1b[0m\r\n`);
              setConnState('error');
            }).finally(() => {
              busyRef.current = false;
              terminal.write(PROMPT);
            });

          } else if (ch === '\x7f' || ch === '\b') {
            // Backspace
            if (lineBufferRef.current.length > 0) {
              lineBufferRef.current = lineBufferRef.current.slice(0, -1);
              terminal.write('\b \b');
            }
          } else if (ch === '\x03') {
            // Ctrl+C
            lineBufferRef.current = '';
            terminal.write('^C\r\n');
            terminal.write(PROMPT);
          } else if (ch >= ' ') {
            // Printable character
            lineBufferRef.current += ch;
            terminal.write(ch);
          }
        }
      });
    };

    init();

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
