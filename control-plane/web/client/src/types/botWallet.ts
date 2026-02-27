/**
 * Bot Wallet Types
 *
 * Types for bot-specific wallets, on-chain funding, transaction history,
 * and auto-purchase rules. Each bot gets a server-managed custodial wallet
 * that users fund from their AI coin or USD balance. Bots use wallet
 * funds to autonomously purchase marketplace capacity.
 */

import type { CapacityType } from './network';

// ---------------------------------------------------------------------------
// Bot Wallet
// ---------------------------------------------------------------------------

export type WalletFundingSource = 'user_ai_coin' | 'user_usd' | 'external_transfer';

export type WalletTransactionType =
  | 'fund'
  | 'withdraw'
  | 'purchase'
  | 'refund'
  | 'earning';

export type WalletTransactionStatus = 'pending' | 'confirmed' | 'failed';

export interface BotWallet {
  botId: string;
  /** On-chain wallet address (server-managed custodial wallet). */
  address: string;
  /** Truncated display: 0x1234…abcd */
  displayAddress: string;
  /** AI coin balance. */
  aiCoinBalance: number;
  /** USD credit balance (cents). */
  usdBalanceCents: number;
  /** Chain ID for on-chain settlement. */
  chainId: number;
  /** Whether the wallet is active and can transact. */
  enabled: boolean;
  /** ISO-8601 */
  createdAt: string;
  /** ISO-8601 */
  updatedAt: string;
}

export interface WalletTransaction {
  id: string;
  botId: string;
  type: WalletTransactionType;
  /** Positive = credit, negative = debit. */
  amountAiCoin: number;
  amountUsdCents: number;
  source: WalletFundingSource;
  status: WalletTransactionStatus;
  /** Optional reference (e.g., marketplace order ID). */
  referenceId: string | null;
  description: string;
  /** On-chain tx hash (null until confirmed on-chain). */
  txHash: string | null;
  /** ISO-8601 */
  createdAt: string;
  /** ISO-8601 */
  confirmedAt: string | null;
}

// ---------------------------------------------------------------------------
// Auto-Purchase Rules
// ---------------------------------------------------------------------------

export interface AutoPurchaseRule {
  id: string;
  botId: string;
  /** What capacity type to auto-buy. */
  capacityType: CapacityType;
  /** Preferred provider (e.g. "Anthropic"). Null = any. */
  preferredProvider: string | null;
  /** Preferred model (e.g. "claude-opus-4-6"). Null = any. */
  preferredModel: string | null;
  /** Maximum price in USD cents per unit the bot will pay. */
  maxCentsPerUnit: number;
  /** Default quantity to purchase per trigger. */
  defaultQuantity: number;
  /** Whether this rule is active. */
  enabled: boolean;
  /** Minimum wallet balance (AI coin) before auto-purchase triggers. */
  minBalanceTrigger: number;
  /** ISO-8601 */
  createdAt: string;
  /** ISO-8601 */
  updatedAt: string;
}

// ---------------------------------------------------------------------------
// Request params
// ---------------------------------------------------------------------------

export interface FundBotParams {
  botId: string;
  source: WalletFundingSource;
  amountAiCoin: number;
  amountUsdCents: number;
}

export interface BotWalletSummary {
  totalBots: number;
  totalAiCoin: number;
  totalUsdCents: number;
}

// ---------------------------------------------------------------------------
// DTOs (snake_case for backend compatibility)
// ---------------------------------------------------------------------------

export interface BotWalletDTO {
  bot_id: string;
  address: string;
  display_address: string;
  ai_coin_balance: number;
  usd_balance_cents: number;
  chain_id: number;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface WalletTransactionDTO {
  id: string;
  bot_id: string;
  type: WalletTransactionType;
  amount_ai_coin: number;
  amount_usd_cents: number;
  source: WalletFundingSource;
  status: WalletTransactionStatus;
  reference_id: string | null;
  description: string;
  tx_hash: string | null;
  created_at: string;
  confirmed_at: string | null;
}

export interface AutoPurchaseRuleDTO {
  id: string;
  bot_id: string;
  capacity_type: CapacityType;
  preferred_provider: string | null;
  preferred_model: string | null;
  max_cents_per_unit: number;
  default_quantity: number;
  enabled: boolean;
  min_balance_trigger: number;
  created_at: string;
  updated_at: string;
}
