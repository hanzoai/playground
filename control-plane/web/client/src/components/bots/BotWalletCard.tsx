/**
 * BotWalletCard — displays bot wallet balance, recent transactions, and fund/withdraw controls.
 */

import { useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Sparkline } from '@/components/ui/Sparkline';
import { BudgetProgressRing } from './BudgetProgressRing';
import { FundBotDialog } from './FundBotDialog';
import {
  Wallet,
  Coins,
  HandCoins,
} from '@/components/ui/icon-bridge';
import { cn } from '@/lib/utils';
import type { BotWallet, WalletTransaction, WalletFundingSource } from '@/types/botWallet';

interface BotWalletCardProps {
  botId: string;
  wallet: BotWallet | null;
  transactions: WalletTransaction[];
  userAiCoinBalance: number;
  userUsdBalanceCents: number;
  onFund: (source: WalletFundingSource, aiCoin: number, usdCents: number) => Promise<void>;
  onWithdraw: (aiCoin: number, usdCents: number) => Promise<void>;
  className?: string;
}

const TX_TYPE_LABEL: Record<WalletTransaction['type'], { label: string; color: string }> = {
  fund: { label: 'Fund', color: 'text-emerald-600 bg-emerald-500/10' },
  withdraw: { label: 'Withdraw', color: 'text-amber-600 bg-amber-500/10' },
  purchase: { label: 'Purchase', color: 'text-blue-600 bg-blue-500/10' },
  refund: { label: 'Refund', color: 'text-purple-600 bg-purple-500/10' },
  earning: { label: 'Earning', color: 'text-emerald-600 bg-emerald-500/10' },
};

const STATUS_DOT: Record<WalletTransaction['status'], string> = {
  pending: 'bg-amber-500',
  confirmed: 'bg-emerald-500',
  failed: 'bg-red-500',
};

function formatAiCoin(v: number): string {
  return v >= 1000 ? `${(v / 1000).toFixed(1)}k` : v.toFixed(1);
}

function computeDailyTotals(txs: WalletTransaction[]): number[] {
  const byDay = new Map<string, number>();
  for (const tx of txs) {
    const day = tx.createdAt.slice(0, 10);
    byDay.set(day, (byDay.get(day) ?? 0) + Math.abs(tx.amountAiCoin));
  }
  const days: number[] = [];
  const now = new Date();
  for (let i = 29; i >= 0; i--) {
    const d = new Date(now);
    d.setDate(d.getDate() - i);
    days.push(byDay.get(d.toISOString().slice(0, 10)) ?? 0);
  }
  return days;
}

export function BotWalletCard({
  botId,
  wallet,
  transactions,
  userAiCoinBalance,
  userUsdBalanceCents,
  onFund,
  onWithdraw,
  className,
}: BotWalletCardProps) {
  const [fundOpen, setFundOpen] = useState(false);

  // No wallet — setup prompt
  if (!wallet) {
    return (
      <>
        <Card
          className={cn('card-elevated cursor-pointer group', className)}
          onClick={() => setFundOpen(true)}
        >
          <CardContent className="flex items-center justify-between p-5">
            <div className="flex items-center gap-3">
              <div className="h-10 w-10 rounded-xl bg-muted/30 flex items-center justify-center">
                <Wallet className="h-5 w-5 text-text-tertiary" />
              </div>
              <div>
                <p className="text-body font-medium">Set Up Bot Wallet</p>
                <p className="text-body-small text-text-tertiary">
                  Fund this bot to purchase compute autonomously
                </p>
              </div>
            </div>
            <Button variant="outline" size="sm" className="opacity-0 group-hover:opacity-100 transition-opacity">
              Fund
            </Button>
          </CardContent>
        </Card>
        <FundBotDialog
          open={fundOpen}
          onOpenChange={setFundOpen}
          botId={botId}
          wallet={null}
          userAiCoinBalance={userAiCoinBalance}
          userUsdBalanceCents={userUsdBalanceCents}
          onFund={onFund}
          onWithdraw={onWithdraw}
        />
      </>
    );
  }

  const targetBalance = 50;
  const pct = Math.min(wallet.aiCoinBalance / targetBalance, 1);
  const sparkData = computeDailyTotals(transactions);
  const recentTxs = transactions.slice(0, 5);

  return (
    <>
      <Card className={cn('card-elevated', className)}>
        <CardHeader className="pb-2">
          <div className="flex items-center justify-between">
            <CardTitle className="flex items-center gap-2">
              <Wallet className="h-4 w-4 text-text-tertiary" />
              Bot Wallet
            </CardTitle>
            <div className="flex items-center gap-2">
              {!wallet.enabled && <Badge variant="secondary" size="sm">Disabled</Badge>}
              <Button variant="ghost" size="sm" className="h-7 gap-1 text-xs" onClick={() => setFundOpen(true)}>
                <HandCoins className="h-3.5 w-3.5" />
                Fund
              </Button>
            </div>
          </div>
        </CardHeader>

        <CardContent className="space-y-5">
          {/* Hero: ring + balances */}
          <div className="flex items-center gap-6">
            <BudgetProgressRing
              value={pct}
              label={formatAiCoin(wallet.aiCoinBalance)}
              sublabel="AI coin"
              size={100}
              strokeWidth={7}
            />
            <div className="flex-1 space-y-3 min-w-0">
              <div>
                <p className="text-caption tracking-wider text-text-tertiary">Balance</p>
                <p className="text-heading-2 font-semibold tabular-nums tracking-tight">
                  {formatAiCoin(wallet.aiCoinBalance)} <span className="text-sm text-muted-foreground">AI</span>
                </p>
                <p className="text-body-small text-text-tertiary tabular-nums">
                  ${(wallet.usdBalanceCents / 100).toFixed(2)} USD
                </p>
              </div>

              {/* Address */}
              <div className="flex items-center gap-1.5">
                <p className="text-[10px] font-mono text-muted-foreground truncate">{wallet.displayAddress}</p>
                <Badge variant="outline" className="text-[8px] shrink-0">Chain {wallet.chainId}</Badge>
              </div>

              {/* Sparkline */}
              {sparkData.length >= 2 && (
                <div className="flex items-center gap-2">
                  <Coins className="h-3 w-3 text-text-tertiary" />
                  <Sparkline
                    data={sparkData}
                    width={80}
                    height={20}
                    color={pct > 0.5 ? 'rgb(16 185 129)' : pct > 0.2 ? 'rgb(245 158 11)' : 'rgb(239 68 68)'}
                    showArea
                  />
                  <span className="text-[10px] text-text-tertiary">30d activity</span>
                </div>
              )}
            </div>
          </div>

          {/* Recent transactions */}
          {recentTxs.length > 0 && (
            <div className="space-y-2">
              <p className="text-caption tracking-wider text-text-tertiary">Recent Transactions</p>
              <div className="space-y-1.5">
                {recentTxs.map((tx) => {
                  const meta = TX_TYPE_LABEL[tx.type];
                  return (
                    <div key={tx.id} className="flex items-center justify-between text-xs py-1">
                      <div className="flex items-center gap-2 min-w-0">
                        <span className={`size-1.5 rounded-full shrink-0 ${STATUS_DOT[tx.status]}`} />
                        <Badge variant="secondary" className={`text-[9px] shrink-0 ${meta.color}`}>
                          {meta.label}
                        </Badge>
                        <span className="truncate text-muted-foreground">{tx.description}</span>
                      </div>
                      <span className={cn('tabular-nums font-mono shrink-0 ml-2', tx.amountAiCoin >= 0 ? 'text-emerald-600' : 'text-red-500')}>
                        {tx.amountAiCoin >= 0 ? '+' : ''}{tx.amountAiCoin.toFixed(1)}
                      </span>
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <FundBotDialog
        open={fundOpen}
        onOpenChange={setFundOpen}
        botId={botId}
        wallet={wallet}
        userAiCoinBalance={userAiCoinBalance}
        userUsdBalanceCents={userUsdBalanceCents}
        onFund={onFund}
        onWithdraw={onWithdraw}
      />
    </>
  );
}
