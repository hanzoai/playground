/**
 * TerminalPanel
 *
 * xterm.js terminal that executes commands on a connected node via
 * gateway node.invoke → system.run. Line-buffered input with shell prompt.
 *
 * State-aware: probes the gateway to verify the node is reachable before
 * enabling input. Shows appropriate overlays for each state.
 */

import { useEffect, useRef, useState, useCallback } from 'react';
import { gateway } from '@/services/gatewayClient';

export type NodeReadiness = 'ready' | 'provisioning' | 'starting' | 'stopping' | 'offline' | 'unknown';

interface TerminalPanelProps {
  agentId: string;
  sessionKey?: string;
  className?: string;
  /** Current node lifecycle status from the backend. */
  nodeStatus?: NodeReadiness;
}

type ConnectionState =
  | 'waiting'       // waiting for gateway WS to connect
  | 'probing'       // checking if node is reachable via gateway
  | 'connected'     // node confirmed reachable, terminal active
  | 'unreachable'   // gateway connected but node not found
  | 'disconnected'  // gateway WS down
  | 'error';        // command returned an error

/** Overlay shown when the terminal is not interactive. */
function StatusOverlay({ state, nodeStatus, onRetry }: {
  state: ConnectionState;
  nodeStatus?: NodeReadiness;
  onRetry?: () => void;
}) {
  const configs: Record<string, { label: string; sub: string; pulse: boolean; retry?: boolean }> = {
    waiting: {
      label: 'Connecting to gateway',
      sub: 'Establishing WebSocket connection…',
      pulse: true,
    },
    probing: {
      label: 'Checking node',
      sub: 'Verifying the bot is reachable…',
      pulse: true,
    },
    unreachable: {
      label: 'Bot not connected',
      sub: 'This bot is not currently connected to the gateway.',
      pulse: false,
      retry: true,
    },
    disconnected: {
      label: 'Gateway disconnected',
      sub: 'WebSocket connection to the gateway is down.',
      pulse: false,
      retry: true,
    },
    provisioning: {
      label: 'Provisioning',
      sub: 'Setting up the bot environment…',
      pulse: true,
    },
    starting: {
      label: 'Starting',
      sub: 'Bot is booting up…',
      pulse: true,
    },
    stopping: {
      label: 'Stopping',
      sub: 'Bot is shutting down…',
      pulse: true,
    },
    offline: {
      label: 'Offline',
      sub: 'Bot is not connected to the gateway.',
      pulse: false,
      retry: true,
    },
    unknown: {
      label: 'Waiting',
      sub: 'Checking bot status…',
      pulse: true,
    },
  };

  // Use nodeStatus if the issue is lifecycle-related, otherwise use connection state
  const key = (nodeStatus && nodeStatus !== 'ready') ? nodeStatus : state;
  const c = configs[key] ?? configs.unknown;

  return (
    <div className="absolute inset-0 z-20 flex flex-col items-center justify-center bg-[#0d1117]/90 backdrop-blur-sm rounded-b-lg">
      {c.pulse && (
        <div className="mb-4 flex items-center gap-2">
          <span className="inline-block h-2 w-2 rounded-full bg-cyan-400 animate-pulse" />
          <span className="inline-block h-2 w-2 rounded-full bg-cyan-400 animate-pulse [animation-delay:150ms]" />
          <span className="inline-block h-2 w-2 rounded-full bg-cyan-400 animate-pulse [animation-delay:300ms]" />
        </div>
      )}
      <span className="text-sm font-medium text-zinc-200">{c.label}</span>
      <span className="mt-1 text-xs text-zinc-500">{c.sub}</span>
      {c.retry && onRetry && (
        <button
          onClick={onRetry}
          className="mt-3 px-3 py-1 text-xs text-cyan-400 border border-cyan-400/30 rounded hover:bg-cyan-400/10 transition-colors"
        >
          Retry
        </button>
      )}
    </div>
  );
}

export function TerminalPanel({ agentId, sessionKey: _sessionKey, className, nodeStatus }: TerminalPanelProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<import('@xterm/xterm').Terminal | null>(null);
  const fitAddonRef = useRef<import('@xterm/addon-fit').FitAddon | null>(null);
  const unsubRef = useRef<(() => void) | undefined>(undefined);
  const gwUnsubRef = useRef<(() => void) | undefined>(undefined);
  const lineBufferRef = useRef('');
  const busyRef = useRef(false);
  const initRef = useRef(false);
  const [connState, setConnState] = useState<ConnectionState>('waiting');

  const PROMPT = '\x1b[36m$\x1b[0m ';

  // Whether the node lifecycle allows terminal interaction
  const lifecycleReady = !nodeStatus || nodeStatus === 'ready';

  // Whether the terminal should be interactive
  const isInteractive = connState === 'connected' && lifecycleReady;
  const showOverlay = !isInteractive;

  // Probe gateway to check if node is actually reachable
  const probeNode = useCallback(async (): Promise<boolean> => {
    if (!gateway.isConnected) {
      setConnState('disconnected');
      return false;
    }
    setConnState('probing');
    try {
      const resp = await gateway.rpc<{ nodeId?: string; connected?: boolean }>(
        'node.describe', { nodeId: agentId }
      );
      if (resp?.connected) {
        setConnState('connected');
        return true;
      }
      setConnState('unreachable');
      return false;
    } catch {
      setConnState('unreachable');
      return false;
    }
  }, [agentId]);

  // Initialize xterm
  useEffect(() => {
    if (!containerRef.current) return;
    if (initRef.current) return;
    initRef.current = true;

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

      // Wait for gateway WS to be connected
      if (!gateway.isConnected) {
        terminal.writeln('\x1b[90mWaiting for gateway connection…\x1b[0m');
        setConnState('waiting');
        gwUnsubRef.current = gateway.onStateChange((state) => {
          if (state === 'connected' && !disposed) {
            gwUnsubRef.current?.();
            gwUnsubRef.current = undefined;
            connectToNode(terminal);
          }
        });
      } else {
        connectToNode(terminal);
      }

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
                if (result.error.code === 'NOT_CONNECTED') {
                  terminal.write('\x1b[33mBot is not connected to the gateway.\x1b[0m\r\n');
                  setConnState('unreachable');
                } else {
                  terminal.write(`\x1b[31m${result.error.message || 'command failed'}\x1b[0m\r\n`);
                }
              }
            }).catch((err: unknown) => {
              const msg = err instanceof Error ? err.message : String((err as { message?: string })?.message ?? err);
              if (msg.includes('not connected') || msg.includes('NOT_CONNECTED')) {
                terminal.write('\x1b[33mGateway connection lost.\x1b[0m\r\n');
                setConnState('disconnected');
              } else {
                terminal.write(`\x1b[31mError: ${msg}\x1b[0m\r\n`);
                setConnState('error');
              }
            }).finally(() => {
              busyRef.current = false;
              terminal.write(PROMPT);
            });

          } else if (ch === '\x7f' || ch === '\b') {
            if (lineBufferRef.current.length > 0) {
              lineBufferRef.current = lineBufferRef.current.slice(0, -1);
              terminal.write('\b \b');
            }
          } else if (ch === '\x03') {
            lineBufferRef.current = '';
            terminal.write('^C\r\n');
            terminal.write(PROMPT);
          } else if (ch >= ' ') {
            lineBufferRef.current += ch;
            terminal.write(ch);
          }
        }
      });
    };

    const connectToNode = async (term: import('@xterm/xterm').Terminal) => {
      const reachable = await probeNode();
      if (reachable) {
        term.writeln(`\x1b[36m● Connected to ${agentId}\x1b[0m`);
        term.writeln('\x1b[90mType commands to execute on this bot\x1b[0m');
        term.writeln('');
        term.write(PROMPT);
      } else {
        term.writeln(`\x1b[33m● Bot ${agentId} is not reachable\x1b[0m`);
        term.writeln('\x1b[90mThe bot may be offline or not connected to the gateway.\x1b[0m');
        term.writeln('');
      }
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
      gwUnsubRef.current?.();
      termRef.current?.dispose();
      termRef.current = null;
      fitAddonRef.current = null;
      initRef.current = false;
    };
  }, [agentId, probeNode]);

  const handleRetry = useCallback(() => {
    const term = termRef.current;
    if (term) {
      term.writeln('');
      term.writeln('\x1b[90mRetrying connection…\x1b[0m');
    }
    probeNode().then((ok) => {
      if (ok && term) {
        term.writeln(`\x1b[36m● Connected to ${agentId}\x1b[0m`);
        term.write(PROMPT);
      }
    });
  }, [agentId, probeNode]);

  // Badge color/label
  const badgeColor =
    isInteractive ? 'bg-green-400' :
    connState === 'probing' || connState === 'waiting' ? 'bg-yellow-400 animate-pulse' :
    connState === 'error' ? 'bg-red-400' :
    connState === 'unreachable' || connState === 'disconnected' ? 'bg-zinc-500' :
    !lifecycleReady ? 'bg-yellow-400 animate-pulse' :
    'bg-zinc-500';

  const badgeLabel =
    !lifecycleReady ? (nodeStatus === 'starting' ? 'Starting' : nodeStatus === 'offline' ? 'Offline' : 'Waiting') :
    isInteractive ? 'Connected' :
    connState === 'probing' ? 'Checking…' :
    connState === 'waiting' ? 'Connecting…' :
    connState === 'unreachable' ? 'Not connected' :
    connState === 'disconnected' ? 'Disconnected' :
    connState === 'error' ? 'Error' :
    'Unknown';

  return (
    <div className={`relative h-full w-full ${className ?? ''}`}>
      {/* Status badge */}
      <div className="absolute top-2 right-2 z-30 flex items-center gap-1.5 px-2 py-0.5 rounded bg-black/60 text-[10px]">
        <span className={`inline-block w-1.5 h-1.5 rounded-full ${badgeColor}`} />
        <span className="text-zinc-400">{badgeLabel}</span>
      </div>

      {/* Overlay for non-interactive states */}
      {showOverlay && (
        <StatusOverlay
          state={connState}
          nodeStatus={nodeStatus}
          onRetry={connState === 'unreachable' || connState === 'disconnected' ? handleRetry : undefined}
        />
      )}

      <div
        ref={containerRef}
        className="h-full w-full bg-[#0d1117] rounded-b-lg overflow-hidden"
      />
    </div>
  );
}
