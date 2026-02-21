import React, { Suspense } from "react";
import { useLocation, Link, useNavigate } from "react-router-dom";
import { cn } from "@/lib/utils";
import {
  Breadcrumb,
  BreadcrumbList,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
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
import { useSpaceStore } from "@/stores/spaceStore";
import { useTenantStore, ENVIRONMENTS } from "@/stores/tenantStore";
import type { Environment } from "@/stores/tenantStore";

// ---------------------------------------------------------------------------
// Org selector — uses IAM hook when available, otherwise tenantStore
// ---------------------------------------------------------------------------

function OrgSelector() {
  const { authMode } = useAuth();
  const orgId = useTenantStore((s) => s.orgId);

  // In IAM mode, read orgs from IAM context via the existing OrgProjectSwitcher pattern
  // For now, display the current org from tenantStore (synced by IAM hooks upstream)
  const currentLabel = orgId || "Hanzo";

  // In API-key mode or when only one org, show static label
  if (authMode !== "iam") {
    return (
      <div className="flex items-center gap-1.5 px-2.5 py-1.5 text-sm font-medium select-none">
        <span className="truncate max-w-[120px]">{currentLabel}</span>
      </div>
    );
  }

  // In IAM mode, render the full IAM org selector lazily
  return (
    <Suspense fallback={<LocalOrgFallback />}>
      <IamOrgSelectorLazy />
    </Suspense>
  );
}

// Lazy-loaded IAM org selector — only imported when IAM mode is active
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
// Space selector — wired to spaceStore
// ---------------------------------------------------------------------------

function SpaceSelector() {
  const spaces = useSpaceStore((s) => s.spaces);
  const activeSpace = useSpaceStore((s) => s.activeSpace);
  const activeSpaceId = useSpaceStore((s) => s.activeSpaceId);
  const setActiveSpace = useSpaceStore((s) => s.setActiveSpace);
  const fetchSpaces = useSpaceStore((s) => s.fetchSpaces);
  const navigate = useNavigate();

  // Bootstrap spaces if not loaded
  React.useEffect(() => {
    if (spaces.length === 0) fetchSpaces();
  }, [spaces.length, fetchSpaces]);

  const currentLabel = activeSpace?.name || "No space";

  if (spaces.length === 0) {
    return (
      <button
        onClick={() => navigate("/spaces")}
        className="flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-sm text-muted-foreground hover:bg-accent transition-colors"
      >
        <span>No space</span>
      </button>
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
      <DropdownMenuContent align="start" className="w-52">
        <DropdownMenuLabel>Space</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {spaces.map((space) => (
          <DropdownMenuItem
            key={space.id}
            onClick={() => setActiveSpace(space.id)}
            className={cn(space.id === activeSpaceId && "bg-accent")}
          >
            <span className="truncate">{space.name}</span>
            {space.id === activeSpaceId && (
              <span className="ml-auto text-[10px] text-primary font-medium">Active</span>
            )}
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => navigate("/spaces")}>
          <span className="text-muted-foreground">Manage spaces...</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
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
        <button className="flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-sm font-medium hover:bg-accent transition-colors">
          <span className="truncate">{currentEnv.name}</span>
          <ChevronDown size={12} className="text-muted-foreground shrink-0" />
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
// TopNavigation
// ---------------------------------------------------------------------------

export function TopNavigation() {
  const location = useLocation();

  // Generate breadcrumbs from current path
  const generateBreadcrumbs = () => {
    const pathSegments = location.pathname.split("/").filter(Boolean);
    const breadcrumbs = [{ label: "Home", href: "/" }];

    const routeMappings: Record<
      string,
      { label: string; href: string; parent?: string }
    > = {
      bots: { label: "Bots", href: "/bots/all" },
      "bots/all": { label: "All Bots", href: "/bots/all", parent: "bots" },
      "bots/executions": { label: "Recent Executions", href: "/bots/executions", parent: "bots" },
      "bots/workflows": { label: "Workflows", href: "/bots/workflows", parent: "bots" },
      nodes: { label: "Nodes", href: "/nodes" },
      packages: { label: "Packages", href: "/packages" },
      settings: { label: "Settings", href: "/settings" },
      agents: { label: "My Bots", href: "/agents" },
      canvas: { label: "Playground", href: "/canvas" },
      playground: { label: "Playground", href: "/playground" },
      spaces: { label: "Spaces", href: "/spaces" },
      "spaces/settings": { label: "Settings", href: "/spaces/settings", parent: "spaces" },
      dashboard: { label: "Dashboard", href: "/dashboard" },
      "dashboard/enhanced": { label: "Enhanced Dashboard", href: "/dashboard/enhanced", parent: "dashboard" },
      identity: { label: "Identity & Trust", href: "/identity/dids" },
      "identity/dids": { label: "DID Explorer", href: "/identity/dids" },
      "identity/credentials": { label: "Credentials", href: "/identity/credentials" },
      teams: { label: "Teams", href: "/teams" },
    };

    let currentPath = "";
    pathSegments.forEach((segment, index) => {
      currentPath += `/${segment}`;
      const routeKey = pathSegments.slice(0, index + 1).join("/");

      if (routeMappings[routeKey]) {
        const mapping = routeMappings[routeKey];

        if (routeKey === "bots/all") {
          const existingBotsIndex = breadcrumbs.findIndex((b) => b.label === "Bots");
          if (existingBotsIndex !== -1) {
            breadcrumbs[existingBotsIndex] = { label: "Bots", href: "/bots/all" };
          } else {
            breadcrumbs.push({ label: "Bots", href: "/bots/all" });
          }
          return;
        }

        breadcrumbs.push({ label: mapping.label, href: mapping.href });
      } else {
        let label = segment.charAt(0).toUpperCase() + segment.slice(1).replace("-", " ");
        const href = currentPath;

        if (
          pathSegments[index - 1] === "bots" &&
          segment !== "all" &&
          segment !== "executions" &&
          segment !== "workflows"
        ) {
          try {
            const decodedId = decodeURIComponent(segment);
            const parts = decodedId.split(".");
            label = parts.length >= 2 ? parts[parts.length - 1] : decodedId;
          } catch {
            label = segment;
          }
          const botsIndex = breadcrumbs.findIndex((b) => b.label === "Bots");
          if (botsIndex !== -1) breadcrumbs[botsIndex].href = "/bots/all";
        } else if (pathSegments[index - 1] === "nodes") {
          label = `Node ${segment}`;
        }

        breadcrumbs.push({ label, href });
      }
    });

    return breadcrumbs;
  };

  const breadcrumbs = generateBreadcrumbs();

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
      {/* Left — Sidebar toggle + Org / Space / Environment */}
      <div className="flex items-center gap-0.5 min-w-0">
        <SidebarTrigger className="-ml-1" />
        <Separator orientation="vertical" className="mx-1.5 h-4" />

        {/* Org */}
        <OrgSelector />

        <span className="text-muted-foreground/50 text-xs select-none">/</span>

        {/* Space */}
        <SpaceSelector />

        <span className="text-muted-foreground/50 text-xs select-none">/</span>

        {/* Environment */}
        <EnvironmentSelector />
      </div>

      {/* Center — Breadcrumbs */}
      <div className="hidden md:flex items-center flex-1 justify-center min-w-0 mx-4">
        <Breadcrumb>
          <BreadcrumbList>
            {breadcrumbs.map((crumb, index) => {
              const isLast = index === breadcrumbs.length - 1;
              return (
                <React.Fragment key={crumb.href}>
                  <BreadcrumbItem>
                    {isLast ? (
                      <BreadcrumbPage className="max-w-[150px] truncate" title={crumb.label}>
                        {crumb.label}
                      </BreadcrumbPage>
                    ) : (
                      <BreadcrumbLink asChild>
                        <Link to={crumb.href} className="max-w-[150px] truncate" title={crumb.label}>
                          {crumb.label}
                        </Link>
                      </BreadcrumbLink>
                    )}
                  </BreadcrumbItem>
                  {!isLast && <BreadcrumbSeparator />}
                </React.Fragment>
              );
            })}
          </BreadcrumbList>
        </Breadcrumb>
      </div>

      {/* Right — Theme toggle */}
      <div className="flex items-center gap-2">
        <ModeToggle />
      </div>
    </header>
  );
}
