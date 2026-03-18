/**
 * CanvasFlow - Core ReactFlow Canvas
 *
 * Full-bleed infinite canvas with bot nodes.
 * Touch support for mobile/tablet.
 */

import { useCallback, useRef, useEffect, useState } from 'react';
import {
  ReactFlow,
  Background,
  BackgroundVariant,
  MiniMap,
  useNodesState,
  useEdgesState,
  useReactFlow,
  addEdge,
  type OnConnect,
  type OnNodesChange,
  type OnEdgesChange,
  type NodeMouseHandler,
  type Connection,
  type Edge,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';

import { useCanvasStore } from '@/stores/canvasStore';
import { usePresenceStore } from '@/stores/presenceStore';
import { gateway } from '@/services/gatewayClient';
import { nodeTypes } from './nodes/registry';
import { CanvasControls } from './CanvasControls';
import { CanvasContextMenu } from './CanvasContextMenu';
import { BotContextMenu, type BotContextMenuState } from './BotContextMenu';
import { CursorOverlay } from './CursorOverlay';
import { SpaceChatSidebar } from '@/components/chat/SpaceChatSidebar';
import { spaceApi } from '@/services/spaceApi';
import type { Bot } from '@/types/canvas';

// ---------------------------------------------------------------------------
// CanvasFlow
// ---------------------------------------------------------------------------

// Throttle interval for cursor broadcasts (10 Hz = 100ms)
const CURSOR_THROTTLE_MS = 100;

export function CanvasFlow({ className }: { className?: string }) {
  const storeNodes = useCanvasStore((s) => s.nodes);
  const storeEdges = useCanvasStore((s) => s.edges);
  const storeViewport = useCanvasStore((s) => s.viewport);
  const storeSetNodes = useCanvasStore((s) => s.setNodes);
  const storeSetEdges = useCanvasStore((s) => s.setEdges);
  const storeSetViewport = useCanvasStore((s) => s.setViewport);
  const selectNode = useCanvasStore((s) => s.selectNode);
  const persist = useCanvasStore((s) => s.persist);
  const upsertBot = useCanvasStore((s) => s.upsertBot);
  const addStarter = useCanvasStore((s) => s.addStarter);
  const autoLayout = useCanvasStore((s) => s.autoLayout);

  const addPeer = usePresenceStore((s) => s.addPeer);
  const removePeer = usePresenceStore((s) => s.removePeer);
  const updateCursor = usePresenceStore((s) => s.updateCursor);

  const [nodes, setNodes, onNodesChange] = useNodesState(storeNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(storeEdges);
  const { fitView, screenToFlowPosition } = useReactFlow();
  const persistTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastCursorBroadcast = useRef(0);

  // Context menus
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number } | null>(null);
  const [botContextMenu, setBotContextMenu] = useState<BotContextMenuState | null>(null);

  // Chat sidebar state
  const [chatOpen, setChatOpen] = useState(false);
  const [chatUnread, setChatUnread] = useState(0);

  // Subscribe to gateway presence events
  useEffect(() => {
    const unsubCursor = gateway.on('presence.cursor.update', (payload) => {
      const data = payload as { userId: string; displayName: string; avatar?: string; x: number; y: number };
      updateCursor(data.userId, { x: data.x, y: data.y });
    });

    const unsubJoin = gateway.on('presence.join', (payload) => {
      const data = payload as { userId: string; displayName: string; avatar?: string; color?: string };
      addPeer({ userId: data.userId, displayName: data.displayName, avatar: data.avatar, color: data.color ?? '', lastSeen: Date.now() });
    });

    const unsubLeave = gateway.on('presence.leave', (payload) => {
      const data = payload as { userId: string };
      removePeer(data.userId);
    });

    return () => {
      unsubCursor();
      unsubJoin();
      unsubLeave();
    };
  }, [addPeer, removePeer, updateCursor]);

  // Throttled cursor broadcast
  const broadcastCursor = useCallback(
    (event: React.MouseEvent) => {
      const now = Date.now();
      if (now - lastCursorBroadcast.current < CURSOR_THROTTLE_MS) return;
      lastCursorBroadcast.current = now;

      const flowPos = screenToFlowPosition({ x: event.clientX, y: event.clientY });
      if (gateway.isConnected) {
        gateway.rpc('presence.cursor', { x: flowPos.x, y: flowPos.y }).catch(() => {
          // Gateway may not support this RPC yet; silently ignore
        });
      }
    },
    [screenToFlowPosition]
  );

  // Sync store → local when store changes externally
  useEffect(() => { setNodes(storeNodes); }, [storeNodes, setNodes]);
  useEffect(() => { setEdges(storeEdges); }, [storeEdges, setEdges]);

  const debouncedPersist = useCallback(() => {
    if (persistTimer.current) clearTimeout(persistTimer.current);
    persistTimer.current = setTimeout(persist, 1000);
  }, [persist]);

  // Use a ref to avoid stale closure capturing old `nodes` in rAF callbacks.
  const nodesRef = useRef(nodes);
  nodesRef.current = nodes;

  const handleNodesChange: OnNodesChange = useCallback(
    (changes) => {
      onNodesChange(changes);
      requestAnimationFrame(() => {
        storeSetNodes(nodesRef.current);
        debouncedPersist();
      });
    },
    [onNodesChange, storeSetNodes, debouncedPersist]
  );

  const handleEdgesChange: OnEdgesChange = useCallback(
    (changes) => {
      onEdgesChange(changes);
      requestAnimationFrame(() => {
        storeSetEdges(edges);
        debouncedPersist();
      });
    },
    [onEdgesChange, storeSetEdges, edges, debouncedPersist]
  );

  const handleConnect: OnConnect = useCallback(
    (connection: Connection) => {
      setEdges((eds: Edge[]) => addEdge(connection, eds));
      debouncedPersist();
    },
    [setEdges, debouncedPersist]
  );

  const handleNodeClick: NodeMouseHandler = useCallback(
    (_event, node) => selectNode(node.id),
    [selectNode]
  );

  const handlePaneClick = useCallback(() => {
    selectNode(null);
    setContextMenu(null);
    setBotContextMenu(null);
  }, [selectNode]);

  const handleMoveEnd = useCallback(
    (_event: unknown, vp: { x: number; y: number; zoom: number }) => {
      storeSetViewport(vp);
      debouncedPersist();
    },
    [storeSetViewport, debouncedPersist]
  );

  const handleContextMenu = useCallback(
    (event: React.MouseEvent) => {
      event.preventDefault();
      setBotContextMenu(null);
      setContextMenu({ x: event.clientX, y: event.clientY });
    },
    []
  );

  const handleNodeContextMenu = useCallback(
    (event: React.MouseEvent, node: { id: string; type?: string; data: Record<string, unknown> }) => {
      event.preventDefault();
      event.stopPropagation();
      setContextMenu(null);
      if (node.type === 'bot') {
        const bot = node.data as unknown as Bot;
        setBotContextMenu({
          x: event.clientX,
          y: event.clientY,
          agentId: bot.agentId,
          sessionKey: bot.sessionKey,
          status: bot.status,
        });
      }
    },
    []
  );

  const handleAutoLayout = useCallback(() => {
    autoLayout();
    setTimeout(() => fitView({ padding: 0.3, duration: 300 }), 50);
    debouncedPersist();
  }, [autoLayout, fitView, debouncedPersist]);

  const handleAddBot = useCallback(
    (pos: { x: number; y: number }) => {
      const flowPos = screenToFlowPosition(pos);
      upsertBot(`new-${Date.now()}`, {
        name: 'New Bot',
        status: 'idle',
        source: 'cloud',
      }, flowPos);
    },
    [screenToFlowPosition, upsertBot]
  );

  const handleAddStarter = useCallback(
    (pos: { x: number; y: number }) => {
      const flowPos = screenToFlowPosition(pos);
      addStarter(flowPos);
    },
    [screenToFlowPosition, addStarter]
  );

  const handleLaunchCloud = useCallback(
    async (type: 'linux' | 'terminal' | 'desktop') => {
      const displayName = `bot-${Date.now().toString(36)}`;
      const os = type === 'desktop' ? 'linux' : type;
      console.info(`[cloud] Provisioning ${type} bot (os=${os})...`);
      try {
        const result = await spaceApi.provisionCloudNode({
          display_name: displayName,
          os,
        });
        console.info(`[cloud] Provisioned: node_id=${result.node_id} pod=${result.pod_name} status=${result.status}`);
        // Add to canvas using the real node ID from the API
        const center = { x: window.innerWidth / 2, y: window.innerHeight / 2 };
        const flowPos = screenToFlowPosition(center);
        upsertBot(result.node_id, {
          name: displayName,
          status: 'provisioning',
          source: 'cloud',
        }, flowPos);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        console.error(`[cloud] Provisioning failed (os=${os}): ${msg}`);
      }
    },
    [screenToFlowPosition, upsertBot]
  );

  return (
    <div className={`h-full w-full ${className ?? ''}`}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodesChange={handleNodesChange}
        onEdgesChange={handleEdgesChange}
        onConnect={handleConnect}
        onNodeClick={handleNodeClick}
        onPaneClick={handlePaneClick}
        onMoveEnd={handleMoveEnd}
        onContextMenu={handleContextMenu}
        onNodeContextMenu={handleNodeContextMenu}
        onMouseMove={broadcastCursor}
        defaultViewport={storeViewport}
        fitView={nodes.length > 0}
        fitViewOptions={{ padding: 0.3, maxZoom: 1.2 }}
        minZoom={0.1}
        maxZoom={4}
        snapToGrid
        snapGrid={[20, 20]}
        panOnDrag
        zoomOnScroll
        zoomOnPinch
        panOnScroll={false}
        selectNodesOnDrag={false}
        nodesDraggable
        proOptions={{ hideAttribution: true }}
        className="touch-manipulation"
      >
        <Background
          variant={BackgroundVariant.Dots}
          gap={20}
          size={1.2}
          color="color-mix(in oklch, var(--muted-foreground) 25%, transparent)"
          className="!bg-background"
        />
        {nodes.filter(n => n.type === 'bot').length >= 2 && (
          <MiniMap
            className="canvas-minimap hidden md:block"
            style={{ backgroundColor: '#161616', width: 180, height: 120 }}
            maskColor="rgba(0, 0, 0, 0.55)"
            maskStrokeColor="rgba(255, 255, 255, 0.15)"
            maskStrokeWidth={1}
            nodeColor={(node) => {
              if (node.type === 'starter') return '#6b7280';
              if (node.type === 'team') return '#818cf8';
              const status = (node.data as Record<string, unknown>)?.status as string;
              if (status === 'busy') return '#34d399';
              if (status === 'error') return '#f87171';
              if (status === 'waiting') return '#fbbf24';
              if (status === 'provisioning') return '#c084fc';
              if (status === 'offline') return '#6b7280';
              return '#60a5fa';
            }}
            nodeStrokeColor="rgba(255, 255, 255, 0.08)"
            nodeStrokeWidth={1}
            nodeBorderRadius={3}
            pannable
            zoomable
          />
        )}
        <CursorOverlay />
        <CanvasControls
          onFitView={() => fitView({ padding: 0.3 })}
          onAutoLayout={handleAutoLayout}
          onAddBot={handleAddBot}
          onAddStarter={handleAddStarter}
          onLaunchCloud={handleLaunchCloud}
          onToggleChat={() => setChatOpen((o) => !o)}
          chatUnread={chatUnread}
        />
      </ReactFlow>

      <CanvasContextMenu
        position={contextMenu}
        onClose={() => setContextMenu(null)}
        onAddBot={handleAddBot}
        onAddStarter={handleAddStarter}
      />

      <BotContextMenu
        state={botContextMenu}
        onClose={() => setBotContextMenu(null)}
      />

      <SpaceChatSidebar
        open={chatOpen}
        onClose={() => setChatOpen(false)}
        onUnreadChange={setChatUnread}
      />
    </div>
  );
}
