/**
 * OrderCard — buyer's order display with proxy endpoint and resale option.
 */

import { useNavigate } from 'react-router-dom';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { CapacityTypeIcon } from './CapacityTypeIcon';
import type { MarketplaceOrder } from '@/types/network';

const STATUS_VARIANT: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
  active: 'default',
  pending: 'outline',
  completed: 'secondary',
  cancelled: 'destructive',
  disputed: 'destructive',
};

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text).catch(() => {});
}

interface Props {
  order: MarketplaceOrder;
  onCancel?: (id: string) => void;
}

export function OrderCard({ order, onCancel }: Props) {
  const navigate = useNavigate();
  const usedPct = order.quantity > 0 ? Math.round((order.usedQuantity / order.quantity) * 100) : 0;
  const remaining = order.quantity - order.usedQuantity;
  const unitLabel = order.unit === 'hour' ? 'hrs' : order.unit === 'request' ? 'reqs' : '1k tok';

  return (
    <Card>
      <CardContent className="p-4 space-y-3">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 min-w-0">
            <CapacityTypeIcon type={order.capacityType} size={14} />
            <span className="text-sm font-medium truncate">{order.listingTitle}</span>
          </div>
          <Badge variant={STATUS_VARIANT[order.status] ?? 'secondary'} className="text-[10px] capitalize shrink-0">
            {order.status}
          </Badge>
        </div>

        {/* Model + Seller */}
        <p className="text-xs text-muted-foreground">
          {order.model} · from {order.sellerDisplayName}
        </p>

        {/* Usage bar */}
        <div>
          <div className="flex items-center justify-between text-[10px] text-muted-foreground mb-1">
            <span>{order.usedQuantity.toFixed(1)} / {order.quantity} {unitLabel} used</span>
            <span>{usedPct}%</span>
          </div>
          <div className="h-1.5 rounded-full bg-muted overflow-hidden">
            <div
              className="h-full rounded-full bg-primary transition-all"
              style={{ width: `${usedPct}%` }}
            />
          </div>
        </div>

        {/* Cost */}
        <p className="text-xs text-muted-foreground">
          Total: ${(order.totalCostCents / 100).toFixed(2)} USD
        </p>

        {/* Proxy endpoint */}
        {order.proxyEndpoint && (
          <div className="rounded-md bg-muted px-3 py-2">
            <p className="text-[10px] text-muted-foreground mb-0.5">Proxy Endpoint</p>
            <div className="flex items-center gap-2">
              <code className="text-xs font-mono truncate flex-1">{order.proxyEndpoint}</code>
              <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px]" onClick={() => copyToClipboard(order.proxyEndpoint!)}>
                Copy
              </Button>
            </div>
          </div>
        )}

        {/* S3 bucket */}
        {order.s3TransferBucket && (
          <div className="rounded-md bg-muted px-3 py-2">
            <p className="text-[10px] text-muted-foreground mb-0.5">S3 Transfer</p>
            <code className="text-xs font-mono">{order.s3TransferBucket}</code>
          </div>
        )}

        {/* Actions */}
        <div className="flex items-center gap-2 pt-1">
          {order.status === 'active' && remaining > 0 && (
            <Button
              variant="outline"
              size="sm"
              className="text-xs"
              onClick={() => navigate(`/marketplace/create?source_order=${order.id}&type=${order.capacityType}&model=${order.model}`)}
            >
              Resell Remaining
            </Button>
          )}
          {order.status === 'pending' && onCancel && (
            <Button
              variant="destructive"
              size="sm"
              className="text-xs"
              onClick={() => onCancel(order.id)}
            >
              Cancel
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
