/**
 * Commerce API helper for E2E billing/credits tests.
 *
 * All Hanzo apps/services use IAM auth — the JWT access token from hanzo.id
 * works across everything (commerce at api.hanzo.ai, app.hanzo.bot, etc.).
 */

interface Balance {
  balance: number;   // in cents
  holds: number;
  available: number;
  currency: string;
}

interface Transaction {
  id: string;
  type: string;       // 'Deposit' | 'Withdraw' | 'Hold' | 'Transfer'
  amount: number;     // cents
  currency: string;
  tags: string[];
  created_at: string;
  expires_at?: string;
  metadata?: Record<string, unknown>;
}

export class CommerceHelper {
  private commerceUrl: string;
  private token: string;

  constructor(token: string, commerceUrl?: string) {
    this.token = token;
    this.commerceUrl = commerceUrl || process.env.E2E_COMMERCE_API_URL || 'https://commerce.hanzo.ai';
  }

  private async request<T>(path: string, opts: RequestInit = {}): Promise<T> {
    const url = `${this.commerceUrl}${path}`;
    const res = await fetch(url, {
      ...opts,
      headers: {
        'Authorization': `Bearer ${this.token}`,
        'Content-Type': 'application/json',
        ...opts.headers,
      },
    });

    if (!res.ok) {
      const body = await res.text();
      throw new Error(`Commerce API ${opts.method || 'GET'} ${path} → ${res.status}: ${body}`);
    }

    return res.json() as Promise<T>;
  }

  /**
   * Get balance for the authenticated user in a given currency.
   */
  async getBalance(userId: string, currency: string = 'usd'): Promise<Balance> {
    return this.request<Balance>(
      `/api/v1/billing/balance?user=${encodeURIComponent(userId)}&currency=${currency}`
    );
  }

  /**
   * Get all balances across currencies.
   */
  async getAllBalances(userId: string): Promise<Record<string, Balance>> {
    return this.request<Record<string, Balance>>(
      `/api/v1/billing/balance/all?user=${encodeURIComponent(userId)}`
    );
  }

  /**
   * Get usage records (transactions tagged "api-usage").
   */
  async getUsageRecords(userId: string, currency: string = 'usd'): Promise<Transaction[]> {
    return this.request<Transaction[]>(
      `/api/v1/billing/usage?user=${encodeURIComponent(userId)}&currency=${currency}`
    );
  }

  /**
   * Grant starter credit ($5 trial) to user.
   * Idempotent-ish — Commerce may reject if already granted.
   */
  async grantStarterCredit(userId: string): Promise<Transaction> {
    return this.request<Transaction>('/api/v1/billing/credit', {
      method: 'POST',
      body: JSON.stringify({ user: userId }),
    });
  }

  /**
   * Record a usage transaction (Withdraw).
   */
  async recordUsage(params: {
    user: string;
    amount: number;
    currency?: string;
    model?: string;
    provider?: string;
    tags?: string[];
  }): Promise<Transaction> {
    return this.request<Transaction>('/api/v1/billing/usage', {
      method: 'POST',
      body: JSON.stringify({
        currency: 'usd',
        ...params,
      }),
    });
  }

  /**
   * Verify the user has sufficient credit to execute bots.
   * Returns true if available balance > 0.
   */
  async hasCredit(userId: string): Promise<boolean> {
    try {
      const balance = await this.getBalance(userId);
      return balance.available > 0;
    } catch {
      // If Commerce API is unreachable, we can't verify — skip gracefully
      return false;
    }
  }

  /**
   * Get the trial credit amount in dollars.
   */
  async getTrialCreditDollars(userId: string): Promise<number> {
    const balance = await this.getBalance(userId);
    return balance.available / 100; // cents → dollars
  }
}
