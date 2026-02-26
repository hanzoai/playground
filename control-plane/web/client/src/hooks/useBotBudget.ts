import { useCallback, useEffect, useState } from 'react';
import {
  getBudget,
  setBudget,
  deleteBudget,
  checkBudget,
  getSpendHistory,
  type BotBudget,
  type BudgetStatus,
  type BotSpendRecord,
} from '../services/budgetApi';

interface UseBotBudgetReturn {
  budget: BotBudget | null;
  status: BudgetStatus | null;
  spendHistory: BotSpendRecord[];
  loading: boolean;
  error: string | null;
  saveBudget: (updates: Partial<BotBudget>) => Promise<void>;
  removeBudget: () => Promise<void>;
  refresh: () => Promise<void>;
}

export function useBotBudget(botId: string | undefined): UseBotBudgetReturn {
  const [budget, setBudgetState] = useState<BotBudget | null>(null);
  const [status, setStatus] = useState<BudgetStatus | null>(null);
  const [spendHistory, setSpendHistory] = useState<BotSpendRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    if (!botId) return;
    try {
      setLoading(true);
      setError(null);
      const [b, s, h] = await Promise.all([
        getBudget(botId),
        checkBudget(botId).catch(() => null),
        getSpendHistory(botId, 30, 50).catch(() => []),
      ]);
      setBudgetState(b);
      setStatus(s);
      setSpendHistory(h);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load budget');
    } finally {
      setLoading(false);
    }
  }, [botId]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const saveBudget = useCallback(async (updates: Partial<BotBudget>) => {
    if (!botId) return;
    try {
      const result = await setBudget(botId, updates);
      setBudgetState(result);
      const s = await checkBudget(botId).catch(() => null);
      setStatus(s);
    } catch (err) {
      throw err;
    }
  }, [botId]);

  const removeBudget = useCallback(async () => {
    if (!botId) return;
    await deleteBudget(botId);
    setBudgetState(null);
    setStatus(null);
  }, [botId]);

  return { budget, status, spendHistory, loading, error, saveBudget, removeBudget, refresh };
}
