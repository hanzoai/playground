/**
 * AutoPurchaseRuleDialog — create or edit an auto-purchase rule.
 */

import { useEffect, useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import type { AutoPurchaseRule } from '@/types/botWallet';
import type { CapacityType } from '@/types/network';

interface AutoPurchaseRuleDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  rule: AutoPurchaseRule | null;
  onSave: (rule: Partial<AutoPurchaseRule>) => Promise<void>;
  onDelete: () => Promise<void>;
}

const CAPACITY_TYPES: { value: CapacityType; label: string }[] = [
  { value: 'claude-code', label: 'Claude Code' },
  { value: 'api-key', label: 'API Key' },
  { value: 'gpu-compute', label: 'GPU Compute' },
  { value: 'inference', label: 'Inference' },
  { value: 'custom-agent', label: 'Custom Agent' },
];

export function AutoPurchaseRuleDialog({
  open,
  onOpenChange,
  rule,
  onSave,
  onDelete,
}: AutoPurchaseRuleDialogProps) {
  const [capacityType, setCapacityType] = useState<CapacityType>('claude-code');
  const [provider, setProvider] = useState('');
  const [model, setModel] = useState('');
  const [maxPrice, setMaxPrice] = useState('1.00');
  const [quantity, setQuantity] = useState('4');
  const [minBalance, setMinBalance] = useState('5');
  const [enabled, setEnabled] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  const isEdit = !!rule;

  useEffect(() => {
    if (rule) {
      setCapacityType(rule.capacityType);
      setProvider(rule.preferredProvider ?? '');
      setModel(rule.preferredModel ?? '');
      setMaxPrice((rule.maxCentsPerUnit / 100).toFixed(2));
      setQuantity(String(rule.defaultQuantity));
      setMinBalance(String(rule.minBalanceTrigger));
      setEnabled(rule.enabled);
    } else {
      setCapacityType('claude-code');
      setProvider('');
      setModel('');
      setMaxPrice('1.00');
      setQuantity('4');
      setMinBalance('5');
      setEnabled(true);
    }
    setError('');
  }, [rule, open]);

  async function handleSave() {
    const maxCents = Math.round(parseFloat(maxPrice) * 100);
    const qty = parseInt(quantity);
    const bal = parseFloat(minBalance);
    if (maxCents <= 0 || qty <= 0 || bal < 0) {
      setError('Invalid values — price, quantity, and balance must be positive.');
      return;
    }
    setSubmitting(true);
    setError('');
    try {
      await onSave({
        ...(rule ? { id: rule.id } : {}),
        capacityType,
        preferredProvider: provider.trim() || null,
        preferredModel: model.trim() || null,
        maxCentsPerUnit: maxCents,
        defaultQuantity: qty,
        minBalanceTrigger: bal,
        enabled,
      });
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save rule');
    } finally {
      setSubmitting(false);
    }
  }

  async function handleDelete() {
    setSubmitting(true);
    try {
      await onDelete();
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete rule');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit Auto-Purchase Rule' : 'New Auto-Purchase Rule'}</DialogTitle>
          <DialogDescription>
            Configure when and what this bot should automatically purchase from the marketplace.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {/* Capacity type */}
          <div>
            <label className="text-sm text-muted-foreground block mb-1">Capacity Type</label>
            <select
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
              value={capacityType}
              onChange={(e) => setCapacityType(e.target.value as CapacityType)}
            >
              {CAPACITY_TYPES.map((ct) => (
                <option key={ct.value} value={ct.value}>{ct.label}</option>
              ))}
            </select>
          </div>

          {/* Provider & Model */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-sm text-muted-foreground block mb-1">Provider (optional)</label>
              <input
                type="text"
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
                placeholder="e.g. Anthropic"
                value={provider}
                onChange={(e) => setProvider(e.target.value)}
              />
            </div>
            <div>
              <label className="text-sm text-muted-foreground block mb-1">Model (optional)</label>
              <input
                type="text"
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
                placeholder="e.g. claude-opus-4-6"
                value={model}
                onChange={(e) => setModel(e.target.value)}
              />
            </div>
          </div>

          {/* Pricing */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-sm text-muted-foreground block mb-1">Max Price ($/unit)</label>
              <input
                type="number"
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm tabular-nums"
                step="0.01"
                min="0.01"
                value={maxPrice}
                onChange={(e) => setMaxPrice(e.target.value)}
              />
            </div>
            <div>
              <label className="text-sm text-muted-foreground block mb-1">Quantity per purchase</label>
              <input
                type="number"
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm tabular-nums"
                step="1"
                min="1"
                value={quantity}
                onChange={(e) => setQuantity(e.target.value)}
              />
            </div>
          </div>

          {/* Min balance trigger */}
          <div>
            <label className="text-sm text-muted-foreground block mb-1">Min AI Coin Balance to Trigger</label>
            <input
              type="number"
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm tabular-nums"
              step="0.1"
              min="0"
              value={minBalance}
              onChange={(e) => setMinBalance(e.target.value)}
            />
            <p className="text-[10px] text-muted-foreground mt-1">
              Auto-purchase only triggers when the bot wallet has at least this much AI coin.
            </p>
          </div>

          {/* Enabled toggle */}
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              className="rounded border-border"
            />
            <span className="text-sm">Rule is active</span>
          </label>

          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>

        <DialogFooter className="flex justify-between">
          <div>
            {isEdit && (
              <Button variant="destructive" size="sm" onClick={handleDelete} disabled={submitting}>
                Delete Rule
              </Button>
            )}
          </div>
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button onClick={handleSave} disabled={submitting}>
              {submitting ? 'Saving...' : isEdit ? 'Update Rule' : 'Create Rule'}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
