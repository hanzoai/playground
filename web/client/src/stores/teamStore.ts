/**
 * Team Store (Zustand)
 *
 * Manages team presets and provisioned teams.
 */

import { create } from 'zustand';
import type { TeamPreset, Team } from '@/types/team';

interface TeamStore {
  presets: TeamPreset[];
  teams: Team[];
  loading: boolean;

  setPresets: (presets: TeamPreset[]) => void;
  addTeam: (team: Team) => void;
  updateTeam: (teamId: string, data: Partial<Team>) => void;
  removeTeam: (teamId: string) => void;
  setLoading: (loading: boolean) => void;
  /** Reset all state (call on logout/tenant switch) */
  reset: () => void;
}

export const useTeamStore = create<TeamStore>((set, get) => ({
  presets: [],
  teams: [],
  loading: false,

  setPresets: (presets) => set({ presets }),

  addTeam: (team) => set({ teams: [...get().teams, team] }),

  updateTeam: (teamId, data) => {
    set({
      teams: get().teams.map((t) =>
        t.id === teamId ? { ...t, ...data } : t
      ),
    });
  },

  removeTeam: (teamId) => {
    set({ teams: get().teams.filter((t) => t.id !== teamId) });
  },

  setLoading: (loading) => set({ loading }),

  /** Reset all state (call on logout/tenant switch) */
  reset: () => set({ presets: [], teams: [], loading: false }),
}));
