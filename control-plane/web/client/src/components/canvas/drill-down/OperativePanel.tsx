/**
 * OperativePanel
 *
 * Remote desktop view for the bot's host machine.
 * - Uses the gateway's /vnc-viewer endpoint (noVNC proxy to local VNC server)
 * - Falls back to canvas host URL if available
 * - Works for macOS Screen Sharing, Linux x11vnc, etc.
 */

import { useMemo } from 'react';
import { gateway } from '@/services/gatewayClient';

interface OperativePanelProps {
  agentId: string;
  className?: string;
}

/**
 * Resolve the VNC viewer URL from the gateway connection.
 * Priority: gateway /vnc-viewer > canvasHostUrl
 */
function resolveDesktopUrl(agentId: string): string | null {
  const helloOk = gateway.serverInfo;
  const token = gateway.authToken;

  // Derive the gateway HTTP base URL from the WebSocket URL
  const wsUrl = gateway.wsUrl;
  if (wsUrl) {
    try {
      const parsed = new URL(wsUrl);
      const httpProto = parsed.protocol === 'wss:' ? 'https:' : 'http:';
      const base = `${httpProto}//${parsed.host}/vnc-viewer`;
      // Pass auth token as query param since iframes cannot send Authorization headers
      return token ? `${base}?token=${encodeURIComponent(token)}` : base;
    } catch {
      // Fall through to other methods
    }
  }

  // Fall back to canvas host if available (e.g. Operative/noVNC on the node)
  if (helloOk?.canvasHostUrl) {
    return `${helloOk.canvasHostUrl}/vnc.html?agent=${encodeURIComponent(agentId)}&autoconnect=true&resize=scale`;
  }

  return null;
}

export function OperativePanel({ agentId, className }: OperativePanelProps) {
  const desktopUrl = useMemo(() => resolveDesktopUrl(agentId), [agentId]);

  if (!desktopUrl) {
    return (
      <div className={`flex items-center justify-center h-full text-xs text-muted-foreground ${className ?? ''}`}>
        <div className="text-center space-y-2">
          <div className="text-2xl">🖥️</div>
          <div>Desktop not available</div>
          <div className="text-[10px]">Enable Screen Sharing on the host machine</div>
        </div>
      </div>
    );
  }

  return (
    <iframe
      src={desktopUrl}
      title={`Desktop - ${agentId}`}
      className={`h-full w-full border-0 rounded-b-lg ${className ?? ''}`}
      allow="clipboard-read; clipboard-write"
    />
  );
}
