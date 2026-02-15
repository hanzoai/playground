/**
 * Reset All Stores
 *
 * Call on logout or tenant switch to prevent cross-tenant state leaks.
 * Also disconnects the gateway singleton.
 */

import { gateway } from '@/services/gatewayClient';
import { useCanvasStore } from './canvasStore';
import { useAgentStore } from './agentStore';
import { useActionPillStore } from './actionPillStore';
import { usePermissionModeStore } from './permissionModeStore';
import { useTeamStore } from './teamStore';

export function resetAllStores(): void {
  gateway.disconnect();
  useCanvasStore.getState().reset();
  useAgentStore.getState().reset();
  useActionPillStore.getState().reset();
  usePermissionModeStore.getState().reset();
  useTeamStore.getState().reset();
}
