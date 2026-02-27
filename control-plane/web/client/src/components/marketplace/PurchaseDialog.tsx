/**
 * PurchaseDialog — buy capacity from a marketplace listing.
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
import { CapacityTypeIcon, capacityTypeLabel } from './CapacityTypeIcon';
import { useNetworkStore } from '@/stores/networkStore';
import type { MarketplaceListing, MarketplaceOrder } from '@/types/network';

interface Props {
  listing: MarketplaceListing;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function PurchaseDialog({ listing, open, onOpenChange }: Props) {
  const purchaseCapacity = useNetworkStore((s) => s.purchaseCapacity);
  const [quantity, setQuantity] = useState(listing.pricing.minUnits);
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<MarketplaceOrder | null>(null);
  const [error, setError] = useState<string | null>(null);

  const unitLabel = listing.pricing.unit === 'hour' ? 'hours' : listing.pricing.unit === 'request' ? 'requests' : '1k tokens';
  const totalCents = quantity * listing.pricing.centsPerUnit;
  const maxQty = listing.pricing.maxUnits ?? listing.remainingCapacity;
  const validQty = quantity >= listing.pricing.minUnits && quantity <= maxQty && quantity <= listing.remainingCapacity;

  const handlePurchase = async () => {
    setSubmitting(true);
    setError(null);
    try {
      const order = await purchaseCapacity({ listingId: listing.id, quantity });
      setResult(order);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Purchase failed');
    } finally {
      setSubmitting(false);
    }
  };

  const handleClose = () => {
    setResult(null);
    setError(null);
    setQuantity(listing.pricing.minUnits);
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-md">
        {result ? (
          <>
            <DialogHeader>
              <DialogTitle>Order Placed</DialogTitle>
              <DialogDescription>Your order is being activated. You will receive a proxy endpoint shortly.</DialogDescription>
            </DialogHeader>
            <div className="space-y-3 py-2">
              <div className="rounded-md bg-muted p-3">
                <p className="text-xs text-muted-foreground mb-1">Order ID</p>
                <code className="text-sm font-mono">{result.id}</code>
              </div>
              <p className="text-sm">
                <span className="font-medium">{result.quantity} {unitLabel}</span> of {result.model}
                {' '}for <span className="font-medium">${(result.totalCostCents / 100).toFixed(2)}</span>
              </p>
              {result.proxyEndpoint && (
                <div className="rounded-md bg-muted p-3">
                  <p className="text-xs text-muted-foreground mb-1">Proxy Endpoint</p>
                  <code className="text-xs font-mono break-all">{result.proxyEndpoint}</code>
                </div>
              )}
              {result.s3TransferBucket && (
                <div className="rounded-md bg-muted p-3">
                  <p className="text-xs text-muted-foreground mb-1">S3 Transfer Bucket</p>
                  <code className="text-xs font-mono">{result.s3TransferBucket}</code>
                </div>
              )}
            </div>
            <DialogFooter>
              <Button onClick={handleClose}>Done</Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>Purchase Capacity</DialogTitle>
              <DialogDescription>Buy AI capacity from this seller.</DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-2">
              {/* Listing summary */}
              <div className="flex items-center gap-2">
                <CapacityTypeIcon type={listing.capacityType} size={16} />
                <div className="min-w-0">
                  <p className="text-sm font-medium truncate">{listing.title}</p>
                  <p className="text-xs text-muted-foreground">{listing.provider} · {listing.model}</p>
                </div>
              </div>

              {/* Price */}
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Price</span>
                <Badge variant="secondary" className="font-mono">
                  ${(listing.pricing.centsPerUnit / 100).toFixed(2)}/{listing.pricing.unit === 'hour' ? 'hr' : listing.pricing.unit}
                </Badge>
              </div>

              {/* Quantity */}
              <div>
                <label className="text-sm text-muted-foreground block mb-1">
                  Quantity ({unitLabel}) — min {listing.pricing.minUnits}, max {maxQty}
                </label>
                <input
                  type="number"
                  className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm tabular-nums"
                  value={quantity}
                  min={listing.pricing.minUnits}
                  max={maxQty}
                  onChange={(e) => setQuantity(Math.max(0, parseInt(e.target.value) || 0))}
                />
              </div>

              {/* Total */}
              <div className="flex items-center justify-between rounded-md bg-muted p-3">
                <span className="text-sm font-medium">Total Cost</span>
                <span className="text-lg font-bold tabular-nums">${(totalCents / 100).toFixed(2)}</span>
              </div>

              {/* Capacity type info */}
              <p className="text-xs text-muted-foreground">
                {capacityTypeLabel(listing.capacityType)} from {listing.sellerDisplayName}.
                {' '}Requests will be routed via secure proxy. Your data is never stored.
              </p>

              {error && (
                <p className="text-sm text-destructive">{error}</p>
              )}
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={handleClose}>Cancel</Button>
              <Button onClick={handlePurchase} disabled={!validQty || submitting}>
                {submitting ? 'Processing...' : `Buy for $${(totalCents / 100).toFixed(2)}`}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
