/**
 * Gateway API - Typed RPC Wrappers
 *
 * Thin typed wrappers over gateway.rpc() for each method
 * we use in the UI. Add new methods as needed.
 */

import { gateway } from './gatewayClient';
import { API_BASE_URL } from './api';
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

/** Provision a full cloud hanzo node on DOKS. */
export async function cloudProvision(params: CloudProvisionParams): Promise<CloudProvisionResult> {
  const resp = await fetch(`${API_BASE_URL}/cloud/nodes/provision`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  });
  if (!resp.ok) throw new Error(`cloud provision failed: ${resp.status}`);
  return resp.json();
}

/** Deprovision a cloud hanzo node. */
export async function cloudDeprovision(nodeId: string): Promise<void> {
  const resp = await fetch(`${API_BASE_URL}/cloud/nodes/${nodeId}`, { method: 'DELETE' });
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
