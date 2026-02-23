import { create } from 'zustand';
import { spaceApi, type Space, type SpaceNode, type SpaceBot, type SpaceMember } from '@/services/spaceApi';
import { teamPlatformStorage } from '@/services/teamPlatformApi';

interface SpaceState {
  // Active space
  activeSpaceId: string | null;
  activeSpace: Space | null;

  // Lists
  spaces: Space[];
  nodes: SpaceNode[];
  bots: SpaceBot[];
  members: SpaceMember[];

  // Loading
  loading: boolean;

  // Actions
  fetchSpaces: () => Promise<void>;
  setActiveSpace: (id: string) => Promise<void>;
  createSpace: (name: string, description?: string) => Promise<Space>;
  deleteSpace: (id: string) => Promise<void>;

  // Nodes
  fetchNodes: () => Promise<void>;
  registerNode: (data: { name: string; type?: string; endpoint?: string; os?: string }) => Promise<SpaceNode>;
  removeNode: (nodeId: string) => Promise<void>;

  // Bots
  fetchBots: () => Promise<void>;
  createBot: (data: { node_id: string; name: string; model?: string; view?: string }) => Promise<SpaceBot>;
  removeBot: (botId: string) => Promise<void>;

  // Members
  fetchMembers: () => Promise<void>;
  addMember: (userId: string, role: string) => Promise<void>;
  removeMember: (userId: string) => Promise<void>;

  // Reset
  reset: () => void;
}

const STORAGE_KEY = 'hanzo-space-active';

/** Create a client-side default space when the API backend is unavailable. */
function makeLocalDefaultSpace(): Space {
  const id = localStorage.getItem(STORAGE_KEY) || `local-${Date.now()}`;
  return {
    id,
    org_id: 'local',
    name: 'Default',
    slug: 'default',
    description: 'Auto-created workspace',
    created_by: 'system',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  };
}

export const useSpaceStore = create<SpaceState>((set, get) => ({
  activeSpaceId: localStorage.getItem(STORAGE_KEY),
  activeSpace: null,
  spaces: [],
  nodes: [],
  bots: [],
  members: [],
  loading: false,

  fetchSpaces: async () => {
    set({ loading: true });
    let spaces: Space[] = [];
    let apiAvailable = true;

    try {
      const result = await spaceApi.list();
      spaces = result.spaces;
    } catch {
      apiAvailable = false;
    }

    // Auto-create a default space when none exist
    if (spaces.length === 0) {
      if (apiAvailable) {
        try {
          const defaultSpace = await spaceApi.create({ name: 'Default', description: 'Auto-created workspace' });
          spaces = [defaultSpace];
        } catch {
          // API create failed — fall back to local space
          spaces = [makeLocalDefaultSpace()];
        }
      } else {
        // API unreachable — create a local-only space so the UI never blocks
        spaces = [makeLocalDefaultSpace()];
      }
    }

    set({ spaces, loading: false });

    const { activeSpaceId } = get();

    // If previously active space was deleted, fall back to first available
    if (activeSpaceId && !spaces.find(s => s.id === activeSpaceId)) {
      if (spaces.length > 0) {
        await get().setActiveSpace(spaces[0].id);
      } else {
        localStorage.removeItem(STORAGE_KEY);
        set({ activeSpaceId: null, activeSpace: null, nodes: [], bots: [], members: [] });
      }
      return;
    }

    // Auto-select first space if none active
    if (!activeSpaceId && spaces.length > 0) {
      await get().setActiveSpace(spaces[0].id);
    } else if (activeSpaceId) {
      // Refresh active space details
      const active = spaces.find(s => s.id === activeSpaceId);
      if (active) {
        set({ activeSpace: active });
      }
    }
  },

  setActiveSpace: async (id: string) => {
    localStorage.setItem(STORAGE_KEY, id);
    try {
      const space = await spaceApi.get(id);
      set({ activeSpaceId: id, activeSpace: space });
      // Refresh nodes, bots, and members for the new space
      await Promise.all([get().fetchNodes(), get().fetchBots(), get().fetchMembers()]);
    } catch {
      // API unavailable — use locally-known space or create a fallback
      const known = get().spaces.find(s => s.id === id);
      set({ activeSpaceId: id, activeSpace: known ?? makeLocalDefaultSpace() });
    }
  },

  createSpace: async (name: string, description?: string) => {
    const space = await spaceApi.create({ name, description });
    set(s => ({ spaces: [space, ...s.spaces] }));
    return space;
  },

  deleteSpace: async (id: string) => {
    await spaceApi.delete(id);
    teamPlatformStorage.remove(id);
    set(s => ({
      spaces: s.spaces.filter(sp => sp.id !== id),
      ...(s.activeSpaceId === id ? { activeSpaceId: null, activeSpace: null, nodes: [], bots: [], members: [] } : {}),
    }));
  },

  fetchNodes: async () => {
    const { activeSpaceId } = get();
    if (!activeSpaceId) return;
    try {
      const { nodes } = await spaceApi.listNodes(activeSpaceId);
      set({ nodes });
    } catch { /* ignore */ }
  },

  registerNode: async (data) => {
    const { activeSpaceId } = get();
    if (!activeSpaceId) throw new Error('No active space');
    const node = await spaceApi.registerNode(activeSpaceId, data);
    set(s => ({ nodes: [node, ...s.nodes] }));
    return node;
  },

  removeNode: async (nodeId: string) => {
    const { activeSpaceId } = get();
    if (!activeSpaceId) return;
    await spaceApi.removeNode(activeSpaceId, nodeId);
    set(s => ({ nodes: s.nodes.filter(n => n.node_id !== nodeId) }));
  },

  fetchBots: async () => {
    const { activeSpaceId } = get();
    if (!activeSpaceId) return;
    try {
      const { bots } = await spaceApi.listBots(activeSpaceId);
      set({ bots });
    } catch { /* ignore */ }
  },

  createBot: async (data) => {
    const { activeSpaceId } = get();
    if (!activeSpaceId) throw new Error('No active space');
    const bot = await spaceApi.createBot(activeSpaceId, data);
    set(s => ({ bots: [bot, ...s.bots] }));
    return bot;
  },

  removeBot: async (botId: string) => {
    const { activeSpaceId } = get();
    if (!activeSpaceId) return;
    await spaceApi.removeBot(activeSpaceId, botId);
    set(s => ({ bots: s.bots.filter(b => b.bot_id !== botId) }));
  },

  fetchMembers: async () => {
    const { activeSpaceId } = get();
    if (!activeSpaceId) return;
    try {
      const { members } = await spaceApi.listMembers(activeSpaceId);
      set({ members });
    } catch { /* ignore */ }
  },

  addMember: async (userId: string, role: string) => {
    const { activeSpaceId } = get();
    if (!activeSpaceId) throw new Error('No active space');
    const member = await spaceApi.addMember(activeSpaceId, { user_id: userId, role });
    set(s => ({ members: [member, ...s.members] }));
  },

  removeMember: async (userId: string) => {
    const { activeSpaceId } = get();
    if (!activeSpaceId) return;
    await spaceApi.removeMember(activeSpaceId, userId);
    set(s => ({ members: s.members.filter(m => m.user_id !== userId) }));
  },

  reset: () => {
    localStorage.removeItem(STORAGE_KEY);
    set({
      activeSpaceId: null,
      activeSpace: null,
      spaces: [],
      nodes: [],
      bots: [],
      members: [],
      loading: false,
    });
  },
}));
