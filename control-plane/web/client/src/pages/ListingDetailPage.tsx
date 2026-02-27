/**
 * ListingDetailPage — view and purchase from a single marketplace listing.
 */

import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import * as networkApi from '@/services/networkApi';
import { PurchaseDialog } from '@/components/marketplace/PurchaseDialog';
import { CapacityTypeIcon, capacityTypeLabel } from '@/components/marketplace/CapacityTypeIcon';
import { ConfidentialBadge, ConfidentialDetailSection } from '@/components/marketplace/ConfidentialBadge';
import { ResaleIndicator } from '@/components/marketplace/ResaleIndicator';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from '@/components/ui/card';
import type { MarketplaceListing } from '@/types/network';

function ratingStars(rating: number): string {
  const full = Math.floor(rating);
  const half = rating - full >= 0.5 ? '½' : '';
  return '★'.repeat(full) + half;
}

export function ListingDetailPage() {
  const { listingId } = useParams<{ listingId: string }>();
  const navigate = useNavigate();
  const [listing, setListing] = useState<MarketplaceListing | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [purchaseOpen, setPurchaseOpen] = useState(false);

  useEffect(() => {
    if (!listingId) return;
    setLoading(true);
    networkApi.getListingById(listingId)
      .then(setListing)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load listing'))
      .finally(() => setLoading(false));
  }, [listingId]);

  if (loading) {
    return <div className="text-center py-12 text-muted-foreground text-sm">Loading listing...</div>;
  }

  if (error || !listing) {
    return (
      <div className="text-center py-12">
        <p className="text-destructive text-sm">{error ?? 'Listing not found'}</p>
        <Button variant="link" size="sm" onClick={() => navigate('/marketplace')}>Back to Marketplace</Button>
      </div>
    );
  }

  const capacityPct = listing.totalCapacity > 0
    ? Math.round((listing.remainingCapacity / listing.totalCapacity) * 100)
    : 0;
  const unitLabel = listing.pricing.unit === 'hour' ? 'hr' : listing.pricing.unit === 'request' ? 'req' : '1k tok';

  return (
    <div className="space-y-6 max-w-3xl">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <button className="hover:text-foreground transition-colors" onClick={() => navigate('/marketplace')}>
          Marketplace
        </button>
        <span>/</span>
        <span className="text-foreground truncate">{listing.title}</span>
      </div>

      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-2">
          <div className="flex items-center gap-2">
            <CapacityTypeIcon type={listing.capacityType} size={20} />
            <h1 className="text-xl font-bold tracking-tight">{listing.title}</h1>
          </div>
          <div className="flex items-center gap-3 text-sm text-muted-foreground">
            <span>by {listing.sellerDisplayName}</span>
            <span className="text-amber-500">{ratingStars(listing.rating)} {listing.rating.toFixed(1)}</span>
            <span>{listing.totalOrders} orders</span>
          </div>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {listing.isResale && <ResaleIndicator parentListingId={listing.parentListingId} />}
          <Badge variant={listing.status === 'active' ? 'default' : 'secondary'} className="capitalize">
            {listing.status}
          </Badge>
        </div>
      </div>

      {/* Description */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Description</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground leading-relaxed whitespace-pre-line">{listing.description}</p>
          <div className="flex items-center gap-4 mt-4 text-xs text-muted-foreground">
            <span className="flex items-center gap-1">
              <Badge variant="outline" className="text-[10px]">{capacityTypeLabel(listing.capacityType)}</Badge>
            </span>
            {listing.confidentialCompute && (
              <ConfidentialBadge info={listing.confidentialCompute} />
            )}
            <span>{listing.provider}</span>
            <span className="font-mono">{listing.model}</span>
          </div>
        </CardContent>
      </Card>

      {/* Confidential Computing */}
      {listing.confidentialCompute && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Confidential Computing</CardTitle>
            <CardDescription>Hardware-based privacy guarantees for this listing.</CardDescription>
          </CardHeader>
          <CardContent>
            <ConfidentialDetailSection info={listing.confidentialCompute} />
          </CardContent>
        </Card>
      )}

      {/* Agent Details */}
      {listing.agentMeta && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Agent Details</CardTitle>
            <CardDescription>DID-verified custom agent information.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <p className="text-xs text-muted-foreground">Agent DID</p>
                <p className="font-mono text-xs break-all">{listing.agentMeta.agentDid}</p>
              </div>
              {listing.agentMeta.botDid && (
                <div>
                  <p className="text-xs text-muted-foreground">Bot DID</p>
                  <p className="font-mono text-xs break-all">{listing.agentMeta.botDid}</p>
                </div>
              )}
              <div>
                <p className="text-xs text-muted-foreground">Specialization</p>
                <p className="font-medium">{listing.agentMeta.specialization}</p>
              </div>
              {listing.agentMeta.successRate !== null && (
                <div>
                  <p className="text-xs text-muted-foreground">Success Rate</p>
                  <p className="font-medium tabular-nums">{(listing.agentMeta.successRate * 100).toFixed(1)}%</p>
                </div>
              )}
              {listing.agentMeta.totalRuns !== null && (
                <div>
                  <p className="text-xs text-muted-foreground">Total Runs</p>
                  <p className="font-medium tabular-nums">{listing.agentMeta.totalRuns.toLocaleString()}</p>
                </div>
              )}
            </div>
            {listing.agentMeta.capabilities.length > 0 && (
              <div>
                <p className="text-xs text-muted-foreground mb-2">Capabilities</p>
                <div className="flex flex-wrap gap-1.5">
                  {listing.agentMeta.capabilities.map((cap) => (
                    <Badge key={cap} variant="secondary" className="text-[10px]">{cap}</Badge>
                  ))}
                </div>
              </div>
            )}
            {listing.agentMeta.trainingDataDescription && (
              <div>
                <p className="text-xs text-muted-foreground mb-1">Training Data</p>
                <p className="text-sm text-muted-foreground">{listing.agentMeta.trainingDataDescription}</p>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Pricing + Capacity */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Pricing & Availability</CardTitle>
          <CardDescription>
            ${(listing.pricing.centsPerUnit / 100).toFixed(2)} per {unitLabel} · min {listing.pricing.minUnits}{listing.pricing.maxUnits ? `, max ${listing.pricing.maxUnits}` : ''} {unitLabel}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Price highlight */}
          <div className="text-center py-4">
            <p className="text-4xl font-bold tabular-nums">${(listing.pricing.centsPerUnit / 100).toFixed(2)}</p>
            <p className="text-sm text-muted-foreground mt-1">per {unitLabel}</p>
          </div>

          {/* Capacity bar */}
          <div>
            <div className="flex items-center justify-between text-sm mb-2">
              <span className="text-muted-foreground">Available capacity</span>
              <span className="font-medium tabular-nums">{listing.remainingCapacity} / {listing.totalCapacity}</span>
            </div>
            <div className="h-2 rounded-full bg-muted overflow-hidden">
              <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${capacityPct}%` }} />
            </div>
            <p className="text-xs text-muted-foreground mt-1">{capacityPct}% remaining</p>
          </div>

          {/* Purchase button */}
          {listing.status === 'active' && listing.remainingCapacity > 0 && (
            <Button className="w-full" size="lg" onClick={() => setPurchaseOpen(true)}>
              Purchase Capacity
            </Button>
          )}
          {listing.status !== 'active' && (
            <p className="text-sm text-muted-foreground text-center py-2">This listing is currently unavailable.</p>
          )}
        </CardContent>
      </Card>

      {/* How it works */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">How It Works</CardTitle>
        </CardHeader>
        <CardContent>
          <ol className="list-decimal list-inside text-sm text-muted-foreground space-y-2">
            <li>Choose the quantity of {unitLabel}s you need and confirm purchase.</li>
            <li>A secure proxy endpoint is provisioned — all requests are routed through the seller's capacity.</li>
            <li>For file-heavy workloads, an S3 transfer bucket is created for data exchange.</li>
            <li>Your data is never stored. All traffic is encrypted end-to-end.</li>
            <li>When done, you can resell any unused capacity at your own price.</li>
          </ol>
        </CardContent>
      </Card>

      {/* Purchase dialog */}
      <PurchaseDialog listing={listing} open={purchaseOpen} onOpenChange={setPurchaseOpen} />
    </div>
  );
}
