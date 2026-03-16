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
const STORAGE_ENV_KEY = 'hanzo_environment';
const STORAGE_KNOWN_ORGS_KEY = 'hanzo_iam_known_orgs';

export type Environment = string;

/** Default environment. Additional environments are created by the user. */
export const DEFAULT_ENVIRONMENT: { id: string; name: string } = {
  id: 'production',
  name: 'Production',
};

/** Locally-created org entry persisted in localStorage. */
export interface KnownOrg {
  name: string;
  displayName: string;
}

interface TenantState {
  orgId: string | null;
  projectId: string | null;
  environment: Environment;
  /** Orgs created locally that may not yet appear in the IAM API response. */
  knownOrgs: KnownOrg[];
  setOrg: (orgId: string | null) => void;
  setProject: (projectId: string | null) => void;
  setEnvironment: (env: Environment) => void;
  /** Register a newly-created org so it appears in the switcher immediately. */
  addKnownOrg: (org: KnownOrg) => void;
  reset: () => void;
}

export const useTenantStore = create<TenantState>((set) => ({
  orgId: (() => {
    try { return localStorage.getItem(STORAGE_ORG_KEY); } catch { return null; }
  })(),
  projectId: (() => {
    try { return localStorage.getItem(STORAGE_PROJECT_KEY); } catch { return null; }
  })(),
  environment: (() => {
    try {
      const stored = localStorage.getItem(STORAGE_ENV_KEY);
      if (!stored || stored === 'default') return 'production' as Environment;
      return stored as Environment;
    } catch { return 'production' as Environment; }
  })(),
  knownOrgs: (() => {
    try {
      const stored = localStorage.getItem(STORAGE_KNOWN_ORGS_KEY);
      return stored ? JSON.parse(stored) as KnownOrg[] : [];
    } catch { return []; }
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

  setEnvironment: (env) => {
    set({ environment: env });
    try { localStorage.setItem(STORAGE_ENV_KEY, env); } catch { /* ok */ }
  },

  addKnownOrg: (org) => {
    set((state) => {
      if (state.knownOrgs.some((o) => o.name === org.name)) return state;
      const next = [...state.knownOrgs, org];
      try { localStorage.setItem(STORAGE_KNOWN_ORGS_KEY, JSON.stringify(next)); } catch { /* ok */ }
      return { knownOrgs: next };
    });
  },

  reset: () => {
    set({ orgId: null, projectId: null, environment: 'production', knownOrgs: [] });
    try {
      localStorage.removeItem(STORAGE_ORG_KEY);
      localStorage.removeItem(STORAGE_PROJECT_KEY);
      localStorage.removeItem(STORAGE_ENV_KEY);
      localStorage.removeItem(STORAGE_KNOWN_ORGS_KEY);
    } catch { /* ok */ }
  },
}));
