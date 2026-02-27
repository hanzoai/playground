/**
 * ListingCard — marketplace listing grid card.
 */

import { useNavigate } from 'react-router-dom';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { CapacityTypeIcon, capacityTypeLabel } from './CapacityTypeIcon';
import { ResaleIndicator } from './ResaleIndicator';
import type { MarketplaceListing } from '@/types/network';

function formatPrice(cents: number, unit: string): string {
  const dollars = (cents / 100).toFixed(2);
  const unitLabel = unit === 'hour' ? 'hr' : unit === 'request' ? 'req' : '1k tok';
  return `$${dollars}/${unitLabel}`;
}

function ratingStars(rating: number): string {
  const full = Math.floor(rating);
  const half = rating - full >= 0.5 ? '½' : '';
  return '★'.repeat(full) + half;
}

interface Props {
  listing: MarketplaceListing;
}

export function ListingCard({ listing }: Props) {
  const navigate = useNavigate();
  const capacityPct = listing.totalCapacity > 0
    ? Math.round((listing.remainingCapacity / listing.totalCapacity) * 100)
    : 0;

  return (
    <Card
      className="cursor-pointer transition-all hover:border-primary/50 hover:shadow-md"
      onClick={() => navigate(`/marketplace/listing/${listing.id}`)}
    >
      <CardContent className="p-4 space-y-3">
        {/* Type + Status */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-1.5">
            <CapacityTypeIcon type={listing.capacityType} size={14} />
            <span className="text-[10px] text-muted-foreground">{capacityTypeLabel(listing.capacityType)}</span>
          </div>
          <div className="flex items-center gap-1.5">
            {listing.isResale && <ResaleIndicator parentListingId={listing.parentListingId} />}
            <Badge
              variant={listing.status === 'active' ? 'default' : 'secondary'}
              className="text-[10px]"
            >
              {listing.status}
            </Badge>
          </div>
        </div>

        {/* Title */}
        <h3 className="text-sm font-semibold leading-tight line-clamp-2">{listing.title}</h3>

        {/* Provider + Model */}
        <p className="text-xs text-muted-foreground">
          {listing.provider} · {listing.model}
        </p>

        {/* Price */}
        <p className="text-lg font-bold tabular-nums">
          {formatPrice(listing.pricing.centsPerUnit, listing.pricing.unit)}
        </p>

        {/* Capacity bar */}
        <div>
          <div className="flex items-center justify-between text-[10px] text-muted-foreground mb-1">
            <span>{capacityPct}% available</span>
            <span>{listing.remainingCapacity}/{listing.totalCapacity}</span>
          </div>
          <div className="h-1.5 rounded-full bg-muted overflow-hidden">
            <div
              className="h-full rounded-full bg-primary transition-all"
              style={{ width: `${capacityPct}%` }}
            />
          </div>
        </div>

        {/* Rating + Orders */}
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span className="text-amber-500">{ratingStars(listing.rating)} {listing.rating.toFixed(1)}</span>
          <span>{listing.totalOrders} orders</span>
        </div>
      </CardContent>
    </Card>
  );
}
