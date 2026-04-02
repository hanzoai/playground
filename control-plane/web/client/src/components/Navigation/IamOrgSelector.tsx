/**
 * IAM Org Selector -- wraps the shared @hanzo/iam UserOrgMenu component.
 * Syncs org changes to the local tenantStore for playground-specific state.
 */

import { useCallback } from "react";
import { UserOrgMenu } from "@hanzo/iam/react";
import { useTenantStore } from "@/stores/tenantStore";

const API_BASE = import.meta.env.VITE_API_BASE_URL || "/v1";

export function IamOrgSelector() {
  const setTenantOrg = useTenantStore((s) => s.setOrg);
  const addKnownOrg = useTenantStore((s) => s.addKnownOrg);

  const handleOrgChange = useCallback(
    (orgId: string) => {
      setTenantOrg(orgId);
      addKnownOrg({ name: orgId, displayName: orgId });
    },
    [setTenantOrg, addKnownOrg],
  );

  return (
    <UserOrgMenu
      onOrgChange={handleOrgChange}
      createOrgEndpoint={`${API_BASE}/orgs`}
    />
  );
}
