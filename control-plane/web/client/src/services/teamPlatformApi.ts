/**
 * Hanzo Team Platform API Client
 *
 * Thin RPC client for the Hanzo Team account service.
 * Persists per-space connection config in localStorage.
 */

// --- Per-space config storage ---

export interface TeamPlatformConfig {
  accountUrl: string;
  token: string;
  workspaceId?: string;
}

const STORAGE_PREFIX = 'hanzo-team-platform-';

export const teamPlatformStorage = {
  get(spaceId: string): TeamPlatformConfig | null {
    try {
      const raw = localStorage.getItem(`${STORAGE_PREFIX}${spaceId}`);
      return raw ? JSON.parse(raw) : null;
    } catch {
      return null;
    }
  },

  set(spaceId: string, config: TeamPlatformConfig): void {
    localStorage.setItem(`${STORAGE_PREFIX}${spaceId}`, JSON.stringify(config));
  },

  remove(spaceId: string): void {
    localStorage.removeItem(`${STORAGE_PREFIX}${spaceId}`);
  },
};

// --- RPC client ---

interface RpcResponse<T = unknown> {
  result?: T;
  error?: { code: number; message: string };
}

async function rpc<T>(accountUrl: string, token: string, method: string, params?: Record<string, unknown>): Promise<T> {
  const url = accountUrl.replace(/\/+$/, '');
  const res = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ method, params: params ?? {} }),
  });

  if (!res.ok) {
    const body = await res.text();
    throw new Error(`${res.status}: ${body}`);
  }

  const data: RpcResponse<T> = await res.json();
  if (data.error) {
    throw new Error(data.error.message);
  }
  return data.result as T;
}

// --- Workspace types ---

export interface TeamWorkspace {
  id: string;
  name: string;
  description?: string;
  createdAt?: string;
}

// --- Public API ---

export const teamPlatformApi = {
  testConnection: async (accountUrl: string, token: string): Promise<boolean> => {
    // Ping the account service — any successful response means connected
    await rpc(accountUrl, token, 'ping');
    return true;
  },

  listWorkspaces: async (accountUrl: string, token: string): Promise<TeamWorkspace[]> => {
    return rpc<TeamWorkspace[]>(accountUrl, token, 'listWorkspaces');
  },

  getWorkspace: async (accountUrl: string, token: string, workspaceId: string): Promise<TeamWorkspace> => {
    return rpc<TeamWorkspace>(accountUrl, token, 'getWorkspace', { workspaceId });
  },
};
