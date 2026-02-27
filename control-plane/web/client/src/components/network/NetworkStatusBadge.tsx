/**
 * NetworkStatusBadge — shows current sharing status.
 *
 * Displays a coloured badge indicating whether the user's AI capacity
 * is actively being shared, idle, disabled, or in cooldown.
 */

import { useNetworkStore } from '@/stores/networkStore';
import { Badge } from '@/components/ui/badge';
import type { SharingStatus } from '@/types/network';

const STATUS_CONFIG: Record<SharingStatus, { label: string; variant: string; dotColor: string }> = {
  active:   { label: 'Sharing',  variant: 'default',     dotColor: 'bg-emerald-500' },
  idle:     { label: 'Idle',     variant: 'secondary',   dotColor: 'bg-gray-400' },
  disabled: { label: 'Off',      variant: 'outline',     dotColor: 'bg-gray-300' },
  cooldown: { label: 'Cooldown', variant: 'secondary',   dotColor: 'bg-amber-500' },
};

interface NetworkStatusBadgeProps {
  compact?: boolean;
}

export function NetworkStatusBadge({ compact }: NetworkStatusBadgeProps) {
  const status = useNetworkStore((s) => s.sharingStatus);
  const rate = useNetworkStore((s) => s.earnings.currentRatePerHour);
  const cfg = STATUS_CONFIG[status];

  return (
    <Badge variant={cfg.variant as 'default' | 'secondary' | 'outline'} className="gap-1.5 text-xs font-medium">
      <span className={`size-1.5 rounded-full ${cfg.dotColor}`} />
      {cfg.label}
      {!compact && status === 'active' && rate > 0 && (
        <span className="text-muted-foreground ml-0.5">
          {rate.toFixed(2)}/hr
        </span>
      )}
    </Badge>
  );
}
