/**
 * Org/Project Switcher Component
 *
 * Wraps the shared @hanzo/auth OrgProjectSwitcher with playground-specific
 * tenant store syncing. Uses @hanzo/iam/react for org/project data.
 */

import { useCallback } from "react";
import { useOrganizations } from "@hanzo/iam/react";
import { useTenantStore } from "../stores/tenantStore";
import { OrgProjectSwitcher as OrgProjectSwitcherBase } from "@hanzo/auth";

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
