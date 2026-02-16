/**
 * DesktopPanel
 *
 * Remote desktop view for bots running on nodes with VNC or RDP.
 * - Linux: noVNC iframe → node's VNC server (port 6080)
 * - Windows: RDP web client
 * - Mac: Screen sharing (future)
 */

import { useMemo } from 'react';
import { cn } from '@/lib/utils';

interface DesktopPanelProps {
  /** Bot's node endpoint URL (e.g. https://node.example.com:9550) */
  nodeEndpoint: string;
  /** OS type to determine which desktop protocol to use */
  os: 'linux' | 'windows' | 'macos';
  /** VNC port on the node (default 6080 for noVNC websockify) */
  vncPort?: number;
  className?: string;
}

export function DesktopPanel({ nodeEndpoint, os, vncPort = 6080, className }: DesktopPanelProps) {
  const desktopUrl = useMemo(() => {
    try {
      const url = new URL(nodeEndpoint);
      if (os === 'linux') {
        // noVNC web client — served by websockify on the node
        return `${url.protocol}//${url.hostname}:${vncPort}/vnc.html?autoconnect=true&resize=scale`;
      }
      if (os === 'windows') {
        // Apache Guacamole or RDP web client on the node
        return `${url.protocol}//${url.hostname}:${vncPort}/rdp/`;
      }
      // macOS — placeholder for screen sharing
      return '';
    } catch {
      return '';
    }
  }, [nodeEndpoint, os, vncPort]);

  if (os === 'macos') {
    return (
      <div className={cn('flex items-center justify-center h-full text-sm text-muted-foreground', className)}>
        <div className="text-center">
          <p className="font-medium mb-1">macOS Screen Sharing</p>
          <p className="text-xs">Connect via screen sharing at the node's address.</p>
          <code className="text-xs mt-2 block font-mono bg-muted px-2 py-1 rounded">{nodeEndpoint}</code>
        </div>
      </div>
    );
  }

  if (!desktopUrl) {
    return (
      <div className={cn('flex items-center justify-center h-full text-sm text-muted-foreground', className)}>
        Unable to determine desktop URL from node endpoint.
      </div>
    );
  }

  return (
    <div className={cn('h-full w-full', className)}>
      <iframe
        src={desktopUrl}
        title={`${os === 'linux' ? 'VNC' : 'RDP'} Desktop`}
        className="w-full h-full border-0 rounded-b-lg"
        allow="clipboard-read; clipboard-write"
        sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
      />
    </div>
  );
}
