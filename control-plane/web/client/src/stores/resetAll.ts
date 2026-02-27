/**
 * Reset All Stores
 *
 * Call on logout or tenant switch to prevent cross-tenant state leaks.
 * Also disconnects the gateway singleton.
 */

import { gateway } from '@/services/gatewayClient';
import { useCanvasStore } from './canvasStore';
import { useBotStore } from './botStore';
import { useActionPillStore } from './actionPillStore';
import { usePermissionModeStore } from './permissionModeStore';
import { useTeamStore } from './teamStore';
import { useTenantStore } from './tenantStore';
import { useSpaceStore } from './spaceStore';
import { useNetworkStore } from './networkStore';

export function resetAllStores(): void {
  gateway.disconnect();
  useCanvasStore.getState().reset();
  useBotStore.getState().reset();
  useActionPillStore.getState().reset();
  usePermissionModeStore.getState().reset();
  useTeamStore.getState().reset();
  useTenantStore.getState().reset();
  useSpaceStore.getState().reset();
  useNetworkStore.getState().reset();
}
