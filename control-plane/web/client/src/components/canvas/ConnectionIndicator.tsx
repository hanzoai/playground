/**
 * ConnectionIndicator
 *
 * Shows gateway WebSocket connection status.
 * Floating pill in the top-right of the canvas.
 */

import type { GatewayConnectionState } from '@/types/gateway';
import { cn } from '@/lib/utils';

interface ConnectionIndicatorProps {
  state: GatewayConnectionState;
  onReconnect?: () => void;
  className?: string;
}

const stateConfig: Record<GatewayConnectionState, {
  label: string;
  color: string;
  pulse: boolean;
}> = {
  disconnected: { label: 'Disconnected', color: 'bg-gray-400', pulse: false },
  connecting: { label: 'Connecting...', color: 'bg-yellow-400', pulse: true },
  authenticating: { label: 'Authenticating...', color: 'bg-yellow-400', pulse: true },
  connected: { label: 'Connected', color: 'bg-green-500', pulse: false },
  reconnecting: { label: 'Reconnecting...', color: 'bg-orange-400', pulse: true },
  error: { label: 'Error', color: 'bg-red-500', pulse: false },
};

export function ConnectionIndicator({ state, onReconnect, className }: ConnectionIndicatorProps) {
  const config = stateConfig[state];

  return (
    <div
      className={cn(
        'flex items-center gap-2 rounded-full border border-border/50 bg-card/90 px-3 py-1.5 shadow-sm backdrop-blur-sm text-xs',
        state === 'error' || state === 'disconnected' ? 'cursor-pointer hover:bg-accent' : '',
        className
      )}
      onClick={state === 'error' || state === 'disconnected' ? onReconnect : undefined}
      role={state === 'error' || state === 'disconnected' ? 'button' : undefined}
      title={state === 'error' || state === 'disconnected' ? 'Click to reconnect' : undefined}
    >
      <span className="relative flex h-2 w-2">
        {config.pulse && (
          <span className={`absolute inline-flex h-full w-full animate-ping rounded-full ${config.color} opacity-75`} />
        )}
        <span className={`relative inline-flex h-2 w-2 rounded-full ${config.color}`} />
      </span>
      <span className="text-muted-foreground select-none">{config.label}</span>
    </div>
  );
}
