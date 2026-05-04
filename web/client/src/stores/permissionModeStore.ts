/**
 * Permission Mode Store (Zustand)
 *
 * Manages plan/auto-accept/ask modes per agent and globally.
 */

import { create } from 'zustand';

export type PermissionMode = 'plan' | 'auto-accept' | 'ask';

const MODES: PermissionMode[] = ['plan', 'auto-accept', 'ask'];

interface PermissionModeStore {
  global: PermissionMode;
  overrides: Map<string, PermissionMode>;

  setGlobal: (mode: PermissionMode) => void;
  setAgent: (agentId: string, mode: PermissionMode) => void;
  clearAgent: (agentId: string) => void;
  getEffective: (agentId: string) => PermissionMode;
  cycleGlobal: () => void;
  cycleAgent: (agentId: string) => void;
  /** Reset all state (call on logout/tenant switch) */
  reset: () => void;
}

export const usePermissionModeStore = create<PermissionModeStore>((set, get) => ({
  global: 'ask',
  overrides: new Map(),

  setGlobal: (mode) => set({ global: mode }),

  setAgent: (agentId, mode) => {
    const next = new Map(get().overrides);
    next.set(agentId, mode);
    set({ overrides: next });
  },

  clearAgent: (agentId) => {
    const next = new Map(get().overrides);
    next.delete(agentId);
    set({ overrides: next });
  },

  getEffective: (agentId) => {
    return get().overrides.get(agentId) ?? get().global;
  },

  cycleGlobal: () => {
    const { global } = get();
    const idx = MODES.indexOf(global);
    set({ global: MODES[(idx + 1) % MODES.length] });
  },

  cycleAgent: (agentId) => {
    const current = get().getEffective(agentId);
    const idx = MODES.indexOf(current);
    const next = new Map(get().overrides);
    next.set(agentId, MODES[(idx + 1) % MODES.length]);
    set({ overrides: next });
  },

  /** Reset all state (call on logout/tenant switch) */
  reset: () => set({ global: 'ask', overrides: new Map() }),
}));
