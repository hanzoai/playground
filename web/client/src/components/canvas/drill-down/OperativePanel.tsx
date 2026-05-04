/**
 * OperativePanel
 *
 * Remote desktop view for a bot node's host machine.
 * - Uses the gateway's /vnc-viewer endpoint with optional nodeId for tunnel mode
 * - Falls back to canvas host URL if available
 * - Works for macOS Screen Sharing, Linux x11vnc, etc.
 */

import { useMemo } from 'react';
import { gateway } from '@/services/gatewayClient';

interface OperativePanelProps {
  agentId: string;
  /** Node ID to tunnel VNC through the gateway to a remote node. */
  nodeId?: string;
  className?: string;
}

/**
 * Resolve the VNC viewer URL from the gateway connection.
 * When nodeId is provided, the gateway tunnels VNC data through the node's
 * WebSocket connection to reach the node's local VNC server.
 */
function resolveDesktopUrl(agentId: string, nodeId?: string): string | null {
  const helloOk = gateway.serverInfo;
  const token = gateway.authToken;

  // Derive the gateway HTTP base URL from the WebSocket URL
  const wsUrl = gateway.wsUrl;
  if (wsUrl) {
    try {
      const parsed = new URL(wsUrl);
      const httpProto = parsed.protocol === 'wss:' ? 'https:' : 'http:';
      const params = new URLSearchParams();
      if (token) params.set('token', token);
      if (nodeId) params.set('nodeId', nodeId);
      const qs = params.toString();
      return `${httpProto}//${parsed.host}/vnc-viewer${qs ? `?${qs}` : ''}`;
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

export function OperativePanel({ agentId, nodeId, className }: OperativePanelProps) {
  const desktopUrl = useMemo(() => resolveDesktopUrl(agentId, nodeId), [agentId, nodeId]);

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
