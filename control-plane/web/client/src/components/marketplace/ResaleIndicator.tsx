/**
 * ResaleIndicator — badge showing a listing is resold capacity.
 */

import { Badge } from '@/components/ui/badge';
import { Link } from '@/components/ui/icon-bridge';

interface Props {
  parentListingId: string | null;
  className?: string;
}

export function ResaleIndicator({ parentListingId, className }: Props) {
  return (
    <Badge variant="outline" className={`text-[10px] gap-1 ${className ?? ''}`}>
      <Link size={10} className="shrink-0" />
      Resold
      {parentListingId && (
        <span className="text-muted-foreground ml-0.5">#{parentListingId.slice(-3)}</span>
      )}
    </Badge>
  );
}
