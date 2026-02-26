import { useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Sparkline } from '@/components/ui/Sparkline';
import { BudgetProgressRing } from './BudgetProgressRing';
import { BudgetSettingsDialog } from './BudgetSettingsDialog';
import {
  CurrencyDollar,
  Settings,
  Analytics,
  Warning,
} from '@/components/ui/icon-bridge';
import { cn } from '@/lib/utils';
import type { BotBudget, BudgetStatus, BotSpendRecord } from '@/services/budgetApi';

interface BotBudgetCardProps {
  botId: string;
  budget: BotBudget | null;
  status: BudgetStatus | null;
  spendHistory: BotSpendRecord[];
  onSave: (updates: Partial<BotBudget>) => Promise<void>;
  onDelete: () => Promise<void>;
  className?: string;
}

function formatUSD(amount: number): string {
  if (amount >= 1000) return `$${(amount / 1000).toFixed(1)}k`;
  if (amount >= 100) return `$${amount.toFixed(0)}`;
  if (amount >= 1) return `$${amount.toFixed(2)}`;
  return `$${amount.toFixed(4)}`;
}

function SpendMetric({
  label,
  spent,
  limit,
  icon: Icon,
}: {
  label: string;
  spent: number;
  limit: number;
  icon: typeof CurrencyDollar;
}) {
  const pct = limit > 0 ? spent / limit : 0;
  const barColor =
    pct < 0.6
      ? 'bg-emerald-500'
      : pct < 0.85
        ? 'bg-amber-500'
        : 'bg-red-500';

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-1.5">
          <Icon className="h-3 w-3 text-text-tertiary" />
          <span className="text-caption tracking-wider">{label}</span>
        </div>
        <span className="text-body-small font-mono tabular-nums text-text-secondary">
          {formatUSD(spent)} / {formatUSD(limit)}
        </span>
      </div>
      <div className="h-1.5 w-full rounded-full bg-muted/30 overflow-hidden">
        <div
          className={cn('h-full rounded-full transition-all duration-500 ease-out', barColor)}
          style={{ width: `${Math.min(pct * 100, 100)}%` }}
        />
      </div>
    </div>
  );
}

export function BotBudgetCard({
  botId,
  budget,
  status,
  spendHistory,
  onSave,
  onDelete,
  className,
}: BotBudgetCardProps) {
  const [settingsOpen, setSettingsOpen] = useState(false);

  // Compute sparkline data from spend history (daily aggregation)
  const dailySpend = computeDailySpend(spendHistory);

  // No budget configured — show setup prompt
  if (!budget) {
    return (
      <>
        <Card
          className={cn(
            'card-elevated cursor-pointer group',
            className,
          )}
          onClick={() => setSettingsOpen(true)}
        >
          <CardContent className="flex items-center justify-between p-5">
            <div className="flex items-center gap-3">
              <div className="h-10 w-10 rounded-xl bg-muted/30 flex items-center justify-center">
                <CurrencyDollar className="h-5 w-5 text-text-tertiary" />
              </div>
              <div>
                <p className="text-body font-medium">Set Up Budget</p>
                <p className="text-body-small text-text-tertiary">
                  Control spending limits for this bot
                </p>
              </div>
            </div>
            <Button variant="outline" size="sm" className="opacity-0 group-hover:opacity-100 transition-opacity">
              Configure
            </Button>
          </CardContent>
        </Card>
        <BudgetSettingsDialog
          open={settingsOpen}
          onOpenChange={setSettingsOpen}
          botId={botId}
          budget={null}
          onSave={onSave}
          onDelete={onDelete}
        />
      </>
    );
  }

  const monthlyPct = budget.monthly_limit_usd > 0
    ? budget.current_month_usd / budget.monthly_limit_usd
    : 0;
  const dailyPct = budget.daily_limit_usd > 0
    ? budget.current_day_usd / budget.daily_limit_usd
    : 0;

  const isAlert = status?.alert_triggered ?? false;
  const isBlocked = status ? !status.allowed : false;

  return (
    <>
      <Card className={cn('card-elevated', className)}>
        <CardHeader className="pb-2">
          <div className="flex items-center justify-between">
            <CardTitle className="flex items-center gap-2">
              <CurrencyDollar className="h-4 w-4 text-text-tertiary" />
              Budget
            </CardTitle>
            <div className="flex items-center gap-2">
              {isBlocked && (
                <Badge variant="destructive" size="sm">
                  <Warning className="h-3 w-3" />
                  Blocked
                </Badge>
              )}
              {isAlert && !isBlocked && (
                <Badge variant="pending" size="sm">
                  Alert
                </Badge>
              )}
              {!budget.enabled && (
                <Badge variant="secondary" size="sm">
                  Paused
                </Badge>
              )}
              <Button
                variant="ghost"
                size="sm"
                className="h-7 w-7 p-0"
                onClick={() => setSettingsOpen(true)}
              >
                <Settings className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        </CardHeader>

        <CardContent className="space-y-5">
          {/* Hero section: Ring + Spend summary */}
          <div className="flex items-center gap-6">
            {/* Monthly progress ring */}
            <BudgetProgressRing
              value={monthlyPct}
              label={`${Math.round(monthlyPct * 100)}%`}
              sublabel="monthly"
              size={100}
              strokeWidth={7}
            />

            {/* Right side: key numbers */}
            <div className="flex-1 space-y-3 min-w-0">
              <div>
                <p className="text-caption tracking-wider text-text-tertiary">Spent this month</p>
                <p className="text-heading-2 font-semibold tabular-nums tracking-tight">
                  {formatUSD(budget.current_month_usd)}
                </p>
                <p className="text-body-small text-text-tertiary">
                  of {formatUSD(budget.monthly_limit_usd)} limit
                </p>
              </div>

              {/* Mini sparkline */}
              {dailySpend.length >= 2 && (
                <div className="flex items-center gap-2">
                  <Analytics className="h-3 w-3 text-text-tertiary" />
                  <Sparkline
                    data={dailySpend}
                    width={80}
                    height={20}
                    color={monthlyPct < 0.6 ? 'rgb(16 185 129)' : monthlyPct < 0.85 ? 'rgb(245 158 11)' : 'rgb(239 68 68)'}
                    showArea
                  />
                  <span className="text-[10px] text-text-tertiary">30d trend</span>
                </div>
              )}
            </div>
          </div>

          {/* Spend bars */}
          <div className="space-y-3">
            <SpendMetric
              label="Monthly"
              spent={budget.current_month_usd}
              limit={budget.monthly_limit_usd}
              icon={CurrencyDollar}
            />
            <SpendMetric
              label="Today"
              spent={budget.current_day_usd}
              limit={budget.daily_limit_usd}
              icon={CurrencyDollar}
            />
          </div>

          {/* Usage stats row */}
          {status && (
            <div className="grid grid-cols-3 gap-3 pt-1">
              <div className="text-center">
                <p className="text-heading-3 tabular-nums">
                  {formatUSD(status.monthly_spent_usd)}
                </p>
                <p className="text-[10px] text-text-tertiary">Month spend</p>
              </div>
              <div className="text-center">
                <p className="text-heading-3 tabular-nums">
                  {formatUSD(status.daily_spent_usd)}
                </p>
                <p className="text-[10px] text-text-tertiary">Day spend</p>
              </div>
              <div className="text-center">
                <p className="text-heading-3 tabular-nums">
                  {formatUSD(
                    budget.monthly_limit_usd - budget.current_month_usd > 0
                      ? budget.monthly_limit_usd - budget.current_month_usd
                      : 0,
                  )}
                </p>
                <p className="text-[10px] text-text-tertiary">Remaining</p>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <BudgetSettingsDialog
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
        botId={botId}
        budget={budget}
        onSave={onSave}
        onDelete={onDelete}
      />
    </>
  );
}

/** Aggregate spend records into daily totals for sparkline */
function computeDailySpend(records: BotSpendRecord[]): number[] {
  if (records.length === 0) return [];

  const byDay = new Map<string, number>();
  for (const r of records) {
    const day = r.recorded_at.slice(0, 10);
    byDay.set(day, (byDay.get(day) ?? 0) + r.amount_usd);
  }

  // Generate last 30 days
  const days: number[] = [];
  const now = new Date();
  for (let i = 29; i >= 0; i--) {
    const d = new Date(now);
    d.setDate(d.getDate() - i);
    const key = d.toISOString().slice(0, 10);
    days.push(byDay.get(key) ?? 0);
  }
  return days;
}
