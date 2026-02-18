/**
 * Tenant Store (Zustand)
 *
 * Manages current org/project selection for multi-tenant mode.
 * Reads from/writes to localStorage for persistence across sessions.
 * Used by useGateway to pass tenant context to the WebSocket gateway.
 */

import { create } from 'zustand';

const STORAGE_ORG_KEY = 'hanzo_iam_current_org';
const STORAGE_PROJECT_KEY = 'hanzo_iam_current_project';

interface TenantState {
  orgId: string | null;
  projectId: string | null;
  setOrg: (orgId: string | null) => void;
  setProject: (projectId: string | null) => void;
  reset: () => void;
}

export const useTenantStore = create<TenantState>((set) => ({
  orgId: (() => {
    try { return localStorage.getItem(STORAGE_ORG_KEY); } catch { return null; }
  })(),
  projectId: (() => {
    try { return localStorage.getItem(STORAGE_PROJECT_KEY); } catch { return null; }
  })(),

  setOrg: (orgId) => {
    set({ orgId, projectId: null });
    try {
      if (orgId) localStorage.setItem(STORAGE_ORG_KEY, orgId);
      else localStorage.removeItem(STORAGE_ORG_KEY);
      localStorage.removeItem(STORAGE_PROJECT_KEY);
    } catch { /* ok */ }
  },

  setProject: (projectId) => {
    set({ projectId });
    try {
      if (projectId) localStorage.setItem(STORAGE_PROJECT_KEY, projectId);
      else localStorage.removeItem(STORAGE_PROJECT_KEY);
    } catch { /* ok */ }
  },

  reset: () => {
    set({ orgId: null, projectId: null });
    try {
      localStorage.removeItem(STORAGE_ORG_KEY);
      localStorage.removeItem(STORAGE_PROJECT_KEY);
    } catch { /* ok */ }
  },
}));
