/**
 * EarningsChart — 30-day area chart of AI coin earnings.
 */

import { useMemo } from 'react';
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from 'recharts';
import type { EarningsRecord } from '@/types/network';

interface EarningsChartProps {
  records: EarningsRecord[];
}

export function EarningsChart({ records }: EarningsChartProps) {
  const data = useMemo(() => {
    const byDay = new Map<string, number>();
    for (const r of records) {
      const day = r.timestamp.slice(0, 10);
      byDay.set(day, (byDay.get(day) ?? 0) + r.amount);
    }
    return Array.from(byDay.entries())
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([date, amount]) => ({
        date: new Date(date).toLocaleDateString('en-US', { month: 'short', day: 'numeric' }),
        amount: +amount.toFixed(2),
      }));
  }, [records]);

  if (data.length === 0) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-muted-foreground">
        No earnings data yet. Start sharing capacity to earn AI coin.
      </div>
    );
  }

  return (
    <ResponsiveContainer width="100%" height={240}>
      <AreaChart data={data} margin={{ top: 4, right: 4, bottom: 0, left: -20 }}>
        <defs>
          <linearGradient id="aiCoinGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor="hsl(var(--primary))" stopOpacity={0.3} />
            <stop offset="95%" stopColor="hsl(var(--primary))" stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" className="stroke-border/30" />
        <XAxis
          dataKey="date"
          tick={{ fontSize: 11, fill: 'hsl(var(--muted-foreground))' }}
          tickLine={false}
          axisLine={false}
        />
        <YAxis
          tick={{ fontSize: 11, fill: 'hsl(var(--muted-foreground))' }}
          tickLine={false}
          axisLine={false}
          tickFormatter={(v: number) => `${v}`}
        />
        <Tooltip
          contentStyle={{
            backgroundColor: 'hsl(var(--popover))',
            borderColor: 'hsl(var(--border))',
            borderRadius: 8,
            fontSize: 12,
          }}
          formatter={(value: number) => [`${value.toFixed(2)} AI`, 'Earned']}
        />
        <Area
          type="monotone"
          dataKey="amount"
          stroke="hsl(var(--primary))"
          fill="url(#aiCoinGrad)"
          strokeWidth={2}
        />
      </AreaChart>
    </ResponsiveContainer>
  );
}
