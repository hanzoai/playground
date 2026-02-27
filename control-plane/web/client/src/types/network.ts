/**
 * Network Marketplace Types
 *
 * Types for AI capacity sharing, wallet connection, and earnings tracking
 * on the Hanzo network. Users share unused AI/LLM capacity when idle and
 * earn AI coin on the Hanzo mainnet.
 */

// ---------------------------------------------------------------------------
// Sharing
// ---------------------------------------------------------------------------

export type SharingStatus = 'active' | 'idle' | 'disabled' | 'cooldown';

export type SharingMode = 'auto' | 'manual' | 'scheduled';

export interface SharingSchedule {
  /** Days of week when sharing is active (0=Sun … 6=Sat). */
  days: number[];
  /** Start hour (0–23) in user's local timezone. */
  startHour: number;
  /** End hour (0–23) in user's local timezone. */
  endHour: number;
  /** IANA timezone identifier (e.g. "America/Los_Angeles"). */
  timezone: string;
}

export interface SharingConfig {
  /** Whether capacity sharing is enabled. Default: true (opt-in). */
  enabled: boolean;
  /** How sharing is activated. */
  mode: SharingMode;
  /** Minutes of idle time before auto-sharing kicks in. Default: 60. */
  idleThresholdMinutes: number;
  /** Schedule for scheduled mode. Null when mode !== 'scheduled'. */
  schedule: SharingSchedule | null;
  /** Maximum percentage of capacity to share (10–100). Default: 80. */
  maxCapacityPercent: number;
}

// ---------------------------------------------------------------------------
// Earnings
// ---------------------------------------------------------------------------

export interface EarningsRecord {
  id: string;
  /** ISO-8601 timestamp. */
  timestamp: string;
  /** AI coin amount earned. */
  amount: number;
  /** Duration of capacity sharing in seconds. */
  durationSeconds: number;
  /** Type of compute contributed. */
  computeType: 'cpu' | 'gpu' | 'inference';
}

export interface EarningsSummary {
  /** Total AI coin earned all-time. */
  totalEarned: number;
  /** AI coin earned in the current 24h period. */
  todayEarned: number;
  /** AI coin earned in the last 7 days. */
  weekEarned: number;
  /** AI coin earned in the last 30 days. */
  monthEarned: number;
  /** Current earning rate (AI coin per hour). Zero when not sharing. */
  currentRatePerHour: number;
  /** Total hours of capacity shared all-time. */
  totalHoursShared: number;
}

// ---------------------------------------------------------------------------
// Wallet
// ---------------------------------------------------------------------------

export type WalletProvider = 'metamask' | 'walletconnect' | 'coinbase' | 'hanzo';

export interface WalletConnection {
  provider: WalletProvider;
  /** Full address. */
  address: string;
  /** Truncated display form: 0x1234…abcd */
  displayAddress: string;
  chainId: number;
  /** ISO-8601 timestamp. */
  connectedAt: string;
}

// ---------------------------------------------------------------------------
// Network Stats
// ---------------------------------------------------------------------------

export interface NetworkStats {
  /** Total nodes currently sharing capacity. */
  activeNodes: number;
  /** Total AI coin distributed in the last 24h. */
  totalDistributed24h: number;
  /** Network-wide compute throughput (TFLOPS). */
  networkTflops: number;
  /** Average earning rate per node per hour. */
  avgRatePerHour: number;
  /** User's rank among all sharers (null if not sharing). */
  userRank: number | null;
  /** Total unique contributors all-time. */
  totalContributors: number;
}

// ---------------------------------------------------------------------------
// DTOs (snake_case for backend compatibility)
// ---------------------------------------------------------------------------

export interface SharingConfigDTO {
  enabled: boolean;
  mode: SharingMode;
  idle_threshold_minutes: number;
  schedule: {
    days: number[];
    start_hour: number;
    end_hour: number;
    timezone: string;
  } | null;
  max_capacity_percent: number;
}

export interface EarningsSummaryDTO {
  total_earned: number;
  today_earned: number;
  week_earned: number;
  month_earned: number;
  current_rate_per_hour: number;
  total_hours_shared: number;
}

export interface EarningsRecordDTO {
  id: string;
  timestamp: string;
  amount: number;
  duration_seconds: number;
  compute_type: 'cpu' | 'gpu' | 'inference';
}

export interface NetworkStatsDTO {
  active_nodes: number;
  total_distributed_24h: number;
  network_tflops: number;
  avg_rate_per_hour: number;
  user_rank: number | null;
  total_contributors: number;
}

// ---------------------------------------------------------------------------
// Marketplace — Listings
// ---------------------------------------------------------------------------

export type ListingStatus = 'active' | 'paused' | 'sold_out' | 'expired';

export type CapacityType = 'claude-code' | 'api-key' | 'gpu-compute' | 'inference';

export type PricingUnit = 'hour' | 'request' | 'token_1k';

export interface ListingPricing {
  /** Price in USD cents per unit. */
  centsPerUnit: number;
  /** Unit of pricing. */
  unit: PricingUnit;
  /** Minimum purchase quantity. */
  minUnits: number;
  /** Maximum purchase quantity (null = unlimited). */
  maxUnits: number | null;
}

export interface MarketplaceListing {
  id: string;
  sellerId: string;
  sellerDisplayName: string;
  /** What kind of AI capacity. */
  capacityType: CapacityType;
  /** Human-readable title. */
  title: string;
  /** Detailed description. */
  description: string;
  /** Provider name (e.g. "Anthropic", "OpenAI"). */
  provider: string;
  /** Specific model available (e.g. "claude-sonnet-4-20250514"). */
  model: string;
  pricing: ListingPricing;
  /** Total hours/units available. */
  totalCapacity: number;
  /** Remaining hours/units available for purchase. */
  remainingCapacity: number;
  status: ListingStatus;
  /** Average rating from buyers (1–5 scale). */
  rating: number;
  /** Number of completed orders. */
  totalOrders: number;
  /** ISO-8601 created timestamp. */
  createdAt: string;
  /** ISO-8601 expiry (null = no expiry). */
  expiresAt: string | null;
  /** Whether this listing is a resale of purchased capacity. */
  isResale: boolean;
  /** Parent listing ID if resale. */
  parentListingId: string | null;
}

// ---------------------------------------------------------------------------
// Marketplace — Orders
// ---------------------------------------------------------------------------

export type OrderStatus = 'pending' | 'active' | 'completed' | 'cancelled' | 'disputed';

export interface MarketplaceOrder {
  id: string;
  listingId: string;
  buyerId: string;
  sellerId: string;
  /** Capacity type from listing. */
  capacityType: CapacityType;
  /** Model from listing. */
  model: string;
  /** Quantity purchased. */
  quantity: number;
  /** Unit from listing pricing. */
  unit: PricingUnit;
  /** Total cost in USD cents. */
  totalCostCents: number;
  status: OrderStatus;
  /** Proxy endpoint for routed requests (populated when active). */
  proxyEndpoint: string | null;
  /** S3 workload transfer bucket (populated when active). */
  s3TransferBucket: string | null;
  /** Units consumed so far. */
  usedQuantity: number;
  /** ISO-8601 timestamps. */
  createdAt: string;
  startedAt: string | null;
  completedAt: string | null;
  /** Seller display name for UI. */
  sellerDisplayName: string;
  /** Listing title for UI. */
  listingTitle: string;
}

// ---------------------------------------------------------------------------
// Marketplace — Seller Dashboard
// ---------------------------------------------------------------------------

export interface ListingRevenue {
  listingId: string;
  title: string;
  totalRevenueCents: number;
  activeOrders: number;
  completedOrders: number;
  capacityUsedPercent: number;
}

export interface SellerDashboard {
  totalRevenueCents: number;
  todayRevenueCents: number;
  weekRevenueCents: number;
  monthRevenueCents: number;
  activeListings: number;
  totalOrders: number;
  activeOrders: number;
  avgRating: number;
  listings: ListingRevenue[];
}

// ---------------------------------------------------------------------------
// Marketplace — Stats
// ---------------------------------------------------------------------------

export interface MarketplaceStats {
  totalListings: number;
  activeListings: number;
  totalVolumeCents24h: number;
  avgPriceCentsPerHour: number;
  uniqueSellers: number;
  uniqueBuyers: number;
}

// ---------------------------------------------------------------------------
// Marketplace — Params
// ---------------------------------------------------------------------------

export interface CreateListingParams {
  capacityType: CapacityType;
  title: string;
  description: string;
  provider: string;
  model: string;
  pricing: ListingPricing;
  totalCapacity: number;
  expiresAt: string | null;
  /** For resale: the order ID providing the capacity. */
  sourceOrderId: string | null;
}

export interface UpdateListingParams {
  title?: string;
  description?: string;
  pricing?: Partial<ListingPricing>;
  totalCapacity?: number;
  status?: 'active' | 'paused';
  expiresAt?: string | null;
}

export interface CreateOrderParams {
  listingId: string;
  quantity: number;
}

export interface MarketplaceFilter {
  capacityType: CapacityType | 'all';
  provider: string | 'all';
  sortBy: 'price_asc' | 'price_desc' | 'rating' | 'newest';
  searchQuery: string;
}

// ---------------------------------------------------------------------------
// Marketplace DTOs (snake_case for backend compatibility)
// ---------------------------------------------------------------------------

export interface ListingPricingDTO {
  cents_per_unit: number;
  unit: PricingUnit;
  min_units: number;
  max_units: number | null;
}

export interface MarketplaceListingDTO {
  id: string;
  seller_id: string;
  seller_display_name: string;
  capacity_type: CapacityType;
  title: string;
  description: string;
  provider: string;
  model: string;
  pricing: ListingPricingDTO;
  total_capacity: number;
  remaining_capacity: number;
  status: ListingStatus;
  rating: number;
  total_orders: number;
  created_at: string;
  expires_at: string | null;
  is_resale: boolean;
  parent_listing_id: string | null;
}

export interface MarketplaceOrderDTO {
  id: string;
  listing_id: string;
  buyer_id: string;
  seller_id: string;
  capacity_type: CapacityType;
  model: string;
  quantity: number;
  unit: PricingUnit;
  total_cost_cents: number;
  status: OrderStatus;
  proxy_endpoint: string | null;
  s3_transfer_bucket: string | null;
  used_quantity: number;
  created_at: string;
  started_at: string | null;
  completed_at: string | null;
  seller_display_name: string;
  listing_title: string;
}

export interface SellerDashboardDTO {
  total_revenue_cents: number;
  today_revenue_cents: number;
  week_revenue_cents: number;
  month_revenue_cents: number;
  active_listings: number;
  total_orders: number;
  active_orders: number;
  avg_rating: number;
  listings: {
    listing_id: string;
    title: string;
    total_revenue_cents: number;
    active_orders: number;
    completed_orders: number;
    capacity_used_percent: number;
  }[];
}

export interface MarketplaceStatsDTO {
  total_listings: number;
  active_listings: number;
  total_volume_cents_24h: number;
  avg_price_cents_per_hour: number;
  unique_sellers: number;
  unique_buyers: number;
}
