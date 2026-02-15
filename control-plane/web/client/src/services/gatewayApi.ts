/**
 * Gateway API - Typed RPC Wrappers
 *
 * Thin typed wrappers over gateway.rpc() for each method
 * we use in the UI. Add new methods as needed.
 */

import { gateway } from './gatewayClient';
import type {
  AgentsCreateParams,
  AgentsCreateResult,
  AgentsListResult,
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

export function agentsList(): Promise<AgentsListResult> {
  return gateway.rpc<AgentsListResult>('agents.list');
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
