/**
 * useGateway Hook
 *
 * React lifecycle hook for the gateway WebSocket connection.
 * Connects on mount, syncs agents, subscribes to events.
 * Passes auth token and tenant context for multi-tenant isolation.
 */

import { useEffect, useState, useCallback, useRef, useMemo } from 'react';
import { gateway } from '@/services/gatewayClient';
import { agentsList } from '@/services/gatewayApi';
import { useBotStore } from '@/stores/botStore';
import { useCanvasStore } from '@/stores/canvasStore';
import { useActionPillStore } from '@/stores/actionPillStore';
import { useAuth } from '@/contexts/AuthContext';
import { useTenantStore } from '@/stores/tenantStore';
import { useSettingsStore } from '@/stores/settingsStore';
import { getGlobalIamToken } from '@/services/api';
import type {
  ConnectParams,
  GatewayConnectionState,
  ChatEvent,
  AgentEvent,
  AgentSummary,
  AgentsListResponse,
  ExecApprovalRequestEvent,
} from '@/types/gateway';

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

const DEFAULT_GATEWAY_URL = 'wss://gw.hanzo.bot';
const ENV_GATEWAY_URL = import.meta.env.VITE_GATEWAY_URL as string | undefined;
const ENV_GATEWAY_TOKEN = import.meta.env.VITE_GATEWAY_TOKEN as string | undefined;
const CLIENT_VERSION = '2.0.0';

/**
 * Validate gateway URL against allowed protocols.
 * When `isUserOverride` is true, allow ws:// even in production
 * (the user explicitly chose to connect to a custom endpoint).
 */
function validateGatewayUrl(url: string, isUserOverride = false): string {
  try {
    const parsed = new URL(url);
    if (!['ws:', 'wss:'].includes(parsed.protocol)) {
      return DEFAULT_GATEWAY_URL;
    }
    if (import.meta.env.PROD && parsed.protocol !== 'wss:' && !isUserOverride) {
      return DEFAULT_GATEWAY_URL;
    }
    return url;
  } catch {
    return DEFAULT_GATEWAY_URL;
  }
}

/** Resolve the effective gateway URL. Priority: settingsStore > env > default */
function resolveGatewayUrl(storeUrl: string | null): { url: string; isUserOverride: boolean } {
  if (storeUrl) {
    return { url: validateGatewayUrl(storeUrl, true), isUserOverride: true };
  }
  if (ENV_GATEWAY_URL) {
    return { url: validateGatewayUrl(ENV_GATEWAY_URL), isUserOverride: false };
  }
  return { url: DEFAULT_GATEWAY_URL, isUserOverride: false };
}

/** Resolve the effective auth token. Priority: IAM SDK > settings > env > localStorage */
function resolveToken(apiKey: string | null, settingsToken: string | null): string | null {
  return apiKey || settingsToken || ENV_GATEWAY_TOKEN || getGlobalIamToken() || null;
}

// ---------------------------------------------------------------------------
// Tenant context from auth or URL
// ---------------------------------------------------------------------------

export interface TenantContext {
  orgId?: string;
  projectId?: string;
  tenantId?: string;
  actorId?: string;
  env?: string;
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useGateway(explicitTenant?: TenantContext) {
  const [connectionState, setConnectionState] = useState<GatewayConnectionState>('disconnected');
  const syncedRef = useRef(false);
  const connectedUrlRef = useRef<string | null>(null);
  const connectedTokenRef = useRef<string | null>(null);
  const { apiKey } = useAuth();

  // Read custom gateway URL/token from settings store
  const settingsGatewayUrl = useSettingsStore((s) => s.gatewayUrl);
  const settingsGatewayToken = useSettingsStore((s) => s.gatewayToken);

  // Resolve effective gateway URL (settingsStore > env > default)
  const { url: effectiveUrl } = useMemo(
    () => resolveGatewayUrl(settingsGatewayUrl),
    [settingsGatewayUrl],
  );

  // Resolve effective token
  const effectiveToken = resolveToken(apiKey, settingsGatewayToken);

  // Read tenant from Zustand store (set by OrgProjectSwitcher in IAM mode)
  const storeOrgId = useTenantStore((s) => s.orgId);
  const storeProjectId = useTenantStore((s) => s.projectId);

  // Use explicit tenant if provided, otherwise derive from store
  const tenant = useMemo(() => {
    if (explicitTenant) return explicitTenant;
    if (storeOrgId) return { orgId: storeOrgId, projectId: storeProjectId ?? undefined };
    return undefined;
  }, [explicitTenant, storeOrgId, storeProjectId]);

  const setAgents = useBotStore((s) => s.setAgents);
  const handleChatEvent = useBotStore((s) => s.handleChatEvent);
  const handleBotEvent = useBotStore((s) => s.handleBotEvent);
  const upsertBot = useCanvasStore((s) => s.upsertBot);
  const addApproval = useActionPillStore((s) => s.add);

  /** Build ConnectParams with current auth + tenant context */
  const buildConnectParams = useCallback((): ConnectParams => ({
    minProtocol: 3,
    maxProtocol: 3,
    client: {
      id: 'bot-control-ui',
      version: CLIENT_VERSION,
      platform: 'web',
      mode: 'ui',
      displayName: 'Hanzo Playground',
    },
    role: 'operator',
    scopes: ['operator.admin'],
    ...(effectiveToken ? { auth: { token: effectiveToken } } : {}),
    ...(tenant ? {
      tenant: {
        orgId: tenant.orgId,
        projectId: tenant.projectId,
        tenantId: tenant.tenantId ?? tenant.orgId,
        actorId: tenant.actorId,
        env: tenant.env ?? (import.meta.env.PROD ? 'production' : 'development'),
      },
    } : {}),
  }), [effectiveToken, tenant]);

  /** Convert agents.list response rows into AgentSummary with derived sessionKey */
  const toAgentSummaries = useCallback((resp: AgentsListResponse): AgentSummary[] => {
    const mainKey = resp.mainKey || 'main';
    return resp.agents.map((row) => ({
      id: row.id,
      name: row.identity?.name ?? row.name ?? row.id,
      emoji: row.identity?.emoji,
      avatar: row.identity?.avatarUrl ?? row.identity?.avatar,
      status: 'idle' as const,
      sessionKey: `agent:${row.id}:${mainKey}`,
    }));
  }, []);

  // Sync agents from gateway to both stores
  const syncAgents = useCallback(async () => {
    try {
      const resp = await agentsList();
      const agents = toAgentSummaries(resp);
      setAgents(agents);

      for (const agent of agents) {
        upsertBot(agent.id, {
          name: agent.name,
          emoji: agent.emoji,
          avatar: agent.avatar,
          status: agent.status,
          sessionKey: agent.sessionKey,
          source: 'cloud',
        });
      }
    } catch {
      // Gateway not ready or auth failed — will retry on reconnect
    }
  }, [setAgents, upsertBot, toAgentSummaries]);

  // Handle agents appearing from snapshot
  const syncFromSnapshot = useCallback((agents: AgentSummary[]) => {
    setAgents(agents);
    for (const agent of agents) {
      upsertBot(agent.id, {
        name: agent.name,
        emoji: agent.emoji,
        avatar: agent.avatar,
        status: agent.status,
        sessionKey: agent.sessionKey,
        model: agent.model,
        workspace: agent.workspace,
        source: 'cloud',
      });
    }
  }, [setAgents, upsertBot]);

  // Subscribe to gateway events (stable — does not trigger reconnect)
  useEffect(() => {
    const unsubState = gateway.onStateChange((state) => {
      setConnectionState(state);
      if (state === 'connected' && !syncedRef.current) {
        syncedRef.current = true;
        const snapshot = gateway.snapshot;
        if (snapshot?.agents && snapshot.agents.length > 0) {
          syncFromSnapshot(snapshot.agents);
        } else {
          syncAgents();
        }
      }
      if (state === 'disconnected' || state === 'error') {
        syncedRef.current = false;
      }
    });

    const unsubChat = gateway.on('chat', (payload) => {
      handleChatEvent(payload as ChatEvent);
    });
    const unsubAgent = gateway.on('agent', (payload) => {
      handleBotEvent(payload as AgentEvent);
    });
    const unsubApproval = gateway.on('exec.approval.requested', (payload) => {
      addApproval(payload as ExecApprovalRequestEvent);
    });

    return () => {
      unsubState();
      unsubChat();
      unsubAgent();
      unsubApproval();
    };
  }, [syncAgents, syncFromSnapshot, handleChatEvent, handleBotEvent, addApproval]);

  // Connect/reconnect only when URL or token actually changes
  useEffect(() => {
    const urlChanged = effectiveUrl !== connectedUrlRef.current;
    const tokenChanged = effectiveToken !== connectedTokenRef.current;

    // Skip if already connected with the same URL and token
    if (!urlChanged && !tokenChanged && gateway.isConnected) return;

    // Skip reconnect if we have no token and already tried without one
    if (!effectiveToken && connectedTokenRef.current === null && connectedUrlRef.current === effectiveUrl) return;

    connectedUrlRef.current = effectiveUrl;
    connectedTokenRef.current = effectiveToken;
    syncedRef.current = false;
    gateway.disconnect();
    gateway.connect(effectiveUrl, buildConnectParams());
  }, [effectiveUrl, effectiveToken, buildConnectParams]);

  const reconnect = useCallback(() => {
    syncedRef.current = false;
    connectedUrlRef.current = null;
    connectedTokenRef.current = null;
    gateway.disconnect();
    gateway.connect(effectiveUrl, buildConnectParams());
  }, [buildConnectParams, effectiveUrl]);

  return {
    connectionState,
    isConnected: connectionState === 'connected',
    gatewayUrl: effectiveUrl,
    reconnect,
    syncAgents,
  };
}
