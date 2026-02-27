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
  MarketplaceListing,
  MarketplaceListingDTO,
  MarketplaceOrder,
  MarketplaceOrderDTO,
  SellerDashboard,
  SellerDashboardDTO,
  MarketplaceStats,
  MarketplaceStatsDTO,
  CreateListingParams,
  UpdateListingParams,
  CreateOrderParams,
  MarketplaceFilter,
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

// ---------------------------------------------------------------------------
// Marketplace DTO converters
// ---------------------------------------------------------------------------

function fromListingDTO(d: MarketplaceListingDTO): MarketplaceListing {
  return {
    id: d.id,
    sellerId: d.seller_id,
    sellerDisplayName: d.seller_display_name,
    capacityType: d.capacity_type,
    title: d.title,
    description: d.description,
    provider: d.provider,
    model: d.model,
    pricing: {
      centsPerUnit: d.pricing.cents_per_unit,
      unit: d.pricing.unit,
      minUnits: d.pricing.min_units,
      maxUnits: d.pricing.max_units,
    },
    totalCapacity: d.total_capacity,
    remainingCapacity: d.remaining_capacity,
    status: d.status,
    rating: d.rating,
    totalOrders: d.total_orders,
    createdAt: d.created_at,
    expiresAt: d.expires_at,
    isResale: d.is_resale,
    parentListingId: d.parent_listing_id,
  };
}

function fromOrderDTO(d: MarketplaceOrderDTO): MarketplaceOrder {
  return {
    id: d.id,
    listingId: d.listing_id,
    buyerId: d.buyer_id,
    sellerId: d.seller_id,
    capacityType: d.capacity_type,
    model: d.model,
    quantity: d.quantity,
    unit: d.unit,
    totalCostCents: d.total_cost_cents,
    status: d.status,
    proxyEndpoint: d.proxy_endpoint,
    s3TransferBucket: d.s3_transfer_bucket,
    usedQuantity: d.used_quantity,
    createdAt: d.created_at,
    startedAt: d.started_at,
    completedAt: d.completed_at,
    sellerDisplayName: d.seller_display_name,
    listingTitle: d.listing_title,
  };
}

function fromSellerDashboardDTO(d: SellerDashboardDTO): SellerDashboard {
  return {
    totalRevenueCents: d.total_revenue_cents,
    todayRevenueCents: d.today_revenue_cents,
    weekRevenueCents: d.week_revenue_cents,
    monthRevenueCents: d.month_revenue_cents,
    activeListings: d.active_listings,
    totalOrders: d.total_orders,
    activeOrders: d.active_orders,
    avgRating: d.avg_rating,
    listings: d.listings.map((l) => ({
      listingId: l.listing_id,
      title: l.title,
      totalRevenueCents: l.total_revenue_cents,
      activeOrders: l.active_orders,
      completedOrders: l.completed_orders,
      capacityUsedPercent: l.capacity_used_percent,
    })),
  };
}

function fromMarketplaceStatsDTO(d: MarketplaceStatsDTO): MarketplaceStats {
  return {
    totalListings: d.total_listings,
    activeListings: d.active_listings,
    totalVolumeCents24h: d.total_volume_cents_24h,
    avgPriceCentsPerHour: d.avg_price_cents_per_hour,
    uniqueSellers: d.unique_sellers,
    uniqueBuyers: d.unique_buyers,
  };
}

// ---------------------------------------------------------------------------
// Marketplace mock data
// ---------------------------------------------------------------------------

const MOCK_LISTINGS: MarketplaceListing[] = [
  {
    id: 'lst-001', sellerId: 'u-alice', sellerDisplayName: 'Alice K.',
    capacityType: 'claude-code', title: 'Claude Code — Opus tier, 24/7 availability',
    description: 'Full Claude Code access with Opus-level API key. High rate limits, low latency. Available 24/7 with automatic failover. Ideal for code generation, review, and refactoring workloads.',
    provider: 'Anthropic', model: 'claude-opus-4-6',
    pricing: { centsPerUnit: 100, unit: 'hour', minUnits: 1, maxUnits: 24 },
    totalCapacity: 720, remainingCapacity: 548, status: 'active',
    rating: 4.8, totalOrders: 47, createdAt: new Date(Date.now() - 30 * 86_400_000).toISOString(),
    expiresAt: null, isResale: false, parentListingId: null,
  },
  {
    id: 'lst-002', sellerId: 'u-bob', sellerDisplayName: 'Bob M.',
    capacityType: 'claude-code', title: 'Claude Sonnet — Budget-friendly dev assistant',
    description: 'Sonnet-tier Claude Code. Great for everyday coding tasks, debugging, and documentation. Reliable uptime with 99.5% SLA.',
    provider: 'Anthropic', model: 'claude-sonnet-4-20250514',
    pricing: { centsPerUnit: 50, unit: 'hour', minUnits: 2, maxUnits: 48 },
    totalCapacity: 1440, remainingCapacity: 1120, status: 'active',
    rating: 4.5, totalOrders: 89, createdAt: new Date(Date.now() - 45 * 86_400_000).toISOString(),
    expiresAt: null, isResale: false, parentListingId: null,
  },
  {
    id: 'lst-003', sellerId: 'u-carol', sellerDisplayName: 'Carol T.',
    capacityType: 'api-key', title: 'GPT-4o API — High throughput',
    description: 'OpenAI GPT-4o API access with tier-5 rate limits. Supports function calling, vision, and JSON mode. Proxied through dedicated VM in us-east-1.',
    provider: 'OpenAI', model: 'gpt-4o',
    pricing: { centsPerUnit: 75, unit: 'hour', minUnits: 1, maxUnits: 12 },
    totalCapacity: 360, remainingCapacity: 180, status: 'active',
    rating: 4.6, totalOrders: 63, createdAt: new Date(Date.now() - 20 * 86_400_000).toISOString(),
    expiresAt: null, isResale: false, parentListingId: null,
  },
  {
    id: 'lst-004', sellerId: 'u-dave', sellerDisplayName: 'Dave R.',
    capacityType: 'gpu-compute', title: 'A100 GPU — ML training & inference',
    description: 'NVIDIA A100 80GB GPU access for ML workloads. Pre-configured with CUDA 12, PyTorch 2.x. S3 transfer for datasets and checkpoints.',
    provider: 'NVIDIA', model: 'a100-80gb',
    pricing: { centsPerUnit: 250, unit: 'hour', minUnits: 1, maxUnits: 8 },
    totalCapacity: 168, remainingCapacity: 92, status: 'active',
    rating: 4.9, totalOrders: 28, createdAt: new Date(Date.now() - 15 * 86_400_000).toISOString(),
    expiresAt: null, isResale: false, parentListingId: null,
  },
  {
    id: 'lst-005', sellerId: 'u-eve', sellerDisplayName: 'Eve S.',
    capacityType: 'inference', title: 'Llama 3.3 70B — Self-hosted inference',
    description: 'Llama 3.3 70B running on dedicated hardware. No rate limits. Private deployment, zero data retention. Great for sensitive workloads.',
    provider: 'Meta', model: 'llama-3.3-70b',
    pricing: { centsPerUnit: 30, unit: 'hour', minUnits: 1, maxUnits: null },
    totalCapacity: 2160, remainingCapacity: 1800, status: 'active',
    rating: 4.3, totalOrders: 112, createdAt: new Date(Date.now() - 60 * 86_400_000).toISOString(),
    expiresAt: null, isResale: false, parentListingId: null,
  },
  {
    id: 'lst-006', sellerId: 'u-frank', sellerDisplayName: 'Frank W.',
    capacityType: 'claude-code', title: '[RESOLD] Claude Opus — 8hr blocks',
    description: 'Resold from bulk purchase. Claude Opus access in 8-hour blocks at a slight premium for convenience. Instant activation.',
    provider: 'Anthropic', model: 'claude-opus-4-6',
    pricing: { centsPerUnit: 125, unit: 'hour', minUnits: 8, maxUnits: 8 },
    totalCapacity: 96, remainingCapacity: 48, status: 'active',
    rating: 4.7, totalOrders: 6, createdAt: new Date(Date.now() - 5 * 86_400_000).toISOString(),
    expiresAt: null, isResale: true, parentListingId: 'lst-001',
  },
  {
    id: 'lst-007', sellerId: 'u-grace', sellerDisplayName: 'Grace L.',
    capacityType: 'api-key', title: 'Gemini 2.5 Pro — Google AI',
    description: 'Google Gemini 2.5 Pro access with 1M context window. Excellent for long-document analysis and multimodal tasks.',
    provider: 'Google', model: 'gemini-2.5-pro',
    pricing: { centsPerUnit: 60, unit: 'hour', minUnits: 1, maxUnits: 24 },
    totalCapacity: 720, remainingCapacity: 600, status: 'active',
    rating: 4.4, totalOrders: 34, createdAt: new Date(Date.now() - 25 * 86_400_000).toISOString(),
    expiresAt: null, isResale: false, parentListingId: null,
  },
  {
    id: 'lst-008', sellerId: 'u-hank', sellerDisplayName: 'Hank P.',
    capacityType: 'inference', title: 'Mixtral 8x22B — Cost-effective MoE',
    description: 'Mixtral 8x22B mixture-of-experts model. Fast inference, low cost. Great for batch processing and classification tasks.',
    provider: 'Mistral', model: 'mixtral-8x22b',
    pricing: { centsPerUnit: 15, unit: 'hour', minUnits: 4, maxUnits: null },
    totalCapacity: 4320, remainingCapacity: 3800, status: 'active',
    rating: 4.1, totalOrders: 156, createdAt: new Date(Date.now() - 90 * 86_400_000).toISOString(),
    expiresAt: null, isResale: false, parentListingId: null,
  },
  {
    id: 'lst-009', sellerId: 'u-iris', sellerDisplayName: 'Iris C.',
    capacityType: 'claude-code', title: 'Claude Haiku — Quick tasks, low cost',
    description: 'Claude Haiku for lightweight coding tasks. Fastest response times, lowest cost. Perfect for code completion and simple automation.',
    provider: 'Anthropic', model: 'claude-haiku-4-5-20251001',
    pricing: { centsPerUnit: 20, unit: 'hour', minUnits: 1, maxUnits: null },
    totalCapacity: 2880, remainingCapacity: 2400, status: 'active',
    rating: 4.2, totalOrders: 203, createdAt: new Date(Date.now() - 40 * 86_400_000).toISOString(),
    expiresAt: null, isResale: false, parentListingId: null,
  },
  {
    id: 'lst-010', sellerId: 'u-jake', sellerDisplayName: 'Jake N.',
    capacityType: 'gpu-compute', title: 'H100 GPU — Premium inference',
    description: 'NVIDIA H100 SXM 80GB for demanding inference and fine-tuning. TensorRT optimized, NVLink interconnect. S3 bucket provisioned per order.',
    provider: 'NVIDIA', model: 'h100-sxm-80gb',
    pricing: { centsPerUnit: 400, unit: 'hour', minUnits: 1, maxUnits: 4 },
    totalCapacity: 96, remainingCapacity: 64, status: 'active',
    rating: 5.0, totalOrders: 12, createdAt: new Date(Date.now() - 10 * 86_400_000).toISOString(),
    expiresAt: null, isResale: false, parentListingId: null,
  },
  {
    id: 'lst-011', sellerId: 'u-kate', sellerDisplayName: 'Kate V.',
    capacityType: 'api-key', title: 'GPT-4o mini — Bulk token packages',
    description: 'GPT-4o mini API access sold per 1K tokens. Buy in bulk for batch processing. Consistent throughput, no rate limit issues.',
    provider: 'OpenAI', model: 'gpt-4o-mini',
    pricing: { centsPerUnit: 2, unit: 'token_1k', minUnits: 100, maxUnits: 10000 },
    totalCapacity: 50000, remainingCapacity: 42000, status: 'active',
    rating: 4.3, totalOrders: 78, createdAt: new Date(Date.now() - 35 * 86_400_000).toISOString(),
    expiresAt: null, isResale: false, parentListingId: null,
  },
  {
    id: 'lst-012', sellerId: 'u-leo', sellerDisplayName: 'Leo D.',
    capacityType: 'claude-code', title: 'Claude Sonnet — Paused (back next week)',
    description: 'Temporarily paused while upgrading infrastructure. Will resume with faster proxy and S3 transfer support.',
    provider: 'Anthropic', model: 'claude-sonnet-4-20250514',
    pricing: { centsPerUnit: 55, unit: 'hour', minUnits: 1, maxUnits: 24 },
    totalCapacity: 480, remainingCapacity: 480, status: 'paused',
    rating: 4.6, totalOrders: 52, createdAt: new Date(Date.now() - 50 * 86_400_000).toISOString(),
    expiresAt: null, isResale: false, parentListingId: null,
  },
];

const MOCK_ORDERS: MarketplaceOrder[] = [
  {
    id: 'ord-001', listingId: 'lst-001', buyerId: 'u-me', sellerId: 'u-alice',
    capacityType: 'claude-code', model: 'claude-opus-4-6', quantity: 8, unit: 'hour',
    totalCostCents: 800, status: 'active',
    proxyEndpoint: 'https://proxy.hanzo.ai/v1/ord-001',
    s3TransferBucket: 's3://hanzo-xfer-ord-001',
    usedQuantity: 3.5, createdAt: new Date(Date.now() - 2 * 86_400_000).toISOString(),
    startedAt: new Date(Date.now() - 2 * 86_400_000).toISOString(), completedAt: null,
    sellerDisplayName: 'Alice K.', listingTitle: 'Claude Code — Opus tier, 24/7 availability',
  },
  {
    id: 'ord-002', listingId: 'lst-003', buyerId: 'u-me', sellerId: 'u-carol',
    capacityType: 'api-key', model: 'gpt-4o', quantity: 4, unit: 'hour',
    totalCostCents: 300, status: 'completed',
    proxyEndpoint: null, s3TransferBucket: null,
    usedQuantity: 4, createdAt: new Date(Date.now() - 10 * 86_400_000).toISOString(),
    startedAt: new Date(Date.now() - 10 * 86_400_000).toISOString(),
    completedAt: new Date(Date.now() - 9 * 86_400_000).toISOString(),
    sellerDisplayName: 'Carol T.', listingTitle: 'GPT-4o API — High throughput',
  },
  {
    id: 'ord-003', listingId: 'lst-005', buyerId: 'u-me', sellerId: 'u-eve',
    capacityType: 'inference', model: 'llama-3.3-70b', quantity: 12, unit: 'hour',
    totalCostCents: 360, status: 'active',
    proxyEndpoint: 'https://proxy.hanzo.ai/v1/ord-003',
    s3TransferBucket: 's3://hanzo-xfer-ord-003',
    usedQuantity: 7.2, createdAt: new Date(Date.now() - 1 * 86_400_000).toISOString(),
    startedAt: new Date(Date.now() - 1 * 86_400_000).toISOString(), completedAt: null,
    sellerDisplayName: 'Eve S.', listingTitle: 'Llama 3.3 70B — Self-hosted inference',
  },
  {
    id: 'ord-004', listingId: 'lst-002', buyerId: 'u-me', sellerId: 'u-bob',
    capacityType: 'claude-code', model: 'claude-sonnet-4-20250514', quantity: 24, unit: 'hour',
    totalCostCents: 1200, status: 'pending',
    proxyEndpoint: null, s3TransferBucket: null,
    usedQuantity: 0, createdAt: new Date(Date.now() - 3600_000).toISOString(),
    startedAt: null, completedAt: null,
    sellerDisplayName: 'Bob M.', listingTitle: 'Claude Sonnet — Budget-friendly dev assistant',
  },
  {
    id: 'ord-005', listingId: 'lst-004', buyerId: 'u-me', sellerId: 'u-dave',
    capacityType: 'gpu-compute', model: 'a100-80gb', quantity: 2, unit: 'hour',
    totalCostCents: 500, status: 'completed',
    proxyEndpoint: null, s3TransferBucket: null,
    usedQuantity: 2, createdAt: new Date(Date.now() - 7 * 86_400_000).toISOString(),
    startedAt: new Date(Date.now() - 7 * 86_400_000).toISOString(),
    completedAt: new Date(Date.now() - 7 * 86_400_000 + 7200_000).toISOString(),
    sellerDisplayName: 'Dave R.', listingTitle: 'A100 GPU — ML training & inference',
  },
];

const MOCK_SELLER_DASHBOARD: SellerDashboard = {
  totalRevenueCents: 18450,
  todayRevenueCents: 350,
  weekRevenueCents: 2800,
  monthRevenueCents: 12600,
  activeListings: 3,
  totalOrders: 47,
  activeOrders: 5,
  avgRating: 4.7,
  listings: [
    { listingId: 'lst-001', title: 'Claude Code — Opus tier', totalRevenueCents: 9400, activeOrders: 3, completedOrders: 44, capacityUsedPercent: 24 },
    { listingId: 'lst-006', title: '[RESOLD] Claude Opus — 8hr blocks', totalRevenueCents: 6000, activeOrders: 1, completedOrders: 5, capacityUsedPercent: 50 },
    { listingId: 'lst-009', title: 'Claude Haiku — Quick tasks', totalRevenueCents: 3050, activeOrders: 1, completedOrders: 14, capacityUsedPercent: 17 },
  ],
};

const MOCK_MARKETPLACE_STATS: MarketplaceStats = {
  totalListings: 847,
  activeListings: 623,
  totalVolumeCents24h: 1240000,
  avgPriceCentsPerHour: 85,
  uniqueSellers: 412,
  uniqueBuyers: 1893,
};

// ---------------------------------------------------------------------------
// Marketplace API
// ---------------------------------------------------------------------------

export async function getMarketplaceListings(filters?: Partial<MarketplaceFilter>): Promise<MarketplaceListing[]> {
  if (MOCK) {
    let results = [...MOCK_LISTINGS];
    if (filters?.capacityType && filters.capacityType !== 'all') {
      results = results.filter((l) => l.capacityType === filters.capacityType);
    }
    if (filters?.provider && filters.provider !== 'all') {
      results = results.filter((l) => l.provider.toLowerCase() === filters.provider!.toLowerCase());
    }
    if (filters?.searchQuery) {
      const q = filters.searchQuery.toLowerCase();
      results = results.filter((l) => l.title.toLowerCase().includes(q) || l.description.toLowerCase().includes(q) || l.model.toLowerCase().includes(q));
    }
    if (filters?.sortBy === 'price_asc') results.sort((a, b) => a.pricing.centsPerUnit - b.pricing.centsPerUnit);
    else if (filters?.sortBy === 'price_desc') results.sort((a, b) => b.pricing.centsPerUnit - a.pricing.centsPerUnit);
    else if (filters?.sortBy === 'rating') results.sort((a, b) => b.rating - a.rating);
    else results.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime());
    return results;
  }
  const params = new URLSearchParams();
  if (filters?.capacityType && filters.capacityType !== 'all') params.set('capacity_type', filters.capacityType);
  if (filters?.provider && filters.provider !== 'all') params.set('provider', filters.provider);
  if (filters?.searchQuery) params.set('q', filters.searchQuery);
  if (filters?.sortBy) params.set('sort', filters.sortBy);
  const qs = params.toString();
  const dtos = await apiFetch<MarketplaceListingDTO[]>(`/api/v1/network/marketplace/listings${qs ? `?${qs}` : ''}`);
  return dtos.map(fromListingDTO);
}

export async function getListingById(id: string): Promise<MarketplaceListing> {
  if (MOCK) {
    const listing = MOCK_LISTINGS.find((l) => l.id === id);
    if (!listing) throw new Error(`Listing ${id} not found`);
    return listing;
  }
  const dto = await apiFetch<MarketplaceListingDTO>(`/api/v1/network/marketplace/listings/${id}`);
  return fromListingDTO(dto);
}

export async function createListing(params: CreateListingParams): Promise<MarketplaceListing> {
  if (MOCK) {
    const listing: MarketplaceListing = {
      id: `lst-${Date.now()}`,
      sellerId: 'u-me',
      sellerDisplayName: 'You',
      capacityType: params.capacityType,
      title: params.title,
      description: params.description,
      provider: params.provider,
      model: params.model,
      pricing: params.pricing,
      totalCapacity: params.totalCapacity,
      remainingCapacity: params.totalCapacity,
      status: 'active',
      rating: 0,
      totalOrders: 0,
      createdAt: new Date().toISOString(),
      expiresAt: params.expiresAt,
      isResale: !!params.sourceOrderId,
      parentListingId: null,
    };
    return listing;
  }
  const dto = await apiFetch<MarketplaceListingDTO>('/api/v1/network/marketplace/listings', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  });
  return fromListingDTO(dto);
}

export async function updateListing(id: string, params: UpdateListingParams): Promise<void> {
  if (MOCK) return;
  await apiFetch<void>(`/api/v1/network/marketplace/listings/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  });
}

export async function deleteListing(id: string): Promise<void> {
  if (MOCK) return;
  await apiFetch<void>(`/api/v1/network/marketplace/listings/${id}`, { method: 'DELETE' });
}

export async function getMyListings(): Promise<MarketplaceListing[]> {
  if (MOCK) return MOCK_LISTINGS.filter((l) => ['lst-001', 'lst-006', 'lst-009'].includes(l.id));
  const dtos = await apiFetch<MarketplaceListingDTO[]>('/api/v1/network/marketplace/listings/mine');
  return dtos.map(fromListingDTO);
}

export async function createOrder(params: CreateOrderParams): Promise<MarketplaceOrder> {
  if (MOCK) {
    const listing = MOCK_LISTINGS.find((l) => l.id === params.listingId);
    const order: MarketplaceOrder = {
      id: `ord-${Date.now()}`,
      listingId: params.listingId,
      buyerId: 'u-me',
      sellerId: listing?.sellerId ?? 'u-unknown',
      capacityType: listing?.capacityType ?? 'api-key',
      model: listing?.model ?? 'unknown',
      quantity: params.quantity,
      unit: listing?.pricing.unit ?? 'hour',
      totalCostCents: params.quantity * (listing?.pricing.centsPerUnit ?? 0),
      status: 'pending',
      proxyEndpoint: null,
      s3TransferBucket: null,
      usedQuantity: 0,
      createdAt: new Date().toISOString(),
      startedAt: null,
      completedAt: null,
      sellerDisplayName: listing?.sellerDisplayName ?? 'Unknown',
      listingTitle: listing?.title ?? 'Unknown listing',
    };
    return order;
  }
  const dto = await apiFetch<MarketplaceOrderDTO>('/api/v1/network/marketplace/orders', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ listing_id: params.listingId, quantity: params.quantity }),
  });
  return fromOrderDTO(dto);
}

export async function getMyOrders(): Promise<MarketplaceOrder[]> {
  if (MOCK) return MOCK_ORDERS;
  const dtos = await apiFetch<MarketplaceOrderDTO[]>('/api/v1/network/marketplace/orders/mine');
  return dtos.map(fromOrderDTO);
}

export async function cancelOrder(id: string): Promise<void> {
  if (MOCK) return;
  await apiFetch<void>(`/api/v1/network/marketplace/orders/${id}/cancel`, { method: 'POST' });
}

export async function getSellerDashboard(): Promise<SellerDashboard> {
  if (MOCK) return MOCK_SELLER_DASHBOARD;
  const dto = await apiFetch<SellerDashboardDTO>('/api/v1/network/marketplace/seller/dashboard');
  return fromSellerDashboardDTO(dto);
}

export async function getMarketplaceStats(): Promise<MarketplaceStats> {
  if (MOCK) return MOCK_MARKETPLACE_STATS;
  const dto = await apiFetch<MarketplaceStatsDTO>('/api/v1/network/marketplace/stats');
  return fromMarketplaceStatsDTO(dto);
}

export async function createResaleListing(orderId: string, params: CreateListingParams): Promise<MarketplaceListing> {
  if (MOCK) {
    const listing = await createListing({ ...params, sourceOrderId: orderId });
    return { ...listing, isResale: true };
  }
  const dto = await apiFetch<MarketplaceListingDTO>('/api/v1/network/marketplace/resale', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ order_id: orderId, ...params }),
  });
  return fromListingDTO(dto);
}
