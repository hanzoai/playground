import { NavLink } from "react-router-dom";
import { useState } from "react";

import type { NavigationSection } from "./types";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from "@/components/ui/sidebar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Icon } from "@/components/ui/icon";
import { ChevronDown } from "@/components/ui/icon-bridge";
import { cn } from "@/lib/utils";
import { useAuth } from "@/contexts/AuthContext";

// Hanzo "H" logo mark — geometric H from official brand assets.
function HanzoLogo({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 67 67" xmlns="http://www.w3.org/2000/svg" className={className}>
      <path d="M22.21 67V44.6369H0V67H22.21Z" fill="currentColor"/>
      <path d="M66.7038 22.3184H22.2534L0.0878906 44.6367H44.4634L66.7038 22.3184Z" fill="currentColor"/>
      <path d="M22.21 0H0V22.3184H22.21V0Z" fill="currentColor"/>
      <path d="M66.7198 0H44.5098V22.3184H66.7198V0Z" fill="currentColor"/>
      <path d="M66.7198 67V44.6369H44.5098V67H66.7198Z" fill="currentColor"/>
    </svg>
  );
}

const ORGS = [
  { id: "hanzo", name: "Hanzo" },
  { id: "lux", name: "Lux" },
  { id: "zoo", name: "Zoo" },
];

const PROJECTS = [
  { id: "default", name: "Default" },
  { id: "staging", name: "Staging" },
  { id: "production", name: "Production" },
];

interface SidebarNewProps {
  sections: NavigationSection[];
}

export function SidebarNew({ sections }: SidebarNewProps) {
  const { state } = useSidebar();
  const isCollapsed = state === "collapsed";
  const { isAuthenticated, authRequired, clearAuth } = useAuth();
  const [activeOrg, setActiveOrg] = useState(ORGS[0]);
  const [activeProject, setActiveProject] = useState(PROJECTS[0]);

  return (
    <Sidebar collapsible="icon" className="border-r border-border/40 bg-sidebar/95 backdrop-blur supports-[backdrop-filter]:bg-sidebar/60">
      {/* Header — Hanzo logo + org/project selectors */}
      <SidebarHeader className="pb-2 border-b border-border/40">
        <SidebarMenu>
          {/* Brand row */}
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild className="active:scale-[0.98] transition-transform">
              <NavLink to="/dashboard">
                <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground shadow-sm">
                  <HanzoLogo className="size-4" />
                </div>
                <div className="grid flex-1 text-left text-sm leading-tight">
                  <span className="truncate font-semibold tracking-tight">Hanzo Bot</span>
                  <span className="truncate text-[10px] text-muted-foreground font-mono">v1.0.0</span>
                </div>
              </NavLink>
            </SidebarMenuButton>
          </SidebarMenuItem>

          {/* Org selector */}
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <SidebarMenuButton
                  className="h-8 text-[13px] justify-between"
                  tooltip={isCollapsed ? `Org: ${activeOrg.name}` : undefined}
                >
                  <span className="flex items-center gap-2">
                    <Icon name="users" size={14} className="text-muted-foreground" />
                    <span className="truncate">{activeOrg.name}</span>
                  </span>
                  <ChevronDown size={12} className="text-muted-foreground shrink-0" />
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="right" align="start" className="w-48">
                <DropdownMenuLabel>Organization</DropdownMenuLabel>
                <DropdownMenuSeparator />
                {ORGS.map((org) => (
                  <DropdownMenuItem
                    key={org.id}
                    onClick={() => setActiveOrg(org)}
                    className={cn(org.id === activeOrg.id && "bg-accent")}
                  >
                    {org.name}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>

          {/* Project selector */}
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <SidebarMenuButton
                  className="h-8 text-[13px] justify-between"
                  tooltip={isCollapsed ? `Project: ${activeProject.name}` : undefined}
                >
                  <span className="flex items-center gap-2">
                    <Icon name="package" size={14} className="text-muted-foreground" />
                    <span className="truncate">{activeProject.name}</span>
                  </span>
                  <ChevronDown size={12} className="text-muted-foreground shrink-0" />
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="right" align="start" className="w-48">
                <DropdownMenuLabel>Project</DropdownMenuLabel>
                <DropdownMenuSeparator />
                {PROJECTS.map((proj) => (
                  <DropdownMenuItem
                    key={proj.id}
                    onClick={() => setActiveProject(proj)}
                    className={cn(proj.id === activeProject.id && "bg-accent")}
                  >
                    {proj.name}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      {/* Content */}
      <SidebarContent className="space-y-4 px-2">
        {sections.map((section) => (
          <SidebarGroup key={section.id} className="space-y-0.5">
            <SidebarGroupLabel className="text-[10px] uppercase tracking-wider font-semibold text-muted-foreground/70 px-2 mb-1">
              {section.title}
            </SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {section.items.map((item) => (
                  <SidebarMenuItem key={item.id}>
                    {item.disabled ? (
                      <SidebarMenuButton
                        isActive={false}
                        tooltip={isCollapsed ? item.label : undefined}
                        disabled
                        className="h-8 text-[13px]"
                      >
                        {item.icon && <Icon name={item.icon} size={15} />}
                        <span>{item.label}</span>
                      </SidebarMenuButton>
                    ) : (
                      <NavLink to={item.href} className="block">
                        {({ isActive }) => (
                          <SidebarMenuButton
                            asChild
                            isActive={isActive}
                            tooltip={isCollapsed ? item.label : undefined}
                            className={cn(
                              "h-8 text-[13px] transition-all duration-200 relative",
                              isActive
                                ? "bg-sidebar-accent text-sidebar-accent-foreground font-medium shadow-sm"
                                : "text-muted-foreground hover:text-foreground hover:bg-sidebar-accent/50"
                            )}
                          >
                            <span className="flex items-center gap-2.5">
                              {isActive && (
                                <div className="absolute left-0 top-1/2 -translate-y-1/2 h-4 w-0.5 bg-primary rounded-r-full" />
                              )}
                              {item.icon && <Icon name={item.icon} size={15} className={cn(isActive ? "text-primary" : "text-muted-foreground")} />}
                              <span>{item.label}</span>
                            </span>
                          </SidebarMenuButton>
                        )}
                      </NavLink>
                    )}
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        ))}
      </SidebarContent>

      {/* Footer — user menu + links */}
      <SidebarFooter className="border-t border-border/40 pt-2">
        <SidebarMenu>
          {/* User dropdown */}
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <SidebarMenuButton
                  className="h-9 text-[13px]"
                  tooltip={isCollapsed ? "Account" : undefined}
                >
                  <div className="flex aspect-square size-6 items-center justify-center rounded-full bg-muted text-muted-foreground text-[10px] font-bold">
                    {isAuthenticated ? "U" : "?"}
                  </div>
                  <div className="grid flex-1 text-left text-sm leading-tight">
                    <span className="truncate text-xs font-medium">
                      {isAuthenticated ? "User" : "Not signed in"}
                    </span>
                    <span className="truncate text-[10px] text-muted-foreground">
                      {activeOrg.name}
                    </span>
                  </div>
                  <ChevronDown size={12} className="text-muted-foreground shrink-0" />
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="right" align="end" className="w-48">
                <DropdownMenuLabel>Account</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem asChild>
                  <a href="https://hanzo.ai/docs" target="_blank" rel="noopener noreferrer">
                    Documentation
                  </a>
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                  <a href="https://github.com/hanzoai/bot" target="_blank" rel="noopener noreferrer">
                    GitHub
                  </a>
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                  <a href="https://github.com/hanzoai/bot/issues" target="_blank" rel="noopener noreferrer">
                    Support
                  </a>
                </DropdownMenuItem>
                {authRequired && (
                  <>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={clearAuth} className="text-destructive">
                      Sign out
                    </DropdownMenuItem>
                  </>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
    </Sidebar>
  );
}
