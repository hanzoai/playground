/**
 * Org/Project Switcher Component
 *
 * Uses @hanzo/iam/react for both the component and org/project data.
 * Syncs tenant selection to playground's Zustand store.
 */

import { useCallback } from "react";
import { useOrganizations, OrgProjectSwitcher as OrgProjectSwitcherBase } from "@hanzo/iam/react";
import { useTenantStore } from "../stores/tenantStore";

export function OrgProjectSwitcher() {
  const orgState = useOrganizations();

  const setTenantOrg = useTenantStore((s) => s.setOrg);
  const setTenantProject = useTenantStore((s) => s.setProject);

  const handleTenantChange = useCallback(
    (orgId: string | null, projectId: string | null) => {
      setTenantOrg(orgId);
      setTenantProject(projectId);
    },
    [setTenantOrg, setTenantProject],
  );

  return (
    <OrgProjectSwitcherBase
      organizations={orgState.organizations}
      currentOrgId={orgState.currentOrgId}
      switchOrg={orgState.switchOrg}
      projects={orgState.projects}
      currentProjectId={orgState.currentProjectId}
      switchProject={orgState.switchProject}
      onTenantChange={handleTenantChange}
    />
  );
}
