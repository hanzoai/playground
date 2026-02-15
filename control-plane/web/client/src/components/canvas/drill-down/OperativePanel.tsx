/**
 * OperativePanel
 *
 * noVNC iframe for viewing the bot's desktop (Operative).
 * Connects to the bot's canvas host URL from gateway.
 */

import { useMemo } from 'react';
import { gateway } from '@/services/gatewayClient';

interface OperativePanelProps {
  agentId: string;
  className?: string;
}

export function OperativePanel({ agentId, className }: OperativePanelProps) {
  const canvasUrl = useMemo(() => {
    const helloOk = gateway.serverInfo;
    if (!helloOk?.canvasHostUrl) return null;
    // Construct noVNC URL for this agent
    return `${helloOk.canvasHostUrl}/vnc.html?agent=${encodeURIComponent(agentId)}&autoconnect=true&resize=scale`;
  }, [agentId]);

  if (!canvasUrl) {
    return (
      <div className={`flex items-center justify-center h-full text-xs text-muted-foreground ${className ?? ''}`}>
        <div className="text-center space-y-2">
          <div className="text-2xl">🖥️</div>
          <div>Desktop not available</div>
          <div className="text-[10px]">No canvas host configured</div>
        </div>
      </div>
    );
  }

  return (
    <iframe
      src={canvasUrl}
      title={`Desktop - ${agentId}`}
      className={`h-full w-full border-0 rounded-b-lg ${className ?? ''}`}
      sandbox="allow-scripts allow-popups allow-forms"
      referrerPolicy="no-referrer"
    />
  );
}
