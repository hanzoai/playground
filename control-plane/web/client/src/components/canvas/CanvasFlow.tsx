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
import { nodeTypes } from './nodes/registry';
import { CanvasControls } from './CanvasControls';
import { CanvasContextMenu } from './CanvasContextMenu';

// ---------------------------------------------------------------------------
// CanvasFlow
// ---------------------------------------------------------------------------

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

  const [nodes, setNodes, onNodesChange] = useNodesState(storeNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(storeEdges);
  const { fitView, screenToFlowPosition } = useReactFlow();
  const persistTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Context menu
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number } | null>(null);

  // Sync store → local when store changes externally
  useEffect(() => { setNodes(storeNodes); }, [storeNodes, setNodes]);
  useEffect(() => { setEdges(storeEdges); }, [storeEdges, setEdges]);

  const debouncedPersist = useCallback(() => {
    if (persistTimer.current) clearTimeout(persistTimer.current);
    persistTimer.current = setTimeout(persist, 1000);
  }, [persist]);

  const handleNodesChange: OnNodesChange = useCallback(
    (changes) => {
      onNodesChange(changes);
      requestAnimationFrame(() => {
        storeSetNodes(nodes);
        debouncedPersist();
      });
    },
    [onNodesChange, storeSetNodes, nodes, debouncedPersist]
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
      setContextMenu({ x: event.clientX, y: event.clientY });
    },
    []
  );

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
          size={1}
          className="!bg-background"
        />
        <MiniMap
          className="!bg-card/80 !border-border/40 !rounded-lg !shadow-lg hidden md:block"
          maskColor="rgba(0, 0, 0, 0.15)"
          nodeStrokeWidth={2}
          pannable
          zoomable
        />
        <CanvasControls onFitView={() => fitView({ padding: 0.3 })} />
      </ReactFlow>

      <CanvasContextMenu
        position={contextMenu}
        onClose={() => setContextMenu(null)}
        onAddBot={handleAddBot}
        onAddStarter={handleAddStarter}
      />
    </div>
  );
}
