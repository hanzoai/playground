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
