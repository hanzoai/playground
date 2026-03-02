/**
 * CanvasPage (Space Canvas)
 *
 * Full-bleed infinite canvas for bot orchestration.
 * Starts empty — shows onboarding when no bots exist in the active space.
 * Responsive: sidebar panel on desktop, drawer on mobile.
 */

import { useEffect, useState } from 'react';
import { ReactFlowProvider } from '@xyflow/react';
import { CanvasFlow } from '@/components/canvas/CanvasFlow';
import { CanvasSidebar } from '@/components/canvas/CanvasSidebar';
import { ConnectionIndicator } from '@/components/canvas/ConnectionIndicator';
import { ActionPill } from '@/components/canvas/ActionPill/ActionPill';
import { CommandPalette } from '@/components/canvas/CommandPalette';
import { FirstBotOnboarding } from '@/components/onboarding/FirstBotOnboarding';
import { useGateway } from '@/hooks/useGateway';
import { useNodeEventsSSE } from '@/hooks/useSSE';
import { nodeList } from '@/services/gatewayApi';
import { useCanvasStore } from '@/stores/canvasStore';
import { useBotStore } from '@/stores/botStore';
import { useSpaceStore } from '@/stores/spaceStore';
import type { Bot } from '@/types/canvas';
import { NODE_TYPES } from '@/types/canvas';

export function CanvasPage() {
  const { connectionState, isConnected, reconnect, syncAgents } = useGateway();
  const restore = useCanvasStore((s) => s.restore);
  const nodes = useCanvasStore((s) => s.nodes);
  const setBotStatus = useCanvasStore((s) => s.setBotStatus);
  const initialized = useBotStore((s) => s.initialized);
  const agentCount = useBotStore((s) => s.agents.size);
  const [sidebarOpen, setSidebarOpen] = useState(false);

  const activeSpace = useSpaceStore((s) => s.activeSpace);
  const spaceBots = useSpaceStore((s) => s.bots);
  const fetchBots = useSpaceStore((s) => s.fetchBots);
  const fetchSpaces = useSpaceStore((s) => s.fetchSpaces);
  const spaceLoading = useSpaceStore((s) => s.loading);
  const spaceBootstrapped = useSpaceStore((s) => s.bootstrapped);

  // Subscribe to node SSE events to track cloud bot status transitions
  const { latestEvent } = useNodeEventsSSE();

  // Restore persisted canvas on mount
  useEffect(() => { restore(); }, [restore]);

  // Update canvas bot status when node events arrive (provisioning → idle)
  useEffect(() => {
    if (!latestEvent) return;
    const eventData = latestEvent.data;
    if (!eventData || typeof eventData !== 'object') return;

    const nodeId = (eventData as any).node_id ?? (eventData as any).data?.node_id;
    if (!nodeId) return;

    // Find matching canvas bot
    const canvasBot = nodes.find(
      (n) => n.type === NODE_TYPES.bot && (n.data as unknown as Bot).agentId === nodeId
    );
    if (!canvasBot) return;
    const botData = canvasBot.data as unknown as Bot;

    switch (latestEvent.type) {
      case 'node_registered':
      case 'node_online':
        // Bot came online — transition from provisioning to idle
        if (botData.status === 'provisioning' || botData.status === 'offline') {
          setBotStatus(nodeId, 'idle');
          // Re-sync from gateway to pick up agent identity/session info
          syncAgents();
        }
        break;
      case 'node_offline':
        setBotStatus(nodeId, 'offline');
        break;
      case 'node_status_updated':
      case 'node_unified_status_changed': {
        const health = (eventData as any).new_status?.health_status
          ?? (eventData as any).status?.health_status
          ?? (eventData as any).health_status;
        if (health === 'active') {
          setBotStatus(nodeId, 'idle');
        } else if (health === 'inactive' || health === 'unreachable') {
          setBotStatus(nodeId, 'offline');
        }
        break;
      }
    }
  }, [latestEvent, nodes, setBotStatus, syncAgents]);

  // Poll gateway node.list to transition provisioning bots to idle.
  // Cloud nodes connect to the gateway (not the Playground backend), so
  // SSE events from the backend never fire for them.  This effect bridges
  // the gap by checking the gateway's connected-node list periodically.
  useEffect(() => {
    if (!isConnected) return;

    async function reconcileProvisioningBots() {
      try {
        const resp = await nodeList(true);
        const gwNodes = (resp.nodes ?? []).filter((n) => n.connected);
        if (gwNodes.length === 0) return;

        for (const node of nodes) {
          if (node.type !== NODE_TYPES.bot) continue;
          const bot = node.data as unknown as Bot;
          if (bot.status !== 'provisioning') continue;

          // Match by nodeId, or by displayName containing the canvas agentId.
          // Cloud nodes have displayName like "agent-cloud-3f7d7054" while
          // the canvas tracks them as "cloud-3f7d7054".
          const match = gwNodes.some(
            (gw) =>
              gw.nodeId === bot.agentId ||
              gw.displayName === bot.agentId ||
              gw.displayName === `agent-${bot.agentId}` ||
              (gw.displayName && bot.agentId && gw.displayName.includes(bot.agentId))
          );
          if (match) {
            setBotStatus(bot.agentId, 'idle');
          }
        }
      } catch {
        // Gateway RPC failed — skip this cycle
      }
    }

    // Run immediately, then every 10s
    reconcileProvisioningBots();
    const timer = setInterval(reconcileProvisioningBots, 10_000);
    return () => clearInterval(timer);
  }, [isConnected, nodes, setBotStatus]);

  // Bootstrap spaces on direct navigation (e.g. /playground)
  useEffect(() => {
    if (!activeSpace) fetchSpaces();
  }, [activeSpace, fetchSpaces]);

  // Fetch space bots when space is active
  useEffect(() => {
    if (activeSpace) fetchBots();
  }, [activeSpace, fetchBots]);

  // Show onboarding when space has no bots and canvas has no nodes
  const showOnboarding = activeSpace && (spaceBots?.length ?? 0) === 0 && (nodes?.length ?? 0) === 0 && initialized;

  return (
    <ReactFlowProvider>
      <div className="absolute inset-0 flex overflow-hidden">
        {/* Sidebar */}
        <CanvasSidebar open={sidebarOpen} onClose={() => setSidebarOpen(false)} />

        {/* Canvas area */}
        <div className="relative flex-1 min-w-0">
          <CanvasFlow />

          {/* Top bar */}
          <div className="absolute top-3 left-3 right-3 z-20 flex items-center justify-between pointer-events-none">
            {/* Left: sidebar toggle */}
            <button
              type="button"
              onClick={() => setSidebarOpen((prev) => !prev)}
              className="pointer-events-auto flex h-8 w-8 items-center justify-center rounded-lg border border-border/50 bg-card/90 text-muted-foreground shadow-sm backdrop-blur-sm transition-colors hover:bg-accent hover:text-foreground touch-manipulation"
              title="Toggle sidebar"
            >
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                <path d="M2 4h10M2 7h10M2 10h10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
            </button>

            {/* Right: connection + count */}
            <div className="pointer-events-auto flex items-center gap-2">
              {isConnected && agentCount > 0 && (
                <div className="flex items-center gap-1.5 rounded-full border border-border/50 bg-card/90 px-3 py-1.5 text-xs text-muted-foreground backdrop-blur-sm shadow-sm select-none">
                  <span className="font-medium text-foreground">{agentCount}</span>
                  <span>{agentCount === 1 ? 'bot' : 'bots'}</span>
                </div>
              )}
              <ConnectionIndicator state={connectionState} onReconnect={reconnect} />
            </div>
          </div>

          {/* ActionPill */}
          <ActionPill />

          {/* Onboarding: empty canvas with active space but no bots */}
          {showOnboarding && (
            <div className="absolute inset-0 z-10 flex items-center justify-center pointer-events-none">
              <div className="pointer-events-auto px-4 py-8">
                <FirstBotOnboarding />
              </div>
            </div>
          )}

          {/* Loading spaces */}
          {!activeSpace && (!spaceBootstrapped || spaceLoading) && (
            <div className="absolute inset-0 z-10 flex items-center justify-center pointer-events-none">
              <div className="flex flex-col items-center gap-3 text-center px-4">
                <div className="h-5 w-5 animate-spin rounded-full border-2 border-muted-foreground border-t-transparent" />
                <p className="text-sm text-muted-foreground">Loading workspace...</p>
              </div>
            </div>
          )}

          {/* No space selected — only after bootstrap completes with no result */}
          {!activeSpace && spaceBootstrapped && !spaceLoading && (
            <div className="absolute inset-0 z-10 flex items-center justify-center pointer-events-none">
              <div className="flex flex-col items-center gap-3 text-center pointer-events-auto px-4">
                <h2 className="text-lg font-semibold">No space selected</h2>
                <p className="text-sm text-muted-foreground max-w-xs">
                  Go to <a href="/spaces" className="text-primary underline">Spaces</a> to create or select a workspace.
                </p>
              </div>
            </div>
          )}

          {/* Disconnected state (only when space is active and has bots) */}
          {activeSpace && spaceBots.length > 0 && !isConnected && connectionState !== 'connecting' && connectionState !== 'authenticating' && connectionState !== 'reconnecting' && (
            <div className="absolute inset-0 z-10 flex items-center justify-center pointer-events-none">
              <div className="flex flex-col items-center gap-3 text-center pointer-events-auto px-4">
                <h2 className="text-lg font-semibold">Reconnecting...</h2>
                <p className="text-sm text-muted-foreground max-w-xs">
                  Connection lost. Click the indicator to reconnect.
                </p>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* CommandPalette (Cmd+K) */}
      <CommandPalette />
    </ReactFlowProvider>
  );
}
