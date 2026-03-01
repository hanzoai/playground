// Billing API — calls Commerce REST API for user balance and billing operations.

import { getGlobalIamToken, getGlobalApiKey } from './api';

const COMMERCE_API = import.meta.env.VITE_COMMERCE_API_URL || 'https://commerce.hanzo.ai';
const DEFAULT_TIMEOUT_MS = 10_000;

export interface BalanceResult {
  user: string;
  balance: number;
  currency: string;
  available: number;
  holds?: number;
}

/** Extract user identifier from a JWT token in owner/name format for Commerce. */
function getUserFromToken(token: string): string | null {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]));
    // Commerce stores balances under owner/name format (e.g. "hanzo/a"),
    // not UUID. IAM JWTs include both owner and name claims.
    if (payload.owner && payload.name) {
      return `${payload.owner}/${payload.name}`;
    }
    return payload.sub || payload.email || null;
  } catch {
    return null;
  }
}

/**
 * Get the current user's credit balance from Commerce API.
 * GET /api/v1/billing/balance?user=<sub>&currency=usd
 */
export async function getBalance(): Promise<BalanceResult> {
  const token = getGlobalIamToken() || getGlobalApiKey();
  if (!token) {
    throw new Error('Not authenticated');
  }

  const userId = getUserFromToken(token);
  if (!userId) {
    throw new Error('Cannot determine user identity from token');
  }

  const url = new URL('/api/v1/billing/balance', COMMERCE_API);
  url.searchParams.set('user', userId);
  url.searchParams.set('currency', 'usd');

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), DEFAULT_TIMEOUT_MS);

  try {
    const res = await fetch(url.toString(), {
      method: 'GET',
      headers: {
        Accept: 'application/json',
        Authorization: `Bearer ${token}`,
      },
      signal: controller.signal,
    });

    if (!res.ok) {
      const text = await res.text().catch(() => '');
      throw new Error(`Balance check failed (${res.status}): ${text}`.trim());
    }

    const data = await res.json();
    return {
      user: userId,
      balance: data.balance ?? 0,
      currency: 'usd',
      available: data.available ?? data.balance ?? 0,
      holds: data.holds ?? 0,
    } satisfies BalanceResult;
  } finally {
    clearTimeout(timer);
  }
}

/** Top-up URL — Hanzo billing portal */
export const TOP_UP_URL = 'https://billing.hanzo.ai';
