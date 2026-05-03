/**
 * SellerDashboardPage — manage listings and track revenue.
 */

import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useNetworkStore } from '@/stores/networkStore';
import { CapacityTypeIcon } from '@/components/marketplace/CapacityTypeIcon';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';

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

function formatCents(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

export function SellerDashboardPage() {
  const navigate = useNavigate();
  const dashboard = useNetworkStore((s) => s.sellerDashboard);
  const myListings = useNetworkStore((s) => s.myListings);
  const refreshSellerDashboard = useNetworkStore((s) => s.refreshSellerDashboard);
  const refreshMyListings = useNetworkStore((s) => s.refreshMyListings);
  const deleteListing = useNetworkStore((s) => s.deleteListing);
  const updateListing = useNetworkStore((s) => s.updateListing);

  useEffect(() => {
    refreshSellerDashboard().catch(() => {});
    refreshMyListings().catch(() => {});
  }, [refreshSellerDashboard, refreshMyListings]);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Seller Dashboard</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Manage your listings and track revenue from AI capacity sales.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => navigate('/marketplace')}>
            Browse Marketplace
          </Button>
          <Button size="sm" onClick={() => navigate('/marketplace/create')}>
            Create Listing
          </Button>
        </div>
      </div>

      {/* Revenue stats */}
      {dashboard && (
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <MetricCard label="Today" value={formatCents(dashboard.todayRevenueCents)} />
          <MetricCard label="This Week" value={formatCents(dashboard.weekRevenueCents)} />
          <MetricCard label="This Month" value={formatCents(dashboard.monthRevenueCents)} />
          <MetricCard
            label="All Time"
            value={formatCents(dashboard.totalRevenueCents)}
            sub={`${dashboard.totalOrders} orders · ${dashboard.avgRating.toFixed(1)} avg rating`}
          />
        </div>
      )}

      {/* Quick stats */}
      {dashboard && (
        <div className="flex items-center gap-4 text-sm text-muted-foreground">
          <span>{dashboard.activeListings} active listings</span>
          <span>{dashboard.activeOrders} active orders</span>
          <span>{dashboard.totalOrders} total orders</span>
          <span className="text-amber-500">{'★'.repeat(Math.floor(dashboard.avgRating))} {dashboard.avgRating.toFixed(1)}</span>
        </div>
      )}

      {/* My Listings */}
      <Card>
        <CardHeader className="pb-2">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">
              My Listings
              <Badge variant="secondary" className="ml-2 text-[10px]">{myListings.length}</Badge>
            </CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          {myListings.length === 0 ? (
            <div className="text-center py-8">
              <p className="text-sm text-muted-foreground mb-3">You haven't created any listings yet.</p>
              <Button size="sm" onClick={() => navigate('/marketplace/create')}>Create Your First Listing</Button>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-xs text-muted-foreground">
                    <th className="pb-2 font-medium">Listing</th>
                    <th className="pb-2 font-medium">Price</th>
                    <th className="pb-2 font-medium">Capacity</th>
                    <th className="pb-2 font-medium">Orders</th>
                    <th className="pb-2 font-medium">Status</th>
                    <th className="pb-2 font-medium text-right">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {myListings.map((listing) => {
                    const unitLabel = listing.pricing.unit === 'hour' ? 'hr' : listing.pricing.unit === 'request' ? 'req' : '1k tok';
                    const capacityPct = listing.totalCapacity > 0
                      ? Math.round((listing.remainingCapacity / listing.totalCapacity) * 100)
                      : 0;

                    return (
                      <tr key={listing.id} className="border-b border-border/30 last:border-0">
                        <td className="py-3">
                          <div className="flex items-center gap-2 min-w-0">
                            <CapacityTypeIcon type={listing.capacityType} size={14} />
                            <div className="min-w-0">
                              <p className="font-medium truncate max-w-[200px]">{listing.title}</p>
                              <p className="text-xs text-muted-foreground">{listing.provider} · {listing.model}</p>
                            </div>
                          </div>
                        </td>
                        <td className="py-3 font-mono tabular-nums">
                          ${(listing.pricing.centsPerUnit / 100).toFixed(2)}/{unitLabel}
                        </td>
                        <td className="py-3">
                          <div className="flex items-center gap-2">
                            <div className="h-1.5 w-16 rounded-full bg-muted overflow-hidden">
                              <div className="h-full rounded-full bg-primary" style={{ width: `${capacityPct}%` }} />
                            </div>
                            <span className="text-xs text-muted-foreground tabular-nums">{capacityPct}%</span>
                          </div>
                        </td>
                        <td className="py-3 text-muted-foreground tabular-nums">{listing.totalOrders}</td>
                        <td className="py-3">
                          <Badge
                            variant={listing.status === 'active' ? 'default' : 'secondary'}
                            className="text-[10px] capitalize"
                          >
                            {listing.status}
                          </Badge>
                          {listing.isResale && (
                            <Badge variant="outline" className="text-[10px] ml-1">Resale</Badge>
                          )}
                        </td>
                        <td className="py-3">
                          <div className="flex items-center justify-end gap-1">
                            <Button
                              variant="ghost"
                              size="sm"
                              className="text-xs h-7"
                              onClick={() => updateListing(listing.id, { status: listing.status === 'active' ? 'paused' : 'active' })}
                            >
                              {listing.status === 'active' ? 'Pause' : 'Resume'}
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="text-xs h-7 text-destructive hover:text-destructive"
                              onClick={() => deleteListing(listing.id)}
                            >
                              Delete
                            </Button>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Revenue by listing */}
      {dashboard && dashboard.listings.length > 0 && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Revenue by Listing</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {dashboard.listings.map((lr) => (
                <div key={lr.listingId} className="flex items-center justify-between">
                  <div className="min-w-0">
                    <p className="text-sm font-medium truncate">{lr.title}</p>
                    <p className="text-xs text-muted-foreground">
                      {lr.activeOrders} active · {lr.completedOrders} completed · {lr.capacityUsedPercent}% used
                    </p>
                  </div>
                  <p className="text-sm font-bold tabular-nums shrink-0">{formatCents(lr.totalRevenueCents)}</p>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Getting started tips */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Seller Tips</CardTitle>
        </CardHeader>
        <CardContent>
          <ul className="text-sm text-muted-foreground space-y-2 list-disc list-inside">
            <li>Set competitive pricing — check the marketplace to see what others charge for similar capacity.</li>
            <li>Claude Code and GPT-4o listings tend to sell fastest. GPU compute commands premium prices.</li>
            <li>Keep your node online for reliability. Buyers rate sellers on uptime and response speed.</li>
            <li>Buy in bulk from others and resell in smaller chunks at a markup for arbitrage opportunities.</li>
            <li>Revenue is deposited to your USD balance. Connect a wallet on the Network page to withdraw as AI coin.</li>
          </ul>
        </CardContent>
      </Card>
    </div>
  );
}
