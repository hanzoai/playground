// Billing API — calls Hanzo Cloud API for user balance via ZAP protocol

import { getGlobalIamToken, getGlobalApiKey } from './api';

const HANZO_API = import.meta.env.VITE_HANZO_API_URL || 'https://api.hanzo.ai';

export interface BalanceResult {
  user: string;
  balance: number;
  currency: string;
  available: number;
}

/**
 * Get the current user's credit balance from Hanzo Cloud API.
 * Uses ZAP protocol: POST /zap { method: "billing.balance" }
 */
export async function getBalance(): Promise<BalanceResult> {
  const token = getGlobalIamToken() || getGlobalApiKey();
  if (!token) {
    throw new Error('Not authenticated');
  }

  const res = await fetch(`${HANZO_API}/zap`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
    body: JSON.stringify({
      method: 'billing.balance',
      id: `bal-${Date.now()}`,
      params: { currency: 'usd' },
    }),
  });

  if (!res.ok) {
    throw new Error(`Balance check failed: ${res.status}`);
  }

  const data = await res.json();
  if (data.error) {
    throw new Error(data.error.message || 'Balance check failed');
  }

  return data.result as BalanceResult;
}

/** Top-up URL — Hanzo IAM billing portal */
export const TOP_UP_URL = 'https://iam.hanzo.ai/billing';
