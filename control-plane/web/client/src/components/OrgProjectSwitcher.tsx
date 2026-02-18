/**
 * Org/Project Switcher Component
 *
 * Dropdown for switching between organizations and projects.
 * Only rendered in IAM auth mode when orgs are available.
 * Syncs selection to the Zustand tenant store so useGateway picks it up.
 */

import { useEffect } from "react";
import { useOrganizations } from "@hanzo/iam/react";
import { useTenantStore } from "../stores/tenantStore";

export function OrgProjectSwitcher() {
  const {
    organizations,
    currentOrgId,
    switchOrg,
  } = useOrganizations();

  const setTenantOrg = useTenantStore((s) => s.setOrg);

  // Sync org selection to tenant store for useGateway
  useEffect(() => {
    setTenantOrg(currentOrgId);
  }, [currentOrgId, setTenantOrg]);

  if (organizations.length <= 1) return null;

  return (
    <select
      value={currentOrgId ?? ""}
      onChange={(e) => switchOrg(e.target.value)}
      className="h-8 rounded-md border border-border bg-background px-2 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
      aria-label="Switch organization"
    >
      {organizations.map((org) => (
        <option key={org.name} value={org.name}>
          {org.displayName || org.name}
        </option>
      ))}
    </select>
  );
}
