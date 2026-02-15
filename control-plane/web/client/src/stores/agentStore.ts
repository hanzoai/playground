/**
 * Agent Store (Zustand)
 *
 * Live agent state from gateway. Syncs agents.list results
 * and real-time events into a single reactive store.
 */

import { create } from 'zustand';
import type { AgentSummary, AgentStatus, ChatEvent, AgentEvent } from '@/types/gateway';

// ---------------------------------------------------------------------------
// State Shape
// ---------------------------------------------------------------------------

interface AgentStoreState {
  /** All known agents keyed by ID */
  agents: Map<string, AgentSummary>;
  /** Last time agents were fetched */
  lastSync: number | null;
  /** Whether initial sync has completed */
  initialized: boolean;
}

interface AgentStoreActions {
  /** Replace entire agent list (from agents.list RPC) */
  setAgents: (agents: AgentSummary[]) => void;
  /** Update a single agent */
  updateAgent: (agentId: string, updates: Partial<AgentSummary>) => void;
  /** Set agent status */
  setStatus: (agentId: string, status: AgentStatus) => void;
  /** Remove an agent */
  removeAgent: (agentId: string) => void;
  /** Handle incoming chat event (updates agent activity) */
  handleChatEvent: (event: ChatEvent) => void;
  /** Handle incoming agent event */
  handleAgentEvent: (event: AgentEvent) => void;
  /** Get agent by ID */
  getAgent: (agentId: string) => AgentSummary | undefined;
  /** Get all agents as array */
  getAgentList: () => AgentSummary[];
  /** Reset all state (call on logout/tenant switch) */
  reset: () => void;
}

type AgentStore = AgentStoreState & AgentStoreActions;

// ---------------------------------------------------------------------------
// Store
// ---------------------------------------------------------------------------

export const useAgentStore = create<AgentStore>((set, get) => ({
  // State
  agents: new Map(),
  lastSync: null,
  initialized: false,

  // Actions
  setAgents: (agents) => {
    const map = new Map<string, AgentSummary>();
    for (const agent of agents) {
      map.set(agent.id, agent);
    }
    set({ agents: map, lastSync: Date.now(), initialized: true });
  },

  updateAgent: (agentId, updates) => {
    const { agents } = get();
    const existing = agents.get(agentId);
    if (!existing) return;
    const next = new Map(agents);
    next.set(agentId, { ...existing, ...updates });
    set({ agents: next });
  },

  setStatus: (agentId, status) => {
    const { agents } = get();
    const existing = agents.get(agentId);
    if (!existing) return;
    const next = new Map(agents);
    next.set(agentId, { ...existing, status });
    set({ agents: next });
  },

  removeAgent: (agentId) => {
    const { agents } = get();
    if (!agents.has(agentId)) return;
    const next = new Map(agents);
    next.delete(agentId);
    set({ agents: next });
  },

  handleChatEvent: (event) => {
    // Find agent by session key
    const { agents } = get();
    for (const [id, agent] of agents) {
      if (agent.sessionKey === event.sessionKey) {
        const next = new Map(agents);
        const status: AgentStatus =
          event.state === 'delta' ? 'busy' :
          event.state === 'final' ? 'idle' :
          event.state === 'error' ? 'error' : 'idle';
        next.set(id, { ...agent, status, lastActivity: new Date().toISOString() });
        set({ agents: next });
        break;
      }
    }
  },

  handleAgentEvent: (event) => {
    // Agent events carry stream info like tool.call, message, etc.
    // We update lastActivity for the agent
    const agentId = event.data?.agentId as string | undefined;
    if (!agentId) return;
    const { agents } = get();
    const agent = agents.get(agentId);
    if (!agent) return;
    const next = new Map(agents);
    next.set(agentId, {
      ...agent,
      status: 'busy',
      lastActivity: new Date(event.ts).toISOString(),
    });
    set({ agents: next });
  },

  getAgent: (agentId) => get().agents.get(agentId),
  getAgentList: () => Array.from(get().agents.values()),

  /** Reset all state (call on logout/tenant switch) */
  reset: () => set({ agents: new Map(), lastSync: null, initialized: false }),
}));
