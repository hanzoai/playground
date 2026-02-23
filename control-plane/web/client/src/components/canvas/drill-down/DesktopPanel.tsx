/**
 * DesktopPanel
 *
 * Remote desktop view for bots running on nodes with VNC or RDP.
 * - All OSes: gateway /vnc-viewer proxy (preferred)
 * - Linux fallback: noVNC iframe → node's VNC server (port 6080)
 * - Windows fallback: RDP web client
 * - Mac fallback: gateway VNC proxy to macOS Screen Sharing (port 5900)
 */

import { useMemo } from 'react';
import { cn } from '@/lib/utils';
import { gateway } from '@/services/gatewayClient';

interface DesktopPanelProps {
  /** Bot's node endpoint URL (e.g. https://node.example.com:9550) */
  nodeEndpoint: string;
  /** OS type to determine which desktop protocol to use */
  os: 'linux' | 'windows' | 'macos';
  /** VNC port on the node (default 6080 for noVNC websockify) */
  vncPort?: number;
  className?: string;
}

/**
 * Derive the gateway HTTP base from its WebSocket URL.
 * e.g. wss://bot.hanzo.ai → https://bot.hanzo.ai
 */
function gatewayHttpBase(): string | null {
  const wsUrl = gateway.wsUrl;
  if (!wsUrl) return null;
  try {
    const parsed = new URL(wsUrl);
    const proto = parsed.protocol === 'wss:' ? 'https:' : 'http:';
    return `${proto}//${parsed.host}`;
  } catch {
    return null;
  }
}

export function DesktopPanel({ nodeEndpoint, os, vncPort = 6080, className }: DesktopPanelProps) {
  const desktopUrl = useMemo(() => {
    // Prefer the gateway VNC proxy — works for all OSes (macOS Screen Sharing,
    // Linux x11vnc, Operative, etc.)
    const gwBase = gatewayHttpBase();
    if (gwBase) {
      const token = gateway.authToken;
      return token ? `${gwBase}/vnc-viewer?token=${encodeURIComponent(token)}` : `${gwBase}/vnc-viewer`;
    }

    // Fallback: direct node endpoint
    try {
      const url = new URL(nodeEndpoint);
      if (os === 'linux') {
        return `${url.protocol}//${url.hostname}:${vncPort}/vnc.html?autoconnect=true&resize=scale`;
      }
      if (os === 'windows') {
        return `${url.protocol}//${url.hostname}:${vncPort}/rdp/`;
      }
      // macOS without gateway — point at node VNC directly
      return `${url.protocol}//${url.hostname}:5900`;
    } catch {
      return '';
    }
  }, [nodeEndpoint, os, vncPort]);

  if (!desktopUrl) {
    return (
      <div className={cn('flex items-center justify-center h-full text-sm text-muted-foreground', className)}>
        <div className="text-center space-y-2">
          <div className="text-2xl">🖥️</div>
          <div>Desktop not available</div>
          <div className="text-[10px]">Connect a bot with Screen Sharing or VNC enabled.</div>
        </div>
      </div>
    );
  }

  return (
    <div className={cn('h-full w-full', className)}>
      <iframe
        src={desktopUrl}
        title={`${os === 'linux' ? 'VNC' : os === 'windows' ? 'RDP' : 'Screen Sharing'} Desktop`}
        className="w-full h-full border-0 rounded-b-lg"
        allow="clipboard-read; clipboard-write"
      />
    </div>
  );
}
