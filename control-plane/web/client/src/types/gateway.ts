/**
 * Gateway WebSocket Protocol Types
 *
 * Matches the hanzo.bot gateway ZAP protocol:
 * - JSON-RPC style request/response frames
 * - Event streaming with sequence numbers
 * - State versioning for efficient syncs
 */

// ---------------------------------------------------------------------------
// Frame Types
// ---------------------------------------------------------------------------

export interface RequestFrame {
  type: 'req';
  id: string;
  method: string;
  params?: unknown;
}

export interface ResponseFrame {
  type: 'res';
  id: string;
  ok: boolean;
  payload?: unknown;
  error?: ErrorShape;
}

export interface EventFrame {
  type: 'event';
  event: string;
  payload?: unknown;
  seq?: number;
  stateVersion?: StateVersion;
}

export interface ErrorShape {
  code: string;
  message: string;
  data?: unknown;
}

export type Frame = RequestFrame | ResponseFrame | EventFrame;

export interface StateVersion {
  v: number;
  ts: number;
}

// ---------------------------------------------------------------------------
// Connection
// ---------------------------------------------------------------------------

export interface ConnectChallenge {
  nonce: string;
}

export interface ConnectParams {
  minProtocol: number;
  maxProtocol: number;
  client: {
    id: string;
    version: string;
    platform: string;
    mode: string;
    displayName?: string;
  };
  caps?: string[];
  role?: 'operator' | 'node';
  scopes?: string[];
  auth?: {
    token?: string;
    password?: string;
  };
  tenant?: {
    orgId?: string;
    projectId?: string;
    tenantId?: string;
    actorId?: string;
    env?: string;
  };
}

export interface HelloOk {
  type: 'hello-ok';
  protocol: number;
  server: {
    version: string;
    commit?: string;
    host?: string;
    connId: string;
  };
  features: {
    methods: string[];
    events: string[];
  };
  snapshot: StateSnapshot;
  canvasHostUrl?: string;
  auth?: {
    deviceToken: string;
    role: string;
    scopes: string[];
  };
  policy: {
    maxPayload: number;
    maxBufferedBytes: number;
    tickIntervalMs: number;
  };
}

export interface StateSnapshot {
  agents?: AgentSummary[];
  sessions?: SessionSummary[];
  [key: string]: unknown;
}

// ---------------------------------------------------------------------------
// Agent Types
// ---------------------------------------------------------------------------

export interface AgentSummary {
  id: string;
  name: string;
  emoji?: string;
  avatar?: string;
  workspace?: string;
  status: BotStatus;
  sessionKey?: string;
  model?: string;
  lastActivity?: string;
  createdAt?: string;
}

export type BotStatus =
  | 'idle'
  | 'busy'
  | 'waiting'
  | 'error'
  | 'offline'
  | 'provisioning';

export interface SessionSummary {
  key: string;
  agentId: string;
  status: string;
  messageCount: number;
  createdAt: string;
  lastActivity: string;
}

// ---------------------------------------------------------------------------
// Chat Types
// ---------------------------------------------------------------------------

export interface ChatEvent {
  runId: string;
  sessionKey: string;
  seq: number;
  state: 'delta' | 'final' | 'aborted' | 'error';
  message?: unknown;
  errorMessage?: string;
  usage?: unknown;
  stopReason?: string;
}

export interface AgentEvent {
  runId: string;
  seq: number;
  stream: string;
  ts: number;
  data: Record<string, unknown>;
}

// ---------------------------------------------------------------------------
// RPC Params & Results
// ---------------------------------------------------------------------------

export interface AgentsListParams {
  // empty
}

/** Raw gateway response for agents.list — the payload envelope */
export interface AgentsListResponse {
  defaultId: string;
  mainKey: string;
  scope: string;
  agents: AgentsListRow[];
}

/** Individual agent row from agents.list (no sessionKey — must be derived) */
export interface AgentsListRow {
  id: string;
  name?: string;
  identity?: {
    name?: string;
    theme?: string;
    emoji?: string;
    avatar?: string;
    avatarUrl?: string;
  };
}

/** @deprecated Use AgentsListResponse instead */
export type AgentsListResult = AgentsListResponse;

export interface AgentsCreateParams {
  name: string;
  workspace: string;
  emoji?: string;
  avatar?: string;
}

export interface AgentsCreateResult {
  id: string;
  name: string;
}

export interface ChatSendParams {
  sessionKey: string;
  message: string;
  thinking?: string;
  deliver?: boolean;
  attachments?: unknown[];
  timeoutMs?: number;
  idempotencyKey: string;
}

export interface ChatAbortParams {
  sessionKey: string;
  runId?: string;
}

export interface ChatHistoryParams {
  sessionKey: string;
  limit?: number;
}

export interface ExecApprovalResolveParams {
  approvalId: string;
  decision: 'allow' | 'deny';
  reason?: string;
}

export interface ExecApprovalRequestEvent {
  approvalId: string;
  agentId: string;
  sessionKey: string;
  toolName: string;
  toolInput: Record<string, unknown>;
  requestedAt: string;
}

export interface TeamPresetsListResult {
  presets: TeamPreset[];
}

export interface TeamPreset {
  id: string;
  name: string;
  description: string;
  emoji: string;
  bots: TeamPresetBot[];
}

export interface TeamPresetBot {
  role: string;
  name: string;
  model?: string;
  systemPrompt?: string;
}

export interface TeamProvisionParams {
  presetId: string;
  workspace?: string;
}

export interface TeamProvisionResult {
  teamId: string;
  agents: AgentSummary[];
}

// ---------------------------------------------------------------------------
// Connection State
// ---------------------------------------------------------------------------

export type GatewayConnectionState =
  | 'disconnected'
  | 'connecting'
  | 'authenticating'
  | 'connected'
  | 'reconnecting'
  | 'error';
