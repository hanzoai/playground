/**
 * UserBalanceBar
 *
 * Bottom-left overlay showing the logged-in user's identity and
 * credit balance from Hanzo Cloud API, with a quick top-up link.
 */

import { useEffect, useState, useCallback } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { getBalance, TOP_UP_URL, type BalanceResult } from '@/services/billingApi';
import { cn } from '@/lib/utils';

const POLL_INTERVAL = 60_000; // refresh balance every 60s

export function UserBalanceBar() {
  const { iamUser, isAuthenticated, authMode } = useAuth();
  const [balance, setBalance] = useState<BalanceResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchBalance = useCallback(async () => {
    try {
      const result = await getBalance();
      setBalance(result);
      setError(null);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg);
      console.warn('[billing] Balance fetch failed:', msg);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!isAuthenticated) return;
    fetchBalance();
    const interval = setInterval(fetchBalance, POLL_INTERVAL);
    return () => clearInterval(interval);
  }, [isAuthenticated, fetchBalance]);

  if (!isAuthenticated) return null;

  const displayName = iamUser?.displayName || iamUser?.name || 'User';
  const email = iamUser?.email;
  const avatarUrl = iamUser?.avatar;
  const initials = displayName
    .split(' ')
    .map((w) => w[0])
    .join('')
    .slice(0, 2)
    .toUpperCase();

  const balanceValue = balance?.available ?? balance?.balance;
  const isLow = balanceValue != null && balanceValue < 1;

  return (
    <div className="absolute bottom-4 right-4 z-10 flex items-center gap-2 rounded-xl border border-border/50 bg-card/90 px-3 py-2 shadow-lg backdrop-blur-sm">
      {/* Avatar */}
      {avatarUrl ? (
        <img
          src={avatarUrl}
          alt={displayName}
          className="h-7 w-7 rounded-full object-cover"
        />
      ) : (
        <div className="flex h-7 w-7 items-center justify-center rounded-full bg-primary/20 text-[10px] font-semibold text-primary">
          {initials}
        </div>
      )}

      {/* User info + balance */}
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-xs font-medium text-foreground truncate max-w-[120px]">
            {displayName}
          </span>
          {authMode === 'iam' && (
            <span className="text-[10px] text-muted-foreground/60">IAM</span>
          )}
        </div>
        <div className="flex items-center gap-1.5">
          {!loading && !error && balanceValue != null && (
            <span
              className={cn(
                'text-xs tabular-nums font-medium',
                isLow ? 'text-red-400' : 'text-emerald-400'
              )}
            >
              ${balanceValue.toFixed(2)}
            </span>
          )}
          {email && (
            <span className="text-[10px] text-muted-foreground/50 truncate max-w-[100px] hidden sm:inline">
              {email}
            </span>
          )}
        </div>
      </div>

      {/* Top-up button — only show when balance is loaded */}
      {!loading && !error && balanceValue != null && (
        <a
          href={TOP_UP_URL}
          target="_blank"
          rel="noopener noreferrer"
          className={cn(
            'ml-1 flex items-center gap-1 rounded-lg px-2 py-1 text-[11px] font-medium',
            'transition-colors',
            isLow
              ? 'bg-red-500/20 text-red-300 hover:bg-red-500/30'
              : 'bg-accent/50 text-muted-foreground hover:bg-accent hover:text-foreground'
          )}
        >
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <path d="M6 2v8M2 6h8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          </svg>
          Top Up
        </a>
      )}
    </div>
  );
}
