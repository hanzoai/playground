/**
 * CapacityTypeIcon — icon + color for each AI capacity type.
 */

import { Bot, Tag, Cpu, Flash } from '@/components/ui/icon-bridge';
import type { CapacityType } from '@/types/network';
import type { Icon } from '@/components/ui/icon-bridge';

const MAP: Record<CapacityType, { icon: Icon; color: string; label: string }> = {
  'claude-code': { icon: Bot, color: 'text-purple-500', label: 'Claude Code' },
  'api-key':     { icon: Tag, color: 'text-blue-500', label: 'API Key' },
  'gpu-compute': { icon: Cpu, color: 'text-orange-500', label: 'GPU' },
  'inference':   { icon: Flash, color: 'text-green-500', label: 'Inference' },
};

interface Props {
  type: CapacityType;
  size?: number;
}

export function CapacityTypeIcon({ type, size = 16 }: Props) {
  const entry = MAP[type];
  const IconComp = entry.icon;
  return <IconComp size={size} className={`shrink-0 ${entry.color}`} />;
}

export function capacityTypeLabel(type: CapacityType): string {
  return MAP[type].label;
}
