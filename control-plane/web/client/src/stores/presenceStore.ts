/**
 * Presence Store (Zustand)
 *
 * Tracks peer cursors and online status for multiplayer canvas.
 * Peers are identified by userId and assigned a deterministic color.
 */

import { create } from 'zustand';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface PeerPresence {
  userId: string;
  displayName: string;
  avatar?: string;
  cursor?: { x: number; y: number };
  color: string;
  lastSeen: number;
}

interface PresenceState {
  peers: Map<string, PeerPresence>;
  addPeer: (peer: PeerPresence) => void;
  removePeer: (userId: string) => void;
  updateCursor: (userId: string, cursor: { x: number; y: number }) => void;
  reset: () => void;
}

// ---------------------------------------------------------------------------
// Color palette for peer cursors
// ---------------------------------------------------------------------------

const PEER_COLORS = [
  '#3b82f6', // blue
  '#22c55e', // green
  '#a855f7', // purple
  '#f97316', // orange
  '#ec4899', // pink
  '#14b8a6', // teal
  '#eab308', // yellow
  '#ef4444', // red
  '#6366f1', // indigo
  '#06b6d4', // cyan
];

/** Assign a stable color based on userId hash. */
function colorForUser(userId: string): string {
  let hash = 0;
  for (let i = 0; i < userId.length; i++) {
    hash = ((hash << 5) - hash + userId.charCodeAt(i)) | 0;
  }
  return PEER_COLORS[Math.abs(hash) % PEER_COLORS.length];
}

// ---------------------------------------------------------------------------
// Store
// ---------------------------------------------------------------------------

export const usePresenceStore = create<PresenceState>((set, get) => ({
  peers: new Map(),

  addPeer: (peer) => {
    const peers = new Map(get().peers);
    peers.set(peer.userId, {
      ...peer,
      color: peer.color || colorForUser(peer.userId),
      lastSeen: Date.now(),
    });
    set({ peers });
  },

  removePeer: (userId) => {
    const peers = new Map(get().peers);
    peers.delete(userId);
    set({ peers });
  },

  updateCursor: (userId, cursor) => {
    const peers = new Map(get().peers);
    const existing = peers.get(userId);
    if (existing) {
      peers.set(userId, { ...existing, cursor, lastSeen: Date.now() });
    }
    set({ peers });
  },

  reset: () => set({ peers: new Map() }),
}));

export { colorForUser };
