// Space API client — typed REST calls to /api/v1/spaces/*

import { getGlobalIamToken, getGlobalApiKey } from './api';

const BASE = import.meta.env.VITE_API_BASE_URL || '/api/v1';

function headers(): HeadersInit {
  const h: HeadersInit = { 'Content-Type': 'application/json' };
  const iamToken = getGlobalIamToken();
  if (iamToken) {
    h['Authorization'] = `Bearer ${iamToken}`;
  } else {
    const apiKey = getGlobalApiKey() || localStorage.getItem('playground-api-key');
    if (apiKey) h['X-API-Key'] = apiKey;
  }
  return h;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, { headers: headers(), ...init });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`${res.status}: ${body}`);
  }
  return res.json();
}

// --- Types ---

export interface Space {
  id: string;
  org_id: string;
  name: string;
  slug: string;
  description: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface SpaceMember {
  space_id: string;
  user_id: string;
  role: 'owner' | 'admin' | 'member' | 'viewer';
  created_at: string;
}

export interface SpaceNode {
  space_id: string;
  node_id: string;
  name: string;
  type: 'local' | 'cloud';
  endpoint: string;
  status: 'online' | 'offline' | 'provisioning';
  os: string;
  registered_at: string;
  last_seen: string;
}

export interface SpaceBot {
  space_id: string;
  bot_id: string;
  node_id: string;
  agent_id: string;
  name: string;
  model: string;
  view: 'terminal' | 'desktop-linux' | 'desktop-mac' | 'desktop-win' | 'chat';
  status: string;
}

// --- Space CRUD ---

export const spaceApi = {
  list: () => request<{ spaces: Space[] }>('/spaces'),

  get: (id: string) => request<Space>(`/spaces/${id}`),

  create: (data: { name: string; slug?: string; description?: string }) =>
    request<Space>('/spaces', { method: 'POST', body: JSON.stringify(data) }),

  update: (id: string, data: { name: string; slug?: string; description?: string }) =>
    request<Space>(`/spaces/${id}`, { method: 'PUT', body: JSON.stringify(data) }),

  delete: (id: string) =>
    request<{ deleted: boolean }>(`/spaces/${id}`, { method: 'DELETE' }),

  // Members
  listMembers: (id: string) =>
    request<{ members: SpaceMember[] }>(`/spaces/${id}/members`),

  addMember: (id: string, data: { user_id: string; role: string }) =>
    request<SpaceMember>(`/spaces/${id}/members`, { method: 'POST', body: JSON.stringify(data) }),

  removeMember: (id: string, uid: string) =>
    request<{ removed: boolean }>(`/spaces/${id}/members/${uid}`, { method: 'DELETE' }),

  // Nodes
  listNodes: (id: string) =>
    request<{ nodes: SpaceNode[] }>(`/spaces/${id}/nodes`),

  registerNode: (id: string, data: { name: string; type?: string; endpoint?: string; os?: string }) =>
    request<SpaceNode>(`/spaces/${id}/nodes/register`, { method: 'POST', body: JSON.stringify(data) }),

  removeNode: (id: string, nid: string) =>
    request<{ removed: boolean }>(`/spaces/${id}/nodes/${nid}`, { method: 'DELETE' }),

  // Bots
  listBots: (id: string) =>
    request<{ bots: SpaceBot[] }>(`/spaces/${id}/bots`),

  createBot: (id: string, data: { node_id: string; name: string; model?: string; view?: string }) =>
    request<SpaceBot>(`/spaces/${id}/bots`, { method: 'POST', body: JSON.stringify(data) }),

  removeBot: (id: string, bid: string) =>
    request<{ removed: boolean }>(`/spaces/${id}/bots/${bid}`, { method: 'DELETE' }),

  sendChat: (id: string, bid: string, body: unknown) =>
    request<unknown>(`/spaces/${id}/bots/${bid}/chat`, { method: 'POST', body: JSON.stringify(body) }),

  getChatHistory: (id: string, bid: string) =>
    request<unknown>(`/spaces/${id}/bots/${bid}/chat`),

  // Node V2 proxy
  proxyV2: (spaceId: string, nodeId: string, path: string, init?: RequestInit) =>
    request<unknown>(`/spaces/${spaceId}/nodes/${nodeId}/v2/${path}`, init),

  // Cloud provisioning
  provisionCloudNode: (data: {
    node_id?: string;
    display_name?: string;
    model?: string;
    os?: string;         // "linux" (default), "terminal", "macos", "windows"
    cpu?: string;        // e.g. "500m", "1000m"
    memory?: string;     // e.g. "512Mi", "2Gi"
    image?: string;      // override agent image
    workspace?: string;
    env?: Record<string, string>;
  }) =>
    request<{ node_id: string; pod_name: string; namespace: string; status: string; endpoint: string; created_at: string }>(
      '/cloud/nodes/provision',
      { method: 'POST', body: JSON.stringify(data) },
    ),

  deprovisionCloudNode: (nodeId: string) =>
    request<{ status: string; node_id: string }>(`/cloud/nodes/${encodeURIComponent(nodeId)}`, { method: 'DELETE' }),

  getCloudNode: (nodeId: string) =>
    request<{ node_id: string; pod_name: string; status: string; os: string }>(`/cloud/nodes/${encodeURIComponent(nodeId)}`),

  getCloudLogs: (nodeId: string, tail?: number) =>
    request<{ node_id: string; logs: string }>(`/cloud/nodes/${encodeURIComponent(nodeId)}/logs?tail=${tail ?? 100}`),

  listCloudNodes: () =>
    request<{ nodes: SpaceNode[]; count: number }>('/cloud/nodes'),

  syncCloudNodes: () =>
    request<{ status: string }>('/cloud/nodes/sync', { method: 'POST' }),
};
