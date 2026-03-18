/**
 * CursorOverlay
 *
 * SVG overlay rendered inside ReactFlow that shows colored cursors
 * for each connected peer. Cursors fade out after 5 seconds of no movement.
 */

import { useEffect, useState } from 'react';
import { usePresenceStore, type PeerPresence } from '@/stores/presenceStore';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const FADE_TIMEOUT_MS = 5000;
const TICK_INTERVAL_MS = 1000;

// ---------------------------------------------------------------------------
// CursorOverlay
// ---------------------------------------------------------------------------

export function CursorOverlay() {
  const peers = usePresenceStore((s) => s.peers);
  const [now, setNow] = useState(Date.now());

  // Tick every second to update fade states
  useEffect(() => {
    const interval = setInterval(() => setNow(Date.now()), TICK_INTERVAL_MS);
    return () => clearInterval(interval);
  }, []);

  const entries = Array.from(peers.values()).filter(
    (p): p is PeerPresence & { cursor: { x: number; y: number } } =>
      p.cursor !== undefined
  );

  if (entries.length === 0) return null;

  return (
    <svg
      className="pointer-events-none absolute inset-0 z-50 overflow-visible"
      style={{ width: '100%', height: '100%' }}
    >
      {entries.map((peer) => {
        const age = now - peer.lastSeen;
        const opacity = age > FADE_TIMEOUT_MS ? 0 : age > FADE_TIMEOUT_MS - 1000 ? 1 - (age - (FADE_TIMEOUT_MS - 1000)) / 1000 : 1;

        return (
          <g
            key={peer.userId}
            style={{
              transform: `translate(${peer.cursor.x}px, ${peer.cursor.y}px)`,
              transition: 'transform 100ms ease-out, opacity 300ms ease',
              opacity,
            }}
          >
            {/* Arrow cursor */}
            <path
              d="M0 0 L0 16 L4.5 12.5 L8 20 L10.5 19 L7 11.5 L12 11.5 Z"
              fill={peer.color}
              stroke="white"
              strokeWidth="1"
            />
            {/* Name label */}
            <g transform="translate(14, 14)">
              <rect
                rx="3"
                ry="3"
                x="0"
                y="0"
                width={peer.displayName.length * 7 + 12}
                height="20"
                fill={peer.color}
                opacity="0.9"
              />
              <text
                x="6"
                y="14"
                fontSize="11"
                fontFamily="system-ui, sans-serif"
                fontWeight="500"
                fill="white"
              >
                {peer.displayName}
              </text>
            </g>
          </g>
        );
      })}
    </svg>
  );
}
