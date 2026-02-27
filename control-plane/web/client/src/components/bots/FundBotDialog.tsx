/**
 * FundBotDialog — fund or withdraw from a bot's wallet.
 */

import { useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import type { BotWallet, WalletFundingSource } from '@/types/botWallet';

interface FundBotDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  botId: string;
  wallet: BotWallet | null;
  userAiCoinBalance: number;
  userUsdBalanceCents: number;
  onFund: (source: WalletFundingSource, aiCoin: number, usdCents: number) => Promise<void>;
  onWithdraw: (aiCoin: number, usdCents: number) => Promise<void>;
}

type Mode = 'fund' | 'withdraw';
type Currency = 'ai_coin' | 'usd';

export function FundBotDialog({
  open,
  onOpenChange,
  wallet,
  userAiCoinBalance,
  userUsdBalanceCents,
  onFund,
  onWithdraw,
}: FundBotDialogProps) {
  const [mode, setMode] = useState<Mode>('fund');
  const [currency, setCurrency] = useState<Currency>('ai_coin');
  const [amount, setAmount] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const numAmount = parseFloat(amount) || 0;
  const maxFund = currency === 'ai_coin' ? userAiCoinBalance : userUsdBalanceCents / 100;
  const maxWithdraw = currency === 'ai_coin'
    ? (wallet?.aiCoinBalance ?? 0)
    : (wallet?.usdBalanceCents ?? 0) / 100;
  const max = mode === 'fund' ? maxFund : maxWithdraw;
  const valid = numAmount > 0 && numAmount <= max;

  async function handleSubmit() {
    if (!valid) return;
    setSubmitting(true);
    setError('');
    setSuccess('');
    try {
      if (mode === 'fund') {
        const source: WalletFundingSource = currency === 'ai_coin' ? 'user_ai_coin' : 'user_usd';
        const aiCoin = currency === 'ai_coin' ? numAmount : 0;
        const usdCents = currency === 'usd' ? Math.round(numAmount * 100) : 0;
        await onFund(source, aiCoin, usdCents);
        setSuccess(`Funded ${currency === 'ai_coin' ? `${numAmount} AI coin` : `$${numAmount.toFixed(2)}`} to bot wallet`);
      } else {
        const aiCoin = currency === 'ai_coin' ? numAmount : 0;
        const usdCents = currency === 'usd' ? Math.round(numAmount * 100) : 0;
        await onWithdraw(aiCoin, usdCents);
        setSuccess(`Withdrew ${currency === 'ai_coin' ? `${numAmount} AI coin` : `$${numAmount.toFixed(2)}`} from bot wallet`);
      }
      setAmount('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Transaction failed');
    } finally {
      setSubmitting(false);
    }
  }

  function handleClose() {
    setError('');
    setSuccess('');
    setAmount('');
    onOpenChange(false);
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{mode === 'fund' ? 'Fund Bot Wallet' : 'Withdraw from Bot Wallet'}</DialogTitle>
          <DialogDescription>
            {mode === 'fund'
              ? 'Transfer AI coin or USD from your balance to this bot.'
              : 'Transfer funds from this bot back to your balance.'}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {/* Mode toggle */}
          <div className="flex gap-2">
            <Button
              variant={mode === 'fund' ? 'default' : 'outline'}
              size="sm"
              className="flex-1"
              onClick={() => { setMode('fund'); setAmount(''); setError(''); setSuccess(''); }}
            >
              Fund
            </Button>
            <Button
              variant={mode === 'withdraw' ? 'default' : 'outline'}
              size="sm"
              className="flex-1"
              onClick={() => { setMode('withdraw'); setAmount(''); setError(''); setSuccess(''); }}
            >
              Withdraw
            </Button>
          </div>

          {/* Currency toggle */}
          <div className="flex gap-2">
            <Button
              variant={currency === 'ai_coin' ? 'default' : 'outline'}
              size="sm"
              className="flex-1"
              onClick={() => { setCurrency('ai_coin'); setAmount(''); }}
            >
              AI Coin
            </Button>
            <Button
              variant={currency === 'usd' ? 'default' : 'outline'}
              size="sm"
              className="flex-1"
              onClick={() => { setCurrency('usd'); setAmount(''); }}
            >
              USD
            </Button>
          </div>

          {/* Balances */}
          <div className="flex justify-between text-xs text-muted-foreground rounded-md bg-muted p-3">
            <div>
              <p className="font-medium text-foreground">Your Balance</p>
              <p className="tabular-nums">
                {userAiCoinBalance.toFixed(1)} AI · ${(userUsdBalanceCents / 100).toFixed(2)}
              </p>
            </div>
            <div className="text-right">
              <p className="font-medium text-foreground">Bot Wallet</p>
              <p className="tabular-nums">
                {wallet?.aiCoinBalance.toFixed(1) ?? '0.0'} AI · ${((wallet?.usdBalanceCents ?? 0) / 100).toFixed(2)}
              </p>
            </div>
          </div>

          {/* Amount */}
          <div>
            <label className="text-sm text-muted-foreground block mb-1">
              Amount ({currency === 'ai_coin' ? 'AI coin' : 'USD'})
              <span className="text-xs ml-2">max: {max.toFixed(currency === 'ai_coin' ? 1 : 2)}</span>
            </label>
            <input
              type="number"
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm tabular-nums"
              placeholder="0.00"
              step={currency === 'ai_coin' ? '0.1' : '0.01'}
              min="0"
              max={max}
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
            />
          </div>

          {/* Quick amounts */}
          <div className="flex gap-1.5">
            {[0.25, 0.5, 0.75, 1.0].map((frac) => (
              <Badge
                key={frac}
                variant="outline"
                className="cursor-pointer text-[10px] hover:bg-primary/10"
                onClick={() => setAmount((max * frac).toFixed(currency === 'ai_coin' ? 1 : 2))}
              >
                {Math.round(frac * 100)}%
              </Badge>
            ))}
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}
          {success && <p className="text-sm text-emerald-600">{success}</p>}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={handleClose}>Cancel</Button>
          <Button onClick={handleSubmit} disabled={!valid || submitting}>
            {submitting ? 'Processing...' : mode === 'fund' ? 'Fund Bot' : 'Withdraw'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
