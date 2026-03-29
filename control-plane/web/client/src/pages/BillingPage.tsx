import { useState, useEffect } from 'react';
import { getBalance, TOP_UP_URL, type BalanceResult } from '@/services/billingApi';
import { getGlobalIamToken, getGlobalApiKey } from '@/services/api';

const BILLING_API = `${import.meta.env.VITE_API_BASE_URL || ''}/v1/billing`;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function authHeaders(): HeadersInit {
  const token = getGlobalIamToken() || getGlobalApiKey();
  return {
    Accept: 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
}

/** Decode JWT to extract user id in owner/name format used by Commerce. */
function getUserId(): string | null {
  const token = getGlobalIamToken() || getGlobalApiKey();
  if (!token) return null;
  try {
    const payload = JSON.parse(atob(token.split('.')[1]));
    if (payload.owner && payload.name) return `${payload.owner}/${payload.name}`;
    return payload.sub || payload.email || null;
  } catch {
    return null;
  }
}

function fmt(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

function fmtDate(d: string | number | undefined): string {
  if (!d) return '-';
  return new Date(d).toLocaleDateString();
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface Transaction {
  id?: string;
  date?: string;
  createdAt?: string;
  description?: string;
  memo?: string;
  amount?: number;
  type?: string;
  kind?: string;
  tags?: string[];
}

interface CreditGrant {
  id?: string;
  name?: string;
  amount?: number;
  remaining?: number;
  balance?: number;
  expiresAt?: string;
  expiration?: string;
  status?: string;
}

interface Subscription {
  id?: string;
  plan?: string | { name?: string; id?: string };
  status?: string;
  currentPeriodEnd?: string;
}

// ---------------------------------------------------------------------------
// Data fetching
// ---------------------------------------------------------------------------

async function fetchJson<T>(path: string, params: Record<string, string> = {}): Promise<T> {
  const url = new URL(`${window.location.origin}${BILLING_API}${path}`);
  for (const [k, v] of Object.entries(params)) url.searchParams.set(k, v);
  const res = await fetch(url.toString(), { headers: authHeaders() });
  if (!res.ok) throw new Error(`${res.status}`);
  return res.json();
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function BillingPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [balance, setBalance] = useState<BalanceResult | null>(null);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [grants, setGrants] = useState<CreditGrant[]>([]);
  const [subscription, setSubscription] = useState<Subscription | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      const userId = getUserId();
      if (!userId) {
        setError('Not authenticated — please sign in.');
        setLoading(false);
        return;
      }

      try {
        // Fetch all in parallel; individual failures are non-fatal.
        const [balRes, txRes, grantRes, subRes] = await Promise.allSettled([
          getBalance(),
          fetchJson<any>('/transactions', { user: userId, limit: '50' }),
          fetchJson<any>('/credit-grants', { userId }),
          fetchJson<any>('/subscriptions', { userId }),
        ]);

        if (cancelled) return;

        if (balRes.status === 'fulfilled') setBalance(balRes.value);
        if (txRes.status === 'fulfilled') {
          const raw = txRes.value;
          setTransactions(Array.isArray(raw) ? raw : raw?.transactions ?? []);
        }
        if (grantRes.status === 'fulfilled') {
          const raw = grantRes.value;
          setGrants(Array.isArray(raw) ? raw : raw?.grants ?? []);
        }
        if (subRes.status === 'fulfilled') {
          const raw = subRes.value;
          const subs: Subscription[] = Array.isArray(raw) ? raw : raw?.subscriptions ?? [];
          setSubscription(subs.find((s) => s.status === 'active') ?? subs[0] ?? null);
        }
      } catch (e: any) {
        if (!cancelled) setError(e.message ?? 'Failed to load billing data');
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    load();
    return () => { cancelled = true; };
  }, []);

  // Derived usage stats from transactions
  const usageTransactions = transactions.filter(
    (t) => t.tags?.includes('api-usage') || t.type === 'withdraw',
  );
  const mtdSpend = usageTransactions.reduce((sum, t) => sum + Math.abs(t.amount ?? 0), 0);
  const apiCalls = usageTransactions.length;

  const recentTx = transactions.slice(0, 20);

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full min-h-[400px]">
        <div className="text-white/40 text-sm animate-pulse">Loading billing data...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-full min-h-[400px]">
        <div className="text-red-400 text-sm">{error}</div>
      </div>
    );
  }

  return (
    <div className="w-full max-w-5xl mx-auto px-6 py-8 space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-white">Billing &amp; Usage</h1>
          <p className="text-sm text-white/40 mt-1">
            Manage your balance, view transactions, and monitor usage.
          </p>
        </div>
        <a
          href="https://billing.hanzo.ai"
          target="_blank"
          rel="noopener noreferrer"
          className="text-sm text-blue-400 hover:text-blue-300 transition-colors"
        >
          Full billing portal &rarr;
        </a>
      </div>

      {/* Balance + Usage cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {/* Balance */}
        <div className="rounded-lg border border-white/10 bg-[#111118] p-5 space-y-3">
          <div className="text-xs uppercase tracking-wider text-white/40">Balance</div>
          <div className="text-3xl font-bold text-white">
            {balance ? fmt(balance.available) : '$0.00'}
          </div>
          {balance && balance.holds ? (
            <div className="text-xs text-white/30">
              {fmt(balance.holds)} on hold
            </div>
          ) : null}
          <a
            href={TOP_UP_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-block mt-2 px-4 py-1.5 rounded text-sm font-medium bg-blue-600 hover:bg-blue-500 text-white transition-colors"
          >
            Top up
          </a>
        </div>

        {/* Month-to-date spend */}
        <div className="rounded-lg border border-white/10 bg-[#111118] p-5 space-y-3">
          <div className="text-xs uppercase tracking-wider text-white/40">Month-to-Date Spend</div>
          <div className="text-3xl font-bold text-white">{fmt(mtdSpend)}</div>
          <div className="text-xs text-white/30">{apiCalls} API calls</div>
        </div>

        {/* Current Plan */}
        <div className="rounded-lg border border-white/10 bg-[#111118] p-5 space-y-3">
          <div className="text-xs uppercase tracking-wider text-white/40">Current Plan</div>
          {subscription ? (
            <>
              <div className="text-xl font-semibold text-white">
                {typeof subscription.plan === 'string'
                  ? subscription.plan
                  : subscription.plan?.name ?? 'Active Plan'}
              </div>
              <div className="text-xs text-white/30">
                Status: {subscription.status ?? 'active'}
                {subscription.currentPeriodEnd &&
                  ` \u00B7 Renews ${fmtDate(subscription.currentPeriodEnd)}`}
              </div>
            </>
          ) : (
            <div className="text-white/30 text-sm">No active plan</div>
          )}
        </div>
      </div>

      {/* Recent Transactions */}
      <section className="space-y-3">
        <h2 className="text-lg font-medium text-white">Recent Transactions</h2>
        {recentTx.length === 0 ? (
          <div className="text-sm text-white/30 py-6 text-center border border-white/10 rounded-lg bg-[#111118]">
            No transactions yet.
          </div>
        ) : (
          <div className="overflow-x-auto rounded-lg border border-white/10 bg-[#111118]">
            <table className="w-full text-sm text-left">
              <thead>
                <tr className="border-b border-white/10 text-white/40 text-xs uppercase tracking-wider">
                  <th className="px-4 py-3">Date</th>
                  <th className="px-4 py-3">Description</th>
                  <th className="px-4 py-3 text-right">Amount</th>
                  <th className="px-4 py-3">Type</th>
                </tr>
              </thead>
              <tbody>
                {recentTx.map((tx, i) => {
                  const amount = tx.amount ?? 0;
                  const isCredit = amount > 0 || tx.type === 'deposit';
                  return (
                    <tr
                      key={tx.id ?? i}
                      className="border-b border-white/5 last:border-0 hover:bg-white/[0.02] transition-colors"
                    >
                      <td className="px-4 py-3 text-white/60 whitespace-nowrap">
                        {fmtDate(tx.date ?? tx.createdAt)}
                      </td>
                      <td className="px-4 py-3 text-white/80">
                        {tx.description ?? tx.memo ?? '-'}
                      </td>
                      <td
                        className={`px-4 py-3 text-right font-mono whitespace-nowrap ${
                          isCredit ? 'text-emerald-400' : 'text-white/80'
                        }`}
                      >
                        {isCredit ? '+' : '-'}{fmt(Math.abs(amount))}
                      </td>
                      <td className="px-4 py-3">
                        <span
                          className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${
                            isCredit
                              ? 'bg-emerald-500/10 text-emerald-400'
                              : 'bg-white/5 text-white/50'
                          }`}
                        >
                          {tx.type ?? tx.kind ?? (isCredit ? 'deposit' : 'withdraw')}
                        </span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* Credit Grants */}
      <section className="space-y-3">
        <h2 className="text-lg font-medium text-white">Credit Grants</h2>
        {grants.length === 0 ? (
          <div className="text-sm text-white/30 py-6 text-center border border-white/10 rounded-lg bg-[#111118]">
            No active credit grants.
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            {grants.map((g, i) => {
              const remaining = g.remaining ?? g.balance ?? 0;
              const total = g.amount ?? remaining;
              const pct = total > 0 ? Math.min((remaining / total) * 100, 100) : 0;
              const expires = g.expiresAt ?? g.expiration;
              return (
                <div
                  key={g.id ?? i}
                  className="rounded-lg border border-white/10 bg-[#111118] p-4 space-y-2"
                >
                  <div className="flex items-center justify-between">
                    <span className="text-white/80 text-sm font-medium">
                      {g.name ?? `Grant #${i + 1}`}
                    </span>
                    {g.status && (
                      <span className="text-xs text-white/30">{g.status}</span>
                    )}
                  </div>
                  <div className="text-white text-lg font-semibold">
                    {fmt(remaining)}{' '}
                    <span className="text-white/30 text-sm font-normal">
                      / {fmt(total)}
                    </span>
                  </div>
                  {/* Progress bar */}
                  <div className="w-full h-1.5 rounded-full bg-white/5 overflow-hidden">
                    <div
                      className="h-full rounded-full bg-blue-500 transition-all"
                      style={{ width: `${pct}%` }}
                    />
                  </div>
                  {expires && (
                    <div className="text-xs text-white/30">
                      Expires {fmtDate(expires)}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </section>
    </div>
  );
}
