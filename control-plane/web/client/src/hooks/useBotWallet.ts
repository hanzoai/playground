import { useCallback, useEffect, useState } from 'react';
import {
  getBotWallet,
  fundBotWallet,
  withdrawFromBotWallet,
  getBotWalletTransactions,
  getAutoPurchaseRules,
  createAutoPurchaseRule,
  updateAutoPurchaseRule,
  deleteAutoPurchaseRule,
  executeAutoPurchaseRule,
} from '../services/botWalletApi';
import type { BotWallet, WalletTransaction, AutoPurchaseRule, WalletFundingSource } from '../types/botWallet';
import type { MarketplaceOrder } from '../types/network';

interface UseBotWalletReturn {
  wallet: BotWallet | null;
  transactions: WalletTransaction[];
  autoPurchaseRules: AutoPurchaseRule[];
  loading: boolean;
  error: string | null;
  fundWallet: (source: WalletFundingSource, aiCoin: number, usdCents: number) => Promise<void>;
  withdrawFromWallet: (aiCoin: number, usdCents: number) => Promise<void>;
  saveAutoPurchaseRule: (rule: Partial<AutoPurchaseRule>) => Promise<void>;
  removeAutoPurchaseRule: (ruleId: string) => Promise<void>;
  triggerAutoPurchase: (ruleId: string) => Promise<MarketplaceOrder>;
  refresh: () => Promise<void>;
}

export function useBotWallet(botId: string | undefined): UseBotWalletReturn {
  const [wallet, setWallet] = useState<BotWallet | null>(null);
  const [transactions, setTransactions] = useState<WalletTransaction[]>([]);
  const [autoPurchaseRules, setAutoPurchaseRules] = useState<AutoPurchaseRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    if (!botId) return;
    try {
      setLoading(true);
      setError(null);
      const [w, txs, rules] = await Promise.all([
        getBotWallet(botId),
        getBotWalletTransactions(botId, 50).catch(() => []),
        getAutoPurchaseRules(botId).catch(() => []),
      ]);
      setWallet(w);
      setTransactions(txs);
      setAutoPurchaseRules(rules);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load bot wallet');
    } finally {
      setLoading(false);
    }
  }, [botId]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const fundWallet = useCallback(async (source: WalletFundingSource, aiCoin: number, usdCents: number) => {
    if (!botId) return;
    await fundBotWallet({ botId, source, amountAiCoin: aiCoin, amountUsdCents: usdCents });
    await refresh();
  }, [botId, refresh]);

  const withdrawFromWallet = useCallback(async (aiCoin: number, usdCents: number) => {
    if (!botId) return;
    await withdrawFromBotWallet(botId, aiCoin, usdCents);
    await refresh();
  }, [botId, refresh]);

  const saveAutoPurchaseRule = useCallback(async (rule: Partial<AutoPurchaseRule>) => {
    if (!botId) return;
    if (rule.id) {
      const updated = await updateAutoPurchaseRule(botId, rule.id, rule);
      setAutoPurchaseRules((prev) => prev.map((r) => r.id === updated.id ? updated : r));
    } else {
      const created = await createAutoPurchaseRule(botId, {
        capacityType: rule.capacityType ?? 'claude-code',
        preferredProvider: rule.preferredProvider ?? null,
        preferredModel: rule.preferredModel ?? null,
        maxCentsPerUnit: rule.maxCentsPerUnit ?? 100,
        defaultQuantity: rule.defaultQuantity ?? 4,
        enabled: rule.enabled ?? true,
        minBalanceTrigger: rule.minBalanceTrigger ?? 5,
      });
      setAutoPurchaseRules((prev) => [created, ...prev]);
    }
  }, [botId]);

  const removeAutoPurchaseRule = useCallback(async (ruleId: string) => {
    if (!botId) return;
    await deleteAutoPurchaseRule(botId, ruleId);
    setAutoPurchaseRules((prev) => prev.filter((r) => r.id !== ruleId));
  }, [botId]);

  const triggerAutoPurchase = useCallback(async (ruleId: string) => {
    if (!botId) throw new Error('No bot ID');
    const order = await executeAutoPurchaseRule(botId, ruleId);
    await refresh();
    return order;
  }, [botId, refresh]);

  return {
    wallet,
    transactions,
    autoPurchaseRules,
    loading,
    error,
    fundWallet,
    withdrawFromWallet,
    saveAutoPurchaseRule,
    removeAutoPurchaseRule,
    triggerAutoPurchase,
    refresh,
  };
}
