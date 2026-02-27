/**
 * Bot Wallet API
 *
 * Client for bot wallet management, funding, withdrawals,
 * transaction history, and auto-purchase rules.
 *
 * When VITE_NETWORK_MOCK is truthy (default until backend is ready),
 * all calls return realistic mock data.
 */

import { getGlobalIamToken, getGlobalApiKey } from './api';
import type {
  BotWallet,
  BotWalletDTO,
  WalletTransaction,
  WalletTransactionDTO,
  AutoPurchaseRule,
  AutoPurchaseRuleDTO,
  FundBotParams,
  BotWalletSummary,
  WalletFundingSource,
} from '@/types/botWallet';
import type { MarketplaceOrder } from '@/types/network';

const API_BASE = import.meta.env.VITE_API_BASE_URL || '/v1';
const MOCK = import.meta.env.VITE_NETWORK_MOCK !== 'false';

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------

function headers(): Record<string, string> {
  const token = getGlobalIamToken() || getGlobalApiKey();
  if (!token) throw new Error('Not authenticated');
  return { Accept: 'application/json', Authorization: `Bearer ${token}` };
}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: { ...headers(), ...init?.headers },
  });
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new Error(`Bot Wallet API ${init?.method || 'GET'} ${path} failed (${res.status}): ${text}`.trim());
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

// ---------------------------------------------------------------------------
// DTO converters
// ---------------------------------------------------------------------------

function fromWalletDTO(d: BotWalletDTO): BotWallet {
  return {
    botId: d.bot_id,
    address: d.address,
    displayAddress: d.display_address,
    aiCoinBalance: d.ai_coin_balance,
    usdBalanceCents: d.usd_balance_cents,
    chainId: d.chain_id,
    enabled: d.enabled,
    createdAt: d.created_at,
    updatedAt: d.updated_at,
  };
}

function fromTransactionDTO(d: WalletTransactionDTO): WalletTransaction {
  return {
    id: d.id,
    botId: d.bot_id,
    type: d.type,
    amountAiCoin: d.amount_ai_coin,
    amountUsdCents: d.amount_usd_cents,
    source: d.source,
    status: d.status,
    referenceId: d.reference_id,
    description: d.description,
    txHash: d.tx_hash,
    createdAt: d.created_at,
    confirmedAt: d.confirmed_at,
  };
}

function fromRuleDTO(d: AutoPurchaseRuleDTO): AutoPurchaseRule {
  return {
    id: d.id,
    botId: d.bot_id,
    capacityType: d.capacity_type,
    preferredProvider: d.preferred_provider,
    preferredModel: d.preferred_model,
    maxCentsPerUnit: d.max_cents_per_unit,
    defaultQuantity: d.default_quantity,
    enabled: d.enabled,
    minBalanceTrigger: d.min_balance_trigger,
    createdAt: d.created_at,
    updatedAt: d.updated_at,
  };
}

// ---------------------------------------------------------------------------
// Mock data generators
// ---------------------------------------------------------------------------

function mockAddress(botId: string): string {
  let hash = 0;
  for (let i = 0; i < botId.length; i++) hash = ((hash << 5) - hash + botId.charCodeAt(i)) | 0;
  const hex = Math.abs(hash).toString(16).padStart(8, '0');
  return `0x${hex}a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8${hex.slice(0, 4)}`;
}

function mockWallet(botId: string): BotWallet {
  const addr = mockAddress(botId);
  return {
    botId,
    address: addr,
    displayAddress: `${addr.slice(0, 6)}...${addr.slice(-4)}`,
    aiCoinBalance: +(Math.random() * 40 + 5).toFixed(2),
    usdBalanceCents: Math.floor(Math.random() * 4500 + 500),
    chainId: 1,
    enabled: true,
    createdAt: new Date(Date.now() - 14 * 86_400_000).toISOString(),
    updatedAt: new Date(Date.now() - 3600_000).toISOString(),
  };
}

function mockTransactions(botId: string): WalletTransaction[] {
  const now = Date.now();
  const types: Array<{ type: WalletTransaction['type']; desc: string; src: WalletFundingSource }> = [
    { type: 'fund', desc: 'Funded from user AI coin balance', src: 'user_ai_coin' },
    { type: 'purchase', desc: 'Purchased Claude Code capacity (lst-001)', src: 'user_ai_coin' },
    { type: 'fund', desc: 'Funded from USD balance', src: 'user_usd' },
    { type: 'earning', desc: 'Earned from shared capacity', src: 'external_transfer' },
    { type: 'purchase', desc: 'Purchased inference capacity (lst-005)', src: 'user_ai_coin' },
    { type: 'fund', desc: 'Topped up from AI coin', src: 'user_ai_coin' },
    { type: 'refund', desc: 'Refund for cancelled order ord-004', src: 'user_ai_coin' },
    { type: 'purchase', desc: 'Auto-purchased GPU compute (lst-004)', src: 'user_ai_coin' },
    { type: 'fund', desc: 'Funded from USD balance', src: 'user_usd' },
    { type: 'earning', desc: 'Earned from shared inference capacity', src: 'external_transfer' },
    { type: 'purchase', desc: 'Purchased custom agent (lst-013)', src: 'user_ai_coin' },
    { type: 'fund', desc: 'Funded from AI coin', src: 'user_ai_coin' },
  ];

  return types.map((t, i) => {
    const isDebit = t.type === 'purchase' || t.type === 'withdraw';
    const aiAmount = +(Math.random() * 8 + 1).toFixed(2);
    const usdAmount = Math.floor(aiAmount * 100);
    return {
      id: `tx-${botId.slice(0, 6)}-${12 - i}`,
      botId,
      type: t.type,
      amountAiCoin: isDebit ? -aiAmount : aiAmount,
      amountUsdCents: isDebit ? -usdAmount : usdAmount,
      source: t.src,
      status: i < 2 ? 'pending' as const : 'confirmed' as const,
      referenceId: t.type === 'purchase' ? `ord-${100 + i}` : null,
      description: t.desc,
      txHash: i >= 2 ? `0x${Math.random().toString(16).slice(2, 18)}${Math.random().toString(16).slice(2, 18)}` : null,
      createdAt: new Date(now - i * 86_400_000 * 2.5).toISOString(),
      confirmedAt: i >= 2 ? new Date(now - i * 86_400_000 * 2.5 + 60_000).toISOString() : null,
    };
  });
}

function mockRules(botId: string): AutoPurchaseRule[] {
  const now = new Date().toISOString();
  return [
    {
      id: `rule-${botId.slice(0, 6)}-1`,
      botId,
      capacityType: 'claude-code',
      preferredProvider: 'Anthropic',
      preferredModel: 'claude-opus-4-6',
      maxCentsPerUnit: 120,
      defaultQuantity: 4,
      enabled: true,
      minBalanceTrigger: 5,
      createdAt: now,
      updatedAt: now,
    },
    {
      id: `rule-${botId.slice(0, 6)}-2`,
      botId,
      capacityType: 'inference',
      preferredProvider: null,
      preferredModel: null,
      maxCentsPerUnit: 50,
      defaultQuantity: 8,
      enabled: false,
      minBalanceTrigger: 10,
      createdAt: now,
      updatedAt: now,
    },
  ];
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

export async function getBotWallet(botId: string): Promise<BotWallet> {
  if (MOCK) return mockWallet(botId);
  const dto = await apiFetch<BotWalletDTO>(`/bots/${botId}/wallet`);
  return fromWalletDTO(dto);
}

export async function fundBotWallet(params: FundBotParams): Promise<WalletTransaction> {
  if (MOCK) {
    return {
      id: `tx-${Date.now()}`,
      botId: params.botId,
      type: 'fund',
      amountAiCoin: params.amountAiCoin,
      amountUsdCents: params.amountUsdCents,
      source: params.source,
      status: 'confirmed',
      referenceId: null,
      description: `Funded ${params.amountAiCoin} AI coin from ${params.source.replace('_', ' ')}`,
      txHash: `0x${Math.random().toString(16).slice(2, 18)}${Math.random().toString(16).slice(2, 18)}`,
      createdAt: new Date().toISOString(),
      confirmedAt: new Date().toISOString(),
    };
  }
  const dto = await apiFetch<WalletTransactionDTO>(`/bots/${params.botId}/wallet/fund`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      source: params.source,
      amount_ai_coin: params.amountAiCoin,
      amount_usd_cents: params.amountUsdCents,
    }),
  });
  return fromTransactionDTO(dto);
}

export async function withdrawFromBotWallet(
  botId: string,
  amountAiCoin: number,
  amountUsdCents: number,
): Promise<WalletTransaction> {
  if (MOCK) {
    return {
      id: `tx-${Date.now()}`,
      botId,
      type: 'withdraw',
      amountAiCoin: -amountAiCoin,
      amountUsdCents: -amountUsdCents,
      source: 'user_ai_coin',
      status: 'confirmed',
      referenceId: null,
      description: `Withdrew ${amountAiCoin} AI coin to user wallet`,
      txHash: `0x${Math.random().toString(16).slice(2, 18)}${Math.random().toString(16).slice(2, 18)}`,
      createdAt: new Date().toISOString(),
      confirmedAt: new Date().toISOString(),
    };
  }
  const dto = await apiFetch<WalletTransactionDTO>(`/bots/${botId}/wallet/withdraw`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ amount_ai_coin: amountAiCoin, amount_usd_cents: amountUsdCents }),
  });
  return fromTransactionDTO(dto);
}

export async function getBotWalletTransactions(botId: string, limit = 50): Promise<WalletTransaction[]> {
  if (MOCK) return mockTransactions(botId).slice(0, limit);
  const dtos = await apiFetch<WalletTransactionDTO[]>(`/bots/${botId}/wallet/transactions?limit=${limit}`);
  return dtos.map(fromTransactionDTO);
}

export async function getAutoPurchaseRules(botId: string): Promise<AutoPurchaseRule[]> {
  if (MOCK) return mockRules(botId);
  const dtos = await apiFetch<AutoPurchaseRuleDTO[]>(`/bots/${botId}/wallet/auto-purchase`);
  return dtos.map(fromRuleDTO);
}

export async function createAutoPurchaseRule(
  botId: string,
  rule: Omit<AutoPurchaseRule, 'id' | 'botId' | 'createdAt' | 'updatedAt'>,
): Promise<AutoPurchaseRule> {
  if (MOCK) {
    return {
      ...rule,
      id: `rule-${Date.now()}`,
      botId,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
  }
  const dto = await apiFetch<AutoPurchaseRuleDTO>(`/bots/${botId}/wallet/auto-purchase`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      capacity_type: rule.capacityType,
      preferred_provider: rule.preferredProvider,
      preferred_model: rule.preferredModel,
      max_cents_per_unit: rule.maxCentsPerUnit,
      default_quantity: rule.defaultQuantity,
      enabled: rule.enabled,
      min_balance_trigger: rule.minBalanceTrigger,
    }),
  });
  return fromRuleDTO(dto);
}

export async function updateAutoPurchaseRule(
  botId: string,
  ruleId: string,
  updates: Partial<AutoPurchaseRule>,
): Promise<AutoPurchaseRule> {
  if (MOCK) {
    const existing = mockRules(botId).find((r) => r.id === ruleId) ?? mockRules(botId)[0];
    return { ...existing, ...updates, updatedAt: new Date().toISOString() };
  }
  const dto = await apiFetch<AutoPurchaseRuleDTO>(`/bots/${botId}/wallet/auto-purchase/${ruleId}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(updates),
  });
  return fromRuleDTO(dto);
}

export async function deleteAutoPurchaseRule(botId: string, ruleId: string): Promise<void> {
  if (MOCK) return;
  await apiFetch<void>(`/bots/${botId}/wallet/auto-purchase/${ruleId}`, { method: 'DELETE' });
}

export async function executeAutoPurchaseRule(botId: string, ruleId: string): Promise<MarketplaceOrder> {
  if (MOCK) {
    const rule = mockRules(botId).find((r) => r.id === ruleId) ?? mockRules(botId)[0];
    return {
      id: `ord-auto-${Date.now()}`,
      listingId: 'lst-001',
      buyerId: botId,
      sellerId: 'u-alice',
      capacityType: rule.capacityType,
      model: rule.preferredModel ?? 'claude-opus-4-6',
      quantity: rule.defaultQuantity,
      unit: 'hour',
      totalCostCents: rule.defaultQuantity * rule.maxCentsPerUnit,
      status: 'pending',
      proxyEndpoint: null,
      s3TransferBucket: null,
      usedQuantity: 0,
      createdAt: new Date().toISOString(),
      startedAt: null,
      completedAt: null,
      sellerDisplayName: 'Alice K.',
      listingTitle: 'Claude Code — Opus tier, 24/7 availability',
    };
  }
  return apiFetch<MarketplaceOrder>(`/bots/${botId}/wallet/auto-purchase/${ruleId}/execute`, { method: 'POST' });
}

export async function getBotWalletSummary(): Promise<BotWalletSummary> {
  if (MOCK) return { totalBots: 3, totalAiCoin: 28.4, totalUsdCents: 8450 };
  return apiFetch<BotWalletSummary>('/bots/wallets/summary');
}
