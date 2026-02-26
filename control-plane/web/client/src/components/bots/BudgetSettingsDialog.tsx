import { useCallback, useEffect, useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import {
  CurrencyDollar,
  Trash,
  Save,
} from '@/components/ui/icon-bridge';
import type { BotBudget } from '@/services/budgetApi';

interface BudgetSettingsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  botId: string;
  budget: BotBudget | null;
  onSave: (updates: Partial<BotBudget>) => Promise<void>;
  onDelete: () => Promise<void>;
}

export function BudgetSettingsDialog({
  open,
  onOpenChange,
  botId,
  budget,
  onSave,
  onDelete,
}: BudgetSettingsDialogProps) {
  const [monthlyLimit, setMonthlyLimit] = useState('100');
  const [dailyLimit, setDailyLimit] = useState('10');
  const [alertThreshold, setAlertThreshold] = useState('80');
  const [enabled, setEnabled] = useState(true);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  // Sync form state when budget changes or dialog opens
  useEffect(() => {
    if (open) {
      if (budget) {
        setMonthlyLimit(String(budget.monthly_limit_usd));
        setDailyLimit(String(budget.daily_limit_usd));
        setAlertThreshold(String(budget.alert_threshold * 100));
        setEnabled(budget.enabled);
      } else {
        setMonthlyLimit('100');
        setDailyLimit('10');
        setAlertThreshold('80');
        setEnabled(true);
      }
      setConfirmDelete(false);
    }
  }, [open, budget]);

  const handleSave = useCallback(async () => {
    const monthly = parseFloat(monthlyLimit);
    const daily = parseFloat(dailyLimit);
    const alert = parseFloat(alertThreshold) / 100;

    if (isNaN(monthly) || monthly <= 0) return;
    if (isNaN(daily) || daily <= 0) return;
    if (isNaN(alert) || alert <= 0 || alert > 1) return;

    setSaving(true);
    try {
      await onSave({
        bot_id: botId,
        monthly_limit_usd: monthly,
        daily_limit_usd: daily,
        alert_threshold: alert,
        enabled,
      });
      onOpenChange(false);
    } catch {
      // Error handling delegated to parent
    } finally {
      setSaving(false);
    }
  }, [botId, monthlyLimit, dailyLimit, alertThreshold, enabled, onSave, onOpenChange]);

  const handleDelete = useCallback(async () => {
    if (!confirmDelete) {
      setConfirmDelete(true);
      return;
    }
    setDeleting(true);
    try {
      await onDelete();
      onOpenChange(false);
    } finally {
      setDeleting(false);
      setConfirmDelete(false);
    }
  }, [confirmDelete, onDelete, onOpenChange]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <CurrencyDollar className="h-5 w-5" />
            {budget ? 'Budget Settings' : 'Set Up Budget'}
          </DialogTitle>
          <DialogDescription>
            Control how much this bot can spend. Executions are blocked when limits are reached.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-5 py-2">
          {/* Enabled toggle */}
          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm font-medium">Budget enforcement</Label>
              <p className="text-[12px] text-text-tertiary mt-0.5">
                When disabled, spending is tracked but not blocked
              </p>
            </div>
            <Switch checked={enabled} onCheckedChange={setEnabled} />
          </div>

          {/* Monthly limit */}
          <div className="space-y-2">
            <Label htmlFor="monthly-limit" className="text-sm font-medium">
              Monthly limit (USD)
            </Label>
            <div className="relative">
              <span className="absolute left-3 top-1/2 -translate-y-1/2 text-text-tertiary text-sm">$</span>
              <Input
                id="monthly-limit"
                type="number"
                min="0"
                step="1"
                value={monthlyLimit}
                onChange={(e) => setMonthlyLimit(e.target.value)}
                className="pl-7 font-mono tabular-nums"
                placeholder="100.00"
              />
            </div>
          </div>

          {/* Daily limit */}
          <div className="space-y-2">
            <Label htmlFor="daily-limit" className="text-sm font-medium">
              Daily limit (USD)
            </Label>
            <div className="relative">
              <span className="absolute left-3 top-1/2 -translate-y-1/2 text-text-tertiary text-sm">$</span>
              <Input
                id="daily-limit"
                type="number"
                min="0"
                step="0.5"
                value={dailyLimit}
                onChange={(e) => setDailyLimit(e.target.value)}
                className="pl-7 font-mono tabular-nums"
                placeholder="10.00"
              />
            </div>
          </div>

          {/* Alert threshold */}
          <div className="space-y-2">
            <Label htmlFor="alert-threshold" className="text-sm font-medium">
              Alert at (% of limit)
            </Label>
            <div className="relative">
              <Input
                id="alert-threshold"
                type="number"
                min="1"
                max="100"
                step="5"
                value={alertThreshold}
                onChange={(e) => setAlertThreshold(e.target.value)}
                className="pr-8 font-mono tabular-nums"
                placeholder="80"
              />
              <span className="absolute right-3 top-1/2 -translate-y-1/2 text-text-tertiary text-sm">%</span>
            </div>
            <p className="text-[11px] text-text-tertiary">
              A notification fires when spend crosses this percentage of the monthly limit
            </p>
          </div>
        </div>

        <DialogFooter className="gap-2 sm:gap-0">
          {budget && (
            <Button
              variant="ghost"
              size="sm"
              onClick={handleDelete}
              disabled={deleting}
              className="text-destructive hover:text-destructive mr-auto"
            >
              <Trash className="h-3.5 w-3.5 mr-1.5" />
              {confirmDelete ? 'Confirm delete' : 'Remove budget'}
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button size="sm" onClick={handleSave} disabled={saving}>
            <Save className="h-3.5 w-3.5 mr-1.5" />
            {saving ? 'Saving...' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
