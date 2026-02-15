/**
 * ActionPill Store (Zustand)
 *
 * Centralized approval queue for tool-use requests from bots.
 * When a bot requests to use a tool, the approval appears here.
 * User can approve/deny via ActionPill UI or keyboard shortcuts.
 */

import { create } from 'zustand';
import type { ExecApprovalRequestEvent } from '@/types/gateway';

export interface ApprovalAction {
  id: string;
  agentId: string;
  sessionKey: string;
  toolName: string;
  toolInput: Record<string, unknown>;
  requestedAt: string;
  status: 'pending' | 'approved' | 'denied';
}

interface ActionPillStore {
  actions: ApprovalAction[];
  activeIndex: number;

  /** Add a new approval request */
  add: (event: ExecApprovalRequestEvent) => void;
  /** Resolve an action (approve or deny) */
  resolve: (id: string, decision: 'approved' | 'denied') => void;
  /** Remove resolved actions */
  prune: () => void;
  /** Navigate to next pending action */
  next: () => void;
  /** Navigate to previous pending action */
  prev: () => void;
  /** Get current active action */
  active: () => ApprovalAction | null;
  /** Get pending count */
  pendingCount: () => number;
  /** Reset all state (call on logout/tenant switch) */
  reset: () => void;
}

export const useActionPillStore = create<ActionPillStore>((set, get) => ({
  actions: [],
  activeIndex: 0,

  add: (event) => {
    const action: ApprovalAction = {
      id: event.approvalId,
      agentId: event.agentId,
      sessionKey: event.sessionKey,
      toolName: event.toolName,
      toolInput: event.toolInput,
      requestedAt: event.requestedAt,
      status: 'pending',
    };
    set({ actions: [...get().actions, action] });
  },

  resolve: (id, decision) => {
    const { actions } = get();
    set({
      actions: actions.map((a) =>
        a.id === id ? { ...a, status: decision } : a
      ),
    });
  },

  prune: () => {
    const { actions } = get();
    set({ actions: actions.filter((a) => a.status === 'pending') });
  },

  next: () => {
    const pending = get().actions.filter((a) => a.status === 'pending');
    if (pending.length === 0) return;
    set({ activeIndex: (get().activeIndex + 1) % pending.length });
  },

  prev: () => {
    const pending = get().actions.filter((a) => a.status === 'pending');
    if (pending.length === 0) return;
    const { activeIndex } = get();
    set({ activeIndex: activeIndex === 0 ? pending.length - 1 : activeIndex - 1 });
  },

  active: () => {
    const pending = get().actions.filter((a) => a.status === 'pending');
    if (pending.length === 0) return null;
    return pending[get().activeIndex % pending.length] ?? null;
  },

  pendingCount: () => get().actions.filter((a) => a.status === 'pending').length,

  /** Reset all state (call on logout/tenant switch) */
  reset: () => set({ actions: [], activeIndex: 0 }),
}));
