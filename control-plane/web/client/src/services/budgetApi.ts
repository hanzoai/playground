import { getGlobalApiKey } from './api';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/v1';

export interface BotBudget {
  bot_id: string;
  monthly_limit_usd: number;
  daily_limit_usd: number;
  alert_threshold: number;
  enabled: boolean;
  current_month_usd: number;
  current_day_usd: number;
  last_reset_date: string;
  updated_at: string;
  created_at: string;
}

export interface BudgetStatus {
  allowed: boolean;
  reason?: string;
  monthly_limit_usd: number;
  monthly_spent_usd: number;
  daily_limit_usd: number;
  daily_spent_usd: number;
  alert_triggered: boolean;
}

export interface BotSpendRecord {
  id: number;
  bot_id: string;
  execution_id: string;
  amount_usd: number;
  description: string;
  recorded_at: string;
}

function headers(): HeadersInit {
  const h: Record<string, string> = { 'Content-Type': 'application/json' };
  const apiKey = getGlobalApiKey();
  if (apiKey) h['X-API-Key'] = apiKey;
  return h;
}

export async function listBudgets(): Promise<BotBudget[]> {
  const res = await fetch(`${API_BASE_URL}/budgets`, { headers: headers() });
  if (!res.ok) throw new Error(`Failed to list budgets: ${res.status}`);
  return res.json();
}

export async function getBudget(botId: string): Promise<BotBudget | null> {
  const res = await fetch(`${API_BASE_URL}/budgets/${botId}`, { headers: headers() });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`Failed to get budget: ${res.status}`);
  return res.json();
}

export async function setBudget(botId: string, budget: Partial<BotBudget>): Promise<BotBudget> {
  const res = await fetch(`${API_BASE_URL}/budgets/${botId}`, {
    method: 'PUT',
    headers: headers(),
    body: JSON.stringify(budget),
  });
  if (!res.ok) throw new Error(`Failed to set budget: ${res.status}`);
  return res.json();
}

export async function deleteBudget(botId: string): Promise<void> {
  const res = await fetch(`${API_BASE_URL}/budgets/${botId}`, {
    method: 'DELETE',
    headers: headers(),
  });
  if (!res.ok) throw new Error(`Failed to delete budget: ${res.status}`);
}

export async function checkBudget(botId: string): Promise<BudgetStatus> {
  const res = await fetch(`${API_BASE_URL}/budgets/${botId}/check`, { headers: headers() });
  if (!res.ok) throw new Error(`Failed to check budget: ${res.status}`);
  return res.json();
}

export async function getSpendHistory(botId: string, days = 30, limit = 100): Promise<BotSpendRecord[]> {
  const res = await fetch(`${API_BASE_URL}/budgets/${botId}/spend?days=${days}&limit=${limit}`, { headers: headers() });
  if (!res.ok) throw new Error(`Failed to get spend history: ${res.status}`);
  return res.json();
}
