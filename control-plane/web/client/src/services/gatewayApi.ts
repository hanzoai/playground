/**
 * Gateway API - Typed RPC Wrappers
 *
 * Thin typed wrappers over gateway.rpc() for each method
 * we use in the UI. Add new methods as needed.
 */

import { gateway } from './gatewayClient';
import { API_BASE_URL, getGlobalIamToken, getGlobalApiKey } from './api';
import type {
  AgentsCreateParams,
  AgentsCreateResult,
  AgentsListResponse,
  ChatAbortParams,
  ChatHistoryParams,
  ChatSendParams,
  ExecApprovalResolveParams,
  TeamPresetsListResult,
  TeamProvisionParams,
  TeamProvisionResult,
} from '@/types/gateway';

// ---------------------------------------------------------------------------
// Agents
// ---------------------------------------------------------------------------

export function agentsList(): Promise<AgentsListResponse> {
  return gateway.rpc<AgentsListResponse>('agents.list');
}

export function agentsCreate(params: AgentsCreateParams): Promise<AgentsCreateResult> {
  return gateway.rpc<AgentsCreateResult>('agents.create', params);
}

export function agentsDelete(agentId: string): Promise<void> {
  return gateway.rpc<void>('agents.delete', { agentId });
}

// ---------------------------------------------------------------------------
// Chat
// ---------------------------------------------------------------------------

export function chatSend(params: ChatSendParams): Promise<void> {
  return gateway.rpc<void>('chat.send', params);
}

export function chatAbort(params: ChatAbortParams): Promise<void> {
  return gateway.rpc<void>('chat.abort', params);
}

export function chatHistory(params: ChatHistoryParams): Promise<unknown[]> {
  return gateway.rpc<unknown[]>('chat.history', params);
}

// ---------------------------------------------------------------------------
// Exec Approvals
// ---------------------------------------------------------------------------

export function execApprovalResolve(params: ExecApprovalResolveParams): Promise<void> {
  return gateway.rpc<void>('exec.approval.resolve', params);
}

// ---------------------------------------------------------------------------
// Teams
// ---------------------------------------------------------------------------

export function teamPresetsList(): Promise<TeamPresetsListResult> {
  return gateway.rpc<TeamPresetsListResult>('team.presets.list');
}

export function teamProvision(params: TeamProvisionParams): Promise<TeamProvisionResult> {
  return gateway.rpc<TeamProvisionResult>('team.provision', params);
}

// ---------------------------------------------------------------------------
// Cloud Provisioning (full cloud nodes on DOKS)
// ---------------------------------------------------------------------------

export interface CloudProvisionParams {
  node_id?: string;
  display_name: string;
  model: string;
  image?: string;
  workspace?: string;
  env?: Record<string, string>;
  labels?: Record<string, string>;
  cpu?: string;
  memory?: string;
  os?: string;
  provider?: string;
  instance_type?: string;
}

export interface CloudProvisionResult {
  node_id: string;
  pod_name: string;
  namespace: string;
  node_type: 'local' | 'cloud';
  status: string;
  endpoint?: string;
  created_at: string;
}

export interface CloudNode {
  node_id: string;
  pod_name: string;
  namespace: string;
  node_type: 'local' | 'cloud';
  status: string;
  image: string;
  endpoint: string;
  owner: string;
  org: string;
  os: string;
  remote_protocol: string;
  remote_url: string;
  labels: Record<string, string>;
  created_at: string;
  last_seen: string;
}

/** Error thrown when billing check fails (HTTP 402). */
export class InsufficientFundsError extends Error {
  balanceCents: number;
  requiredCents: number;
  hoursAfford: number;

  constructor(data: { error: string; balance_cents: number; required_cents: number; hours_afford: number }) {
    super(data.error);
    this.name = 'InsufficientFundsError';
    this.balanceCents = data.balance_cents;
    this.requiredCents = data.required_cents;
    this.hoursAfford = data.hours_afford;
  }
}

/** Build auth headers for cloud API calls. */
function cloudAuthHeaders(): Record<string, string> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = getGlobalIamToken();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  } else {
    const apiKey = getGlobalApiKey();
    if (apiKey) headers['X-API-Key'] = apiKey;
  }
  return headers;
}

/** Provision a full cloud hanzo node on DOKS. */
export async function cloudProvision(params: CloudProvisionParams): Promise<CloudProvisionResult> {
  const resp = await fetch(`${API_BASE_URL}/cloud/nodes/provision`, {
    method: 'POST',
    headers: cloudAuthHeaders(),
    body: JSON.stringify(params),
  });
  if (resp.status === 402) {
    const data = await resp.json();
    throw new InsufficientFundsError(data);
  }
  if (resp.status === 403) {
    throw new Error('Authentication required. Please sign in to launch bots.');
  }
  if (!resp.ok) {
    const data = await resp.json().catch(() => ({ message: `cloud provision failed: ${resp.status}` }));
    throw new Error(data.message || data.error || `cloud provision failed: ${resp.status}`);
  }
  return resp.json();
}

/** Billing balance for cloud provisioning presets. */
export interface CloudBillingBalance {
  balance_cents: number;
  currency: string;
  presets: Array<{ name: string; cents_per_hour: number; hours_afford: number }>;
}

/** Get the user's billing balance and affordability per preset. */
export async function cloudGetBillingBalance(): Promise<CloudBillingBalance> {
  const resp = await fetch(`${API_BASE_URL}/cloud/billing/balance`, {
    headers: cloudAuthHeaders(),
  });
  if (!resp.ok) return { balance_cents: 0, currency: 'usd', presets: [] };
  return resp.json();
}

/** Deprovision a cloud hanzo node. */
export async function cloudDeprovision(nodeId: string): Promise<void> {
  const resp = await fetch(`${API_BASE_URL}/cloud/nodes/${nodeId}`, {
    method: 'DELETE',
    headers: cloudAuthHeaders(),
  });
  if (!resp.ok) throw new Error(`cloud deprovision failed: ${resp.status}`);
}

/** List all cloud nodes (optionally filtered by org). */
export async function cloudListNodes(org?: string): Promise<{ nodes: CloudNode[]; count: number }> {
  const url = org ? `${API_BASE_URL}/cloud/nodes?org=${org}` : `${API_BASE_URL}/cloud/nodes`;
  const resp = await fetch(url);
  if (!resp.ok) throw new Error(`cloud list failed: ${resp.status}`);
  return resp.json();
}

/** Get a specific cloud node. */
export async function cloudGetNode(nodeId: string): Promise<CloudNode> {
  const resp = await fetch(`${API_BASE_URL}/cloud/nodes/${nodeId}`);
  if (!resp.ok) throw new Error(`cloud get node failed: ${resp.status}`);
  return resp.json();
}

/** Get logs for a cloud node. */
export async function cloudGetLogs(nodeId: string, tail = 100): Promise<{ node_id: string; logs: string }> {
  const resp = await fetch(`${API_BASE_URL}/cloud/nodes/${nodeId}/logs?tail=${tail}`);
  if (!resp.ok) throw new Error(`cloud logs failed: ${resp.status}`);
  return resp.json();
}

/** Provision an entire team of cloud agents. */
export async function cloudTeamProvision(teamName: string, agents: CloudProvisionParams[], workspace?: string): Promise<{
  team_name: string;
  agents: CloudProvisionResult[];
  errors: string[];
  count: number;
}> {
  const resp = await fetch(`${API_BASE_URL}/cloud/teams/provision`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ team_name: teamName, agents, workspace }),
  });
  if (!resp.ok) throw new Error(`team provision failed: ${resp.status}`);
  return resp.json();
}

// ---------------------------------------------------------------------------
// Cloud Pricing & Presets
// ---------------------------------------------------------------------------

export interface PricingTier {
  slug: string;
  vcpus: number;
  memory_mb: number;
  disk_gb: number;
  cents_per_hour: number;
}

export interface CloudPreset {
  id: string;
  name: string;
  description: string;
  slug: string;
  vcpus: number;
  // Go backend sends snake_case, pricing service sends camelCase.
  memory_gb?: number;
  memoryGB?: number;
  cents_per_hour?: number;
  centsPerHour?: number;
  provider?: string;
}

export async function cloudGetPricing(): Promise<{ provider: string; region: string; tiers: PricingTier[] }> {
  const resp = await fetch(`${API_BASE_URL}/cloud/pricing`);
  if (!resp.ok) throw new Error(`cloud pricing failed: ${resp.status}`);
  return resp.json();
}

export async function cloudGetPresets(): Promise<{ presets: CloudPreset[] }> {
  const resp = await fetch(`${API_BASE_URL}/cloud/presets`);
  if (!resp.ok) throw new Error(`cloud presets failed: ${resp.status}`);
  return resp.json();
}

// ---------------------------------------------------------------------------
// Node Invoke (forward commands to gateway-connected nodes)
// ---------------------------------------------------------------------------

export function nodeInvoke(nodeId: string, command: string, params?: unknown): Promise<unknown> {
  return gateway.rpc<unknown>('node.invoke', {
    nodeId,
    command,
    params,
    idempotencyKey: `${nodeId}-${command}-${Date.now()}`,
  });
}

// ---------------------------------------------------------------------------
// Node List (gateway-connected nodes)
// ---------------------------------------------------------------------------

export interface GatewayNode {
  nodeId: string;
  displayName?: string;
  platform?: string;
  version?: string;
  connected: boolean;
  connectedAtMs?: number;
  caps?: string[];
  commands?: string[];
}

/** List nodes currently connected to the gateway. */
export function nodeList(connectedOnly = true): Promise<{ nodes?: GatewayNode[] }> {
  return gateway.rpc<{ nodes?: GatewayNode[] }>('node.list', { connectedOnly });
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

export function health(): Promise<{ ok: boolean }> {
  return gateway.rpc<{ ok: boolean }>('health');
}

// ---------------------------------------------------------------------------
// Status
// ---------------------------------------------------------------------------

export function status(): Promise<unknown> {
  return gateway.rpc<unknown>('status');
}
