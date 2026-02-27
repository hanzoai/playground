/**
 * MarketplacePage — browse and purchase AI capacity.
 */

import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useNetworkStore } from '@/stores/networkStore';
import { ListingCard } from '@/components/marketplace/ListingCard';
import { OrderCard } from '@/components/marketplace/OrderCard';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import type { CapacityType } from '@/types/network';

function MetricCard({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <Card>
      <CardContent className="p-4">
        <p className="text-xs text-muted-foreground">{label}</p>
        <p className="text-2xl font-bold tabular-nums mt-1">{value}</p>
        {sub && <p className="text-xs text-muted-foreground mt-0.5">{sub}</p>}
      </CardContent>
    </Card>
  );
}

const CAPACITY_TYPES: { value: CapacityType | 'all'; label: string }[] = [
  { value: 'all', label: 'All Types' },
  { value: 'claude-code', label: 'Claude Code' },
  { value: 'custom-agent', label: 'Custom Agent' },
  { value: 'api-key', label: 'API Key' },
  { value: 'gpu-compute', label: 'GPU Compute' },
  { value: 'inference', label: 'Inference' },
];

const SORT_OPTIONS: { value: string; label: string }[] = [
  { value: 'newest', label: 'Newest' },
  { value: 'price_asc', label: 'Price: Low → High' },
  { value: 'price_desc', label: 'Price: High → Low' },
  { value: 'rating', label: 'Top Rated' },
];

export function MarketplacePage() {
  const navigate = useNavigate();
  const listings = useNetworkStore((s) => s.marketplaceListings);
  const myOrders = useNetworkStore((s) => s.myOrders);
  const stats = useNetworkStore((s) => s.marketplaceStats);
  const filter = useNetworkStore((s) => s.marketplaceFilter);
  const loading = useNetworkStore((s) => s.marketplaceLoading);
  const refreshMarketplace = useNetworkStore((s) => s.refreshMarketplace);
  const refreshMyOrders = useNetworkStore((s) => s.refreshMyOrders);
  const setFilter = useNetworkStore((s) => s.setMarketplaceFilter);
  const cancelOrder = useNetworkStore((s) => s.cancelOrder);

  useEffect(() => {
    refreshMarketplace().catch(() => {});
    refreshMyOrders().catch(() => {});
  }, [refreshMarketplace, refreshMyOrders]);

  // Re-fetch when filter changes
  useEffect(() => {
    refreshMarketplace().catch(() => {});
  }, [filter.capacityType, filter.provider, filter.sortBy, filter.searchQuery, refreshMarketplace]);

  const activeOrders = myOrders.filter((o) => o.status === 'active' || o.status === 'pending');
  const activeListings = listings.filter((l) => l.status === 'active');

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">AI Capacity Marketplace</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Buy and sell AI compute — Claude Code, custom agents, API keys, GPUs, and inference.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => navigate('/marketplace/seller')}>
            Seller Dashboard
          </Button>
          <Button size="sm" onClick={() => navigate('/marketplace/create')}>
            Sell Capacity
          </Button>
        </div>
      </div>

      {/* Stats row */}
      {stats && (
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <MetricCard
            label="Active Listings"
            value={stats.activeListings.toLocaleString()}
            sub={`${stats.totalListings.toLocaleString()} total`}
          />
          <MetricCard
            label="24h Volume"
            value={`$${(stats.totalVolumeCents24h / 100).toLocaleString()}`}
          />
          <MetricCard
            label="Avg Price / Hr"
            value={`$${(stats.avgPriceCentsPerHour / 100).toFixed(2)}`}
          />
          <MetricCard
            label="Sellers"
            value={stats.uniqueSellers.toLocaleString()}
            sub={`${stats.uniqueBuyers.toLocaleString()} buyers`}
          />
        </div>
      )}

      {/* Active orders banner */}
      {activeOrders.length > 0 && (
        <Card>
          <CardHeader className="pb-2">
            <div className="flex items-center justify-between">
              <CardTitle className="text-base">
                Your Active Orders
                <Badge variant="secondary" className="ml-2 text-[10px]">{activeOrders.length}</Badge>
              </CardTitle>
            </div>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
              {activeOrders.slice(0, 4).map((order) => (
                <OrderCard key={order.id} order={order} onCancel={cancelOrder} />
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Filter bar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <select
          className="rounded-md border border-border bg-background px-3 py-1.5 text-sm"
          value={filter.capacityType}
          onChange={(e) => setFilter({ capacityType: e.target.value as CapacityType | 'all' })}
        >
          {CAPACITY_TYPES.map((ct) => (
            <option key={ct.value} value={ct.value}>{ct.label}</option>
          ))}
        </select>

        <select
          className="rounded-md border border-border bg-background px-3 py-1.5 text-sm"
          value={filter.sortBy}
          onChange={(e) => setFilter({ sortBy: e.target.value as typeof filter.sortBy })}
        >
          {SORT_OPTIONS.map((s) => (
            <option key={s.value} value={s.value}>{s.label}</option>
          ))}
        </select>

        <input
          type="text"
          placeholder="Search listings..."
          className="flex-1 rounded-md border border-border bg-background px-3 py-1.5 text-sm"
          value={filter.searchQuery}
          onChange={(e) => setFilter({ searchQuery: e.target.value })}
        />
      </div>

      {/* Listings grid */}
      {loading ? (
        <div className="text-center py-12 text-muted-foreground text-sm">Loading listings...</div>
      ) : activeListings.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-muted-foreground text-sm">No listings found matching your filters.</p>
          <Button variant="link" size="sm" onClick={() => setFilter({ capacityType: 'all', provider: 'all', searchQuery: '', sortBy: 'newest' })}>
            Clear filters
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {activeListings.map((listing) => (
            <ListingCard key={listing.id} listing={listing} />
          ))}
        </div>
      )}

      {/* Show paused listings */}
      {listings.filter((l) => l.status !== 'active').length > 0 && (
        <div>
          <h3 className="text-sm font-medium text-muted-foreground mb-3">Unavailable</h3>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 opacity-60">
            {listings.filter((l) => l.status !== 'active').map((listing) => (
              <ListingCard key={listing.id} listing={listing} />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
