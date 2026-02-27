/**
 * Network Store (Zustand)
 *
 * Manages AI capacity sharing state, wallet connection,
 * earnings tracking, and network statistics.
 * Reads from localStorage on init, syncs to backend on change.
 */

import { create } from 'zustand';
import type {
  SharingConfig,
  SharingMode,
  SharingSchedule,
  SharingStatus,
  EarningsSummary,
  EarningsRecord,
  NetworkStats,
  WalletConnection,
  WalletProvider,
} from '@/types/network';
import * as networkApi from '@/services/networkApi';

const STORAGE_KEY = 'hanzo_network_sharing';

// ---------------------------------------------------------------------------
// State & Actions
// ---------------------------------------------------------------------------

interface NetworkState {
  sharingConfig: SharingConfig;
  sharingStatus: SharingStatus;
  sharingStatusSince: string | null;
  earnings: EarningsSummary;
  earningsHistory: EarningsRecord[];
  wallet: WalletConnection | null;
  aiCoinBalance: number;
  aiCoinPending: number;
  networkStats: NetworkStats | null;
  initialized: boolean;
  syncing: boolean;
  lastSyncError: string | null;
}

interface NetworkActions {
  setSharingEnabled: (enabled: boolean) => void;
  setSharingMode: (mode: SharingMode) => void;
  setIdleThreshold: (minutes: number) => void;
  setMaxCapacity: (percent: number) => void;
  setSharingSchedule: (schedule: SharingSchedule | null) => void;
  connectWallet: (provider: WalletProvider, address: string, chainId: number) => Promise<void>;
  disconnectWallet: () => Promise<void>;
  syncFromBackend: () => Promise<void>;
  refreshEarnings: () => Promise<void>;
  refreshNetworkStats: () => Promise<void>;
  refreshAiCoinBalance: () => Promise<void>;
  reset: () => void;
}

type NetworkStoreState = NetworkState & NetworkActions;

// ---------------------------------------------------------------------------
// Defaults — sharing opt-in by default
// ---------------------------------------------------------------------------

const EARNINGS_ZERO: EarningsSummary = {
  totalEarned: 0,
  todayEarned: 0,
  weekEarned: 0,
  monthEarned: 0,
  currentRatePerHour: 0,
  totalHoursShared: 0,
};

const DEFAULTS: NetworkState = {
  sharingConfig: {
    enabled: true,
    mode: 'auto',
    idleThresholdMinutes: 60,
    schedule: null,
    maxCapacityPercent: 80,
  },
  sharingStatus: 'idle',
  sharingStatusSince: null,
  earnings: EARNINGS_ZERO,
  earningsHistory: [],
  wallet: null,
  aiCoinBalance: 0,
  aiCoinPending: 0,
  networkStats: null,
  initialized: false,
  syncing: false,
  lastSyncError: null,
};

// ---------------------------------------------------------------------------
// localStorage persistence
// ---------------------------------------------------------------------------

function loadFromStorage(): Partial<NetworkState> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return {};
    return JSON.parse(raw) as Partial<NetworkState>;
  } catch {
    return {};
  }
}

function persistToStorage(state: NetworkState) {
  try {
    const { sharingConfig, wallet, aiCoinBalance, aiCoinPending } = state;
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ sharingConfig, wallet, aiCoinBalance, aiCoinPending }));
  } catch { /* best effort */ }
}

function configSnapshot(state: NetworkStoreState): SharingConfig {
  return { ...state.sharingConfig };
}

// ---------------------------------------------------------------------------
// Debounced backend sync
// ---------------------------------------------------------------------------

let syncTimer: ReturnType<typeof setTimeout> | null = null;

function debouncedConfigSync(config: SharingConfig) {
  if (syncTimer) clearTimeout(syncTimer);
  syncTimer = setTimeout(() => {
    networkApi.putSharingConfig(config).catch(() => { /* best effort */ });
  }, 800);
}

// ---------------------------------------------------------------------------
// Store
// ---------------------------------------------------------------------------

const stored = loadFromStorage();
const initial: NetworkState = {
  ...DEFAULTS,
  sharingConfig: stored.sharingConfig ? { ...DEFAULTS.sharingConfig, ...stored.sharingConfig } : DEFAULTS.sharingConfig,
  wallet: stored.wallet ?? DEFAULTS.wallet,
  aiCoinBalance: stored.aiCoinBalance ?? DEFAULTS.aiCoinBalance,
  aiCoinPending: stored.aiCoinPending ?? DEFAULTS.aiCoinPending,
};

export const useNetworkStore = create<NetworkStoreState>((set, get) => ({
  ...initial,

  // -- Sharing controls --

  setSharingEnabled: (enabled) => {
    const config = { ...configSnapshot(get()), enabled };
    set({ sharingConfig: config });
    persistToStorage(get());
    debouncedConfigSync(config);
  },

  setSharingMode: (mode) => {
    const config = { ...configSnapshot(get()), mode };
    set({ sharingConfig: config });
    persistToStorage(get());
    debouncedConfigSync(config);
  },

  setIdleThreshold: (minutes) => {
    const config = { ...configSnapshot(get()), idleThresholdMinutes: Math.max(15, Math.min(120, minutes)) };
    set({ sharingConfig: config });
    persistToStorage(get());
    debouncedConfigSync(config);
  },

  setMaxCapacity: (percent) => {
    const config = { ...configSnapshot(get()), maxCapacityPercent: Math.max(10, Math.min(100, percent)) };
    set({ sharingConfig: config });
    persistToStorage(get());
    debouncedConfigSync(config);
  },

  setSharingSchedule: (schedule) => {
    const config = { ...configSnapshot(get()), schedule };
    set({ sharingConfig: config });
    persistToStorage(get());
    debouncedConfigSync(config);
  },

  // -- Wallet --

  connectWallet: async (provider, address, chainId) => {
    const wallet = await networkApi.connectWallet(provider, address, chainId);
    set({ wallet });
    persistToStorage(get());
  },

  disconnectWallet: async () => {
    await networkApi.disconnectWallet();
    set({ wallet: null });
    persistToStorage(get());
  },

  // -- Sync --

  syncFromBackend: async () => {
    set({ syncing: true, lastSyncError: null });
    try {
      const [config, status, earnings, history, stats, balance, wallet] = await Promise.allSettled([
        networkApi.getSharingConfig(),
        networkApi.getSharingStatus(),
        networkApi.getEarningsSummary(),
        networkApi.getEarningsHistory(),
        networkApi.getNetworkStats(),
        networkApi.getAiCoinBalance(),
        networkApi.getWallet(),
      ]);

      const patch: Partial<NetworkState> = { initialized: true, syncing: false };
      if (config.status === 'fulfilled') patch.sharingConfig = config.value;
      if (status.status === 'fulfilled') {
        patch.sharingStatus = status.value.status;
        patch.sharingStatusSince = status.value.since;
      }
      if (earnings.status === 'fulfilled') patch.earnings = earnings.value;
      if (history.status === 'fulfilled') patch.earningsHistory = history.value;
      if (stats.status === 'fulfilled') patch.networkStats = stats.value;
      if (balance.status === 'fulfilled') {
        patch.aiCoinBalance = balance.value.balance;
        patch.aiCoinPending = balance.value.pending;
      }
      if (wallet.status === 'fulfilled') patch.wallet = wallet.value;

      set(patch);
      persistToStorage(get());
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      set({ syncing: false, lastSyncError: msg });
    }
  },

  refreshEarnings: async () => {
    const [summary, history] = await Promise.allSettled([
      networkApi.getEarningsSummary(),
      networkApi.getEarningsHistory(),
    ]);
    const patch: Partial<NetworkState> = {};
    if (summary.status === 'fulfilled') patch.earnings = summary.value;
    if (history.status === 'fulfilled') patch.earningsHistory = history.value;
    set(patch);
  },

  refreshNetworkStats: async () => {
    const stats = await networkApi.getNetworkStats();
    set({ networkStats: stats });
  },

  refreshAiCoinBalance: async () => {
    const { balance, pending } = await networkApi.getAiCoinBalance();
    set({ aiCoinBalance: balance, aiCoinPending: pending });
    persistToStorage(get());
  },

  // -- Reset (on logout) --

  reset: () => {
    localStorage.removeItem(STORAGE_KEY);
    set(DEFAULTS);
  },
}));
