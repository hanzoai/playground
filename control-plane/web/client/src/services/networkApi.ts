/**
 * Network Marketplace API
 *
 * Client for the AI capacity sharing marketplace endpoints.
 * Users share unused AI/LLM capacity and earn AI coin on the Hanzo mainnet.
 *
 * When VITE_NETWORK_MOCK is truthy (default until backend is ready),
 * all calls return realistic mock data so the UI is fully functional.
 */

import { getGlobalIamToken, getGlobalApiKey } from './api';
import type {
  SharingConfig,
  SharingConfigDTO,
  SharingStatus,
  EarningsSummary,
  EarningsSummaryDTO,
  EarningsRecord,
  EarningsRecordDTO,
  NetworkStats,
  NetworkStatsDTO,
  WalletConnection,
  WalletProvider,
} from '@/types/network';

const NETWORK_API =
  import.meta.env.VITE_NETWORK_API_URL ||
  import.meta.env.VITE_COMMERCE_API_URL ||
  'https://commerce.hanzo.ai';

const DEFAULT_TIMEOUT_MS = 10_000;

/** Mock mode — enabled by default until backend endpoints exist. */
const MOCK = import.meta.env.VITE_NETWORK_MOCK !== 'false';

// ---------------------------------------------------------------------------
// DTO ↔ Domain converters
// ---------------------------------------------------------------------------

function fromSharingConfigDTO(d: SharingConfigDTO): SharingConfig {
  return {
    enabled: d.enabled,
    mode: d.mode,
    idleThresholdMinutes: d.idle_threshold_minutes,
    schedule: d.schedule
      ? { days: d.schedule.days, startHour: d.schedule.start_hour, endHour: d.schedule.end_hour, timezone: d.schedule.timezone }
      : null,
    maxCapacityPercent: d.max_capacity_percent,
  };
}

function toSharingConfigDTO(c: SharingConfig): SharingConfigDTO {
  return {
    enabled: c.enabled,
    mode: c.mode,
    idle_threshold_minutes: c.idleThresholdMinutes,
    schedule: c.schedule
      ? { days: c.schedule.days, start_hour: c.schedule.startHour, end_hour: c.schedule.endHour, timezone: c.schedule.timezone }
      : null,
    max_capacity_percent: c.maxCapacityPercent,
  };
}

function fromEarningsSummaryDTO(d: EarningsSummaryDTO): EarningsSummary {
  return {
    totalEarned: d.total_earned,
    todayEarned: d.today_earned,
    weekEarned: d.week_earned,
    monthEarned: d.month_earned,
    currentRatePerHour: d.current_rate_per_hour,
    totalHoursShared: d.total_hours_shared,
  };
}

function fromEarningsRecordDTO(d: EarningsRecordDTO): EarningsRecord {
  return {
    id: d.id,
    timestamp: d.timestamp,
    amount: d.amount,
    durationSeconds: d.duration_seconds,
    computeType: d.compute_type,
  };
}

function fromNetworkStatsDTO(d: NetworkStatsDTO): NetworkStats {
  return {
    activeNodes: d.active_nodes,
    totalDistributed24h: d.total_distributed_24h,
    networkTflops: d.network_tflops,
    avgRatePerHour: d.avg_rate_per_hour,
    userRank: d.user_rank,
    totalContributors: d.total_contributors,
  };
}

// ---------------------------------------------------------------------------
// Auth helpers (same pattern as billingApi.ts)
// ---------------------------------------------------------------------------

function authHeaders(): Record<string, string> {
  const token = getGlobalIamToken() || getGlobalApiKey();
  if (!token) throw new Error('Not authenticated');
  return { Accept: 'application/json', Authorization: `Bearer ${token}` };
}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), DEFAULT_TIMEOUT_MS);
  try {
    const res = await fetch(`${NETWORK_API}${path}`, {
      ...init,
      headers: { ...authHeaders(), ...init?.headers },
      signal: controller.signal,
    });
    if (!res.ok) {
      const text = await res.text().catch(() => '');
      throw new Error(`Network API ${init?.method || 'GET'} ${path} failed (${res.status}): ${text}`.trim());
    }
    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  } finally {
    clearTimeout(timer);
  }
}

// ---------------------------------------------------------------------------
// Mock data
// ---------------------------------------------------------------------------

function mockEarningsHistory(): EarningsRecord[] {
  const now = Date.now();
  return Array.from({ length: 30 }, (_, i) => ({
    id: `earn-${30 - i}`,
    timestamp: new Date(now - i * 86_400_000).toISOString(),
    amount: +(Math.random() * 3 + 0.5).toFixed(2),
    durationSeconds: Math.floor(Math.random() * 14400 + 3600),
    computeType: (['inference', 'cpu', 'gpu'] as const)[i % 3],
  }));
}

const MOCK_CONFIG: SharingConfig = {
  enabled: true,
  mode: 'auto',
  idleThresholdMinutes: 60,
  schedule: null,
  maxCapacityPercent: 80,
};

const MOCK_EARNINGS: EarningsSummary = {
  totalEarned: 42.5,
  todayEarned: 1.85,
  weekEarned: 12.3,
  monthEarned: 38.7,
  currentRatePerHour: 0.23,
  totalHoursShared: 184,
};

const MOCK_STATS: NetworkStats = {
  activeNodes: 1247,
  totalDistributed24h: 2840,
  networkTflops: 156.4,
  avgRatePerHour: 0.19,
  userRank: 342,
  totalContributors: 8920,
};

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

export async function getSharingConfig(): Promise<SharingConfig> {
  if (MOCK) return MOCK_CONFIG;
  const dto = await apiFetch<SharingConfigDTO>('/api/v1/network/sharing/config');
  return fromSharingConfigDTO(dto);
}

export async function putSharingConfig(config: SharingConfig): Promise<void> {
  if (MOCK) return;
  await apiFetch<void>('/api/v1/network/sharing/config', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(toSharingConfigDTO(config)),
  });
}

export async function getSharingStatus(): Promise<{ status: SharingStatus; since: string | null }> {
  if (MOCK) return { status: 'active', since: new Date(Date.now() - 3_600_000).toISOString() };
  return apiFetch('/api/v1/network/sharing/status');
}

export async function getEarningsSummary(): Promise<EarningsSummary> {
  if (MOCK) return MOCK_EARNINGS;
  const dto = await apiFetch<EarningsSummaryDTO>('/api/v1/network/earnings/summary');
  return fromEarningsSummaryDTO(dto);
}

export async function getEarningsHistory(days = 30, limit = 100): Promise<EarningsRecord[]> {
  if (MOCK) return mockEarningsHistory().slice(0, limit);
  const dtos = await apiFetch<EarningsRecordDTO[]>(`/api/v1/network/earnings/history?days=${days}&limit=${limit}`);
  return dtos.map(fromEarningsRecordDTO);
}

export async function getNetworkStats(): Promise<NetworkStats> {
  if (MOCK) return MOCK_STATS;
  const dto = await apiFetch<NetworkStatsDTO>('/api/v1/network/stats');
  return fromNetworkStatsDTO(dto);
}

export async function getWallet(): Promise<WalletConnection | null> {
  if (MOCK) return null;
  try {
    return await apiFetch<WalletConnection>('/api/v1/network/wallet');
  } catch {
    return null;
  }
}

export async function connectWallet(
  provider: WalletProvider,
  address: string,
  chainId: number,
): Promise<WalletConnection> {
  const displayAddress = `${address.slice(0, 6)}...${address.slice(-4)}`;
  if (MOCK) {
    return { provider, address, displayAddress, chainId, connectedAt: new Date().toISOString() };
  }
  return apiFetch<WalletConnection>('/api/v1/network/wallet', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ provider, address, chain_id: chainId }),
  });
}

export async function disconnectWallet(): Promise<void> {
  if (MOCK) return;
  await apiFetch<void>('/api/v1/network/wallet', { method: 'DELETE' });
}

export async function getAiCoinBalance(): Promise<{ balance: number; pending: number }> {
  if (MOCK) return { balance: 42.5, pending: 1.2 };
  return apiFetch('/api/v1/network/balance');
}
