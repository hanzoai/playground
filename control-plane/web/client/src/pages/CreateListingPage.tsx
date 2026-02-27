/**
 * CreateListingPage — create a new capacity listing or resale.
 */

import { useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useNetworkStore } from '@/stores/networkStore';
import { CapacityTypeIcon, capacityTypeLabel } from '@/components/marketplace/CapacityTypeIcon';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from '@/components/ui/card';
import type { CapacityType, PricingUnit, CreateListingParams } from '@/types/network';

const CAPACITY_TYPES: CapacityType[] = ['claude-code', 'api-key', 'gpu-compute', 'inference'];
const PRICING_UNITS: { value: PricingUnit; label: string }[] = [
  { value: 'hour', label: 'Per Hour' },
  { value: 'request', label: 'Per Request' },
  { value: 'token_1k', label: 'Per 1K Tokens' },
];

export function CreateListingPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const createListing = useNetworkStore((s) => s.createListing);
  const createResale = useNetworkStore((s) => s.createResale);

  const sourceOrderId = searchParams.get('source_order');
  const presetType = searchParams.get('type') as CapacityType | null;
  const presetModel = searchParams.get('model');
  const isResale = !!sourceOrderId;

  const [capacityType, setCapacityType] = useState<CapacityType>(presetType ?? 'claude-code');
  const [title, setTitle] = useState(isResale ? `[RESOLD] ${presetModel ?? 'AI Capacity'}` : '');
  const [description, setDescription] = useState('');
  const [provider, setProvider] = useState(capacityType === 'claude-code' ? 'Anthropic' : '');
  const [model, setModel] = useState(presetModel ?? '');
  const [centsPerUnit, setCentsPerUnit] = useState(100);
  const [pricingUnit, setPricingUnit] = useState<PricingUnit>('hour');
  const [minUnits, setMinUnits] = useState(1);
  const [maxUnits, setMaxUnits] = useState(24);
  const [totalCapacity, setTotalCapacity] = useState(100);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const unitLabel = pricingUnit === 'hour' ? 'hr' : pricingUnit === 'request' ? 'req' : '1k tok';

  const handleSubmit = async () => {
    if (!title.trim() || !provider.trim() || !model.trim()) {
      setError('Please fill in all required fields.');
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const params: CreateListingParams = {
        capacityType,
        title: title.trim(),
        description: description.trim(),
        provider: provider.trim(),
        model: model.trim(),
        pricing: { centsPerUnit, unit: pricingUnit, minUnits, maxUnits: maxUnits > 0 ? maxUnits : null },
        totalCapacity,
        expiresAt: null,
        sourceOrderId,
      };
      if (isResale && sourceOrderId) {
        await createResale(sourceOrderId, params);
      } else {
        await createListing(params);
      }
      navigate('/marketplace/seller');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create listing');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">
          {isResale ? 'Resell Capacity' : 'Sell AI Capacity'}
        </h1>
        <p className="text-sm text-muted-foreground mt-1">
          {isResale
            ? 'List your unused purchased capacity for resale at your own price.'
            : 'List your AI capacity on the marketplace for others to purchase.'}
        </p>
      </div>

      {/* Capacity Type */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Capacity Type</CardTitle>
          <CardDescription>What kind of AI access are you offering?</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            {CAPACITY_TYPES.map((ct) => (
              <button
                key={ct}
                className={`flex items-center gap-2 rounded-lg border p-3 text-sm transition-colors ${
                  capacityType === ct ? 'border-primary bg-primary/5 font-medium' : 'border-border hover:border-primary/30'
                }`}
                onClick={() => {
                  setCapacityType(ct);
                  if (ct === 'claude-code') setProvider('Anthropic');
                  else if (ct === 'gpu-compute') setProvider('NVIDIA');
                  else setProvider('');
                }}
              >
                <CapacityTypeIcon type={ct} size={16} />
                <span className="text-xs">{capacityTypeLabel(ct)}</span>
              </button>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Details */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Listing Details</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <label className="text-sm text-muted-foreground block mb-1">Title *</label>
            <input
              type="text"
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
              placeholder="e.g. Claude Code — Opus tier, 24/7 availability"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
          </div>
          <div>
            <label className="text-sm text-muted-foreground block mb-1">Description</label>
            <textarea
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm min-h-[80px] resize-y"
              placeholder="Describe your offering, availability, and any special features..."
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="text-sm text-muted-foreground block mb-1">Provider *</label>
              <input
                type="text"
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
                placeholder="e.g. Anthropic"
                value={provider}
                onChange={(e) => setProvider(e.target.value)}
              />
            </div>
            <div>
              <label className="text-sm text-muted-foreground block mb-1">Model *</label>
              <input
                type="text"
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
                placeholder="e.g. claude-opus-4-6"
                value={model}
                onChange={(e) => setModel(e.target.value)}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Pricing */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Pricing</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="text-sm text-muted-foreground block mb-1">Price (USD cents per unit)</label>
              <input
                type="number"
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm tabular-nums"
                value={centsPerUnit}
                min={1}
                onChange={(e) => setCentsPerUnit(Math.max(1, parseInt(e.target.value) || 1))}
              />
              <p className="text-xs text-muted-foreground mt-1">${(centsPerUnit / 100).toFixed(2)} per {unitLabel}</p>
            </div>
            <div>
              <label className="text-sm text-muted-foreground block mb-1">Pricing Unit</label>
              <select
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
                value={pricingUnit}
                onChange={(e) => setPricingUnit(e.target.value as PricingUnit)}
              >
                {PRICING_UNITS.map((pu) => (
                  <option key={pu.value} value={pu.value}>{pu.label}</option>
                ))}
              </select>
            </div>
          </div>
          <div className="grid grid-cols-3 gap-4">
            <div>
              <label className="text-sm text-muted-foreground block mb-1">Min Purchase</label>
              <input
                type="number"
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm tabular-nums"
                value={minUnits}
                min={1}
                onChange={(e) => setMinUnits(Math.max(1, parseInt(e.target.value) || 1))}
              />
            </div>
            <div>
              <label className="text-sm text-muted-foreground block mb-1">Max Purchase (0=unlimited)</label>
              <input
                type="number"
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm tabular-nums"
                value={maxUnits}
                min={0}
                onChange={(e) => setMaxUnits(Math.max(0, parseInt(e.target.value) || 0))}
              />
            </div>
            <div>
              <label className="text-sm text-muted-foreground block mb-1">Total Capacity</label>
              <input
                type="number"
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm tabular-nums"
                value={totalCapacity}
                min={1}
                onChange={(e) => setTotalCapacity(Math.max(1, parseInt(e.target.value) || 1))}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Preview */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Preview</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="rounded-lg border border-border p-4 space-y-2">
            <div className="flex items-center gap-2">
              <CapacityTypeIcon type={capacityType} size={16} />
              <Badge variant="outline" className="text-[10px]">{capacityTypeLabel(capacityType)}</Badge>
              {isResale && <Badge variant="outline" className="text-[10px]">Resold</Badge>}
            </div>
            <h3 className="text-sm font-semibold">{title || 'Listing title...'}</h3>
            <p className="text-xs text-muted-foreground">{provider || '—'} · {model || '—'}</p>
            <p className="text-lg font-bold tabular-nums">${(centsPerUnit / 100).toFixed(2)}/{unitLabel}</p>
            <p className="text-xs text-muted-foreground">{totalCapacity} {unitLabel}s available</p>
          </div>
        </CardContent>
      </Card>

      {error && (
        <p className="text-sm text-destructive">{error}</p>
      )}

      {/* Submit */}
      <div className="flex items-center gap-3">
        <Button variant="outline" onClick={() => navigate(-1)}>Cancel</Button>
        <Button onClick={handleSubmit} disabled={submitting}>
          {submitting ? 'Creating...' : isResale ? 'Create Resale Listing' : 'Create Listing'}
        </Button>
      </div>
    </div>
  );
}
