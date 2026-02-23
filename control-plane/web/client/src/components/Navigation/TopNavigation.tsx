import React, { Suspense } from "react";
import { useLocation } from "react-router-dom";
import { cn } from "@/lib/utils";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SidebarTrigger } from "@/components/ui/sidebar";
import { Separator } from "@/components/ui/separator";
import { ModeToggle } from "@/components/ui/mode-toggle";
import { ChevronDown } from "@/components/ui/icon-bridge";
import { useAuth } from "@/contexts/AuthContext";
import { useTenantStore, ENVIRONMENTS } from "@/stores/tenantStore";
import type { Environment } from "@/stores/tenantStore";

// ---------------------------------------------------------------------------
// Org selector — uses IAM hook when available, otherwise tenantStore
// ---------------------------------------------------------------------------

function OrgSelector() {
  const { authMode } = useAuth();
  const orgId = useTenantStore((s) => s.orgId);
  const currentLabel = orgId || "Hanzo";

  if (authMode !== "iam") {
    return (
      <div className="flex items-center gap-1.5 px-2.5 py-1.5 text-sm font-medium select-none">
        <span className="truncate max-w-[120px]">{currentLabel}</span>
      </div>
    );
  }

  return (
    <Suspense fallback={<LocalOrgFallback />}>
      <IamOrgSelectorLazy />
    </Suspense>
  );
}

const IamOrgSelectorLazy = React.lazy(() =>
  import("@/components/Navigation/IamOrgSelector").then((m) => ({ default: m.IamOrgSelector }))
);

function LocalOrgFallback() {
  const orgId = useTenantStore((s) => s.orgId);
  return (
    <div className="flex items-center gap-1.5 px-2.5 py-1.5 text-sm font-medium select-none">
      <span className="truncate max-w-[120px]">{orgId || "Hanzo"}</span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Environment selector — wired to tenantStore
// ---------------------------------------------------------------------------

function EnvironmentSelector() {
  const environment = useTenantStore((s) => s.environment);
  const setEnvironment = useTenantStore((s) => s.setEnvironment);
  const currentEnv = ENVIRONMENTS.find((e) => e.id === environment) || ENVIRONMENTS[0];

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button className="flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium text-muted-foreground hover:bg-accent hover:text-foreground transition-colors">
          <span className="truncate">{currentEnv.name}</span>
          <ChevronDown size={10} className="text-muted-foreground/60 shrink-0" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-40">
        <DropdownMenuLabel>Environment</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {ENVIRONMENTS.map((env) => (
          <DropdownMenuItem
            key={env.id}
            onClick={() => setEnvironment(env.id as Environment)}
            className={cn(env.id === environment && "bg-accent")}
          >
            {env.name}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

// ---------------------------------------------------------------------------
// Page title from route
// ---------------------------------------------------------------------------

const ROUTE_TITLES: Record<string, string> = {
  dashboard: "Dashboard",
  "bots/all": "Control Plane",
  nodes: "Nodes",
  playground: "Playground",
  spaces: "Spaces",
  teams: "Teams",
  executions: "Executions",
  workflows: "Workflows",
  packages: "Packages",
  settings: "Settings",
  agents: "My Bots",
  "identity/dids": "DID Explorer",
  "identity/credentials": "Credentials",
};

function usePageTitle(): string | null {
  const { pathname } = useLocation();
  const path = pathname.replace(/^\//, "");
  // Exact match first
  if (ROUTE_TITLES[path]) return ROUTE_TITLES[path];
  // First segment match
  const first = path.split("/")[0];
  if (first && ROUTE_TITLES[first]) return ROUTE_TITLES[first];
  return null;
}

// ---------------------------------------------------------------------------
// TopNavigation
// ---------------------------------------------------------------------------

export function TopNavigation() {
  const pageTitle = usePageTitle();

  return (
    <header
      className={cn(
        "h-12 flex items-center justify-between sticky top-0 z-50",
        "bg-gradient-to-r from-bg-base via-bg-subtle to-bg-base",
        "backdrop-blur-xl border-b border-border/40",
        "shadow-soft transition-all duration-200",
        "px-2 md:px-4"
      )}
    >
      {/* Left — Sidebar toggle + Org / Environment */}
      <div className="flex items-center gap-0.5 min-w-0">
        <SidebarTrigger className="-ml-1" />
        <Separator orientation="vertical" className="mx-1.5 h-4" />

        {/* Org */}
        <OrgSelector />

        <span className="text-muted-foreground/50 text-xs select-none">/</span>

        {/* Environment (subtle) */}
        <EnvironmentSelector />
      </div>

      {/* Center — Page title */}
      {pageTitle && (
        <div className="hidden md:flex items-center">
          <span className="text-sm font-medium text-foreground/70">{pageTitle}</span>
        </div>
      )}

      {/* Right — Theme toggle */}
      <div className="flex items-center gap-2">
        <ModeToggle />
      </div>
    </header>
  );
}
