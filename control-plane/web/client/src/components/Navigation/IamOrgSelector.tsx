/**
 * IAM Org Selector -- only loaded when IAM mode is active.
 * Uses useOrganizations() which requires IamProvider context.
 * Includes a "Create Organization" option at the bottom of the dropdown.
 */

import { useState } from "react";
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
  const [createOpen, setCreateOpen] = useState(false);

  const handleSwitch = (orgName: string) => {
    orgState.switchOrg(orgName);
    setTenantOrg(orgName);
  };

  const currentLabel =
    orgState.currentOrg?.displayName || orgState.currentOrgId || "Select org";

  // When there is zero or one org, show the static label plus the create option
  if (!orgState.organizations || orgState.organizations.length <= 1) {
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
            {orgState.organizations?.map((org) => (
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
          {orgState.organizations.map((org) => (
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
