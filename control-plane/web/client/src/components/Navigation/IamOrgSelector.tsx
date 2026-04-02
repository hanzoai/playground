/**
 * IAM Org Selector -- only loaded when IAM mode is active.
 * Uses useOrganizations() which requires IamProvider context.
 * Includes a "Create Organization" option at the bottom of the dropdown.
 */

import { useMemo, useState } from "react";
import { useOrganizations } from "@hanzo/iam/react";
import { useTenantStore } from "@/stores/tenantStore";
import { cn } from "@/lib/utils";
import { ChevronDown, Plus } from "@/components/ui/icon-bridge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { CreateOrgDialog } from "@/components/org/CreateOrgDialog";

export function IamOrgSelector() {
  const orgState = useOrganizations();
  const setTenantOrg = useTenantStore((s) => s.setOrg);
  const knownOrgs = useTenantStore((s) => s.knownOrgs);
  const [createOpen, setCreateOpen] = useState(false);

  // Merge IAM-fetched orgs with locally-created orgs so newly-created
  // orgs appear immediately, even if the IAM API didn't return them.
  const allOrgs = useMemo(() => {
    const iamOrgs = orgState.organizations ?? [];
    const names = new Set(iamOrgs.map((o) => o.name));
    const extras = knownOrgs
      .filter((ko) => !names.has(ko.name))
      .map((ko) => ({ owner: "admin", name: ko.name, displayName: ko.displayName }));
    const merged = [...iamOrgs, ...extras];
    // Ensure the currently-selected org always appears in the list
    const mergedNames = new Set(merged.map((o) => o.name));
    if (orgState.currentOrgId && !mergedNames.has(orgState.currentOrgId)) {
      merged.push({ owner: "admin", name: orgState.currentOrgId, displayName: orgState.currentOrgId });
    }
    return merged;
  }, [orgState.organizations, knownOrgs, orgState.currentOrgId]);

  const handleSwitch = (orgName: string) => {
    orgState.switchOrg(orgName);
    setTenantOrg(orgName);
  };

  const currentLabel =
    allOrgs.find((o) => o.name === orgState.currentOrgId)?.displayName
    || orgState.currentOrgId
    || "Select org";

  // When there is zero or one org, show the static label plus the create option
  if (allOrgs.length <= 1) {
    return (
      <>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button className="flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-sm font-medium hover:bg-accent transition-colors">
              <span className="truncate max-w-[120px]">{currentLabel}</span>
              <ChevronDown size={12} className="text-muted-foreground shrink-0" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-48">
            <DropdownMenuLabel>Organization</DropdownMenuLabel>
            <DropdownMenuSeparator />
            {allOrgs.map((org) => (
              <DropdownMenuItem
                key={org.name}
                onClick={() => handleSwitch(org.name)}
                className={cn(org.name === orgState.currentOrgId && "bg-accent")}
              >
                {org.displayName || org.name}
              </DropdownMenuItem>
            ))}
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => setCreateOpen(true)}>
              <Plus size={14} className="mr-1.5 text-muted-foreground" />
              Create Organization
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <CreateOrgDialog open={createOpen} onOpenChange={setCreateOpen} />
      </>
    );
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button className="flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-sm font-medium hover:bg-accent transition-colors">
            <span className="truncate max-w-[120px]">{currentLabel}</span>
            <ChevronDown size={12} className="text-muted-foreground shrink-0" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-48">
          <DropdownMenuLabel>Organization</DropdownMenuLabel>
          <DropdownMenuSeparator />
          {allOrgs.map((org) => (
            <DropdownMenuItem
              key={org.name}
              onClick={() => handleSwitch(org.name)}
              className={cn(org.name === orgState.currentOrgId && "bg-accent")}
            >
              {org.displayName || org.name}
            </DropdownMenuItem>
          ))}
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={() => setCreateOpen(true)}>
            <Plus size={14} className="mr-1.5 text-muted-foreground" />
            Create Organization
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <CreateOrgDialog open={createOpen} onOpenChange={setCreateOpen} />
    </>
  );
}
