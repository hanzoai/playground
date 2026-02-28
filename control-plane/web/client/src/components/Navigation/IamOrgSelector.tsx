/**
 * IAM Org Selector — only loaded when IAM mode is active.
 * Uses useOrganizations() which requires IamProvider context.
 */

import { useOrganizations } from "@hanzo/iam/react";
import { useTenantStore } from "@/stores/tenantStore";
import { cn } from "@/lib/utils";
import { ChevronDown } from "@/components/ui/icon-bridge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export function IamOrgSelector() {
  const orgState = useOrganizations();
  const setTenantOrg = useTenantStore((s) => s.setOrg);

  const handleSwitch = (orgName: string) => {
    orgState.switchOrg(orgName);
    setTenantOrg(orgName);
  };

  const currentLabel =
    orgState.currentOrg?.displayName || orgState.currentOrgId || "Select org";

  if (!orgState.organizations || orgState.organizations.length <= 1) {
    return (
      <div className="flex items-center gap-1.5 px-2.5 py-1.5 text-sm font-medium select-none">
        <span className="truncate max-w-[120px]">{currentLabel}</span>
      </div>
    );
  }

  return (
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
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
