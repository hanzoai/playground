/**
 * OrgSettingsPage -- organization settings and member management.
 *
 * Shows org details (name, display name, logo) and a member list.
 * Uses Casdoor /api/get-organization and /api/get-organization-users endpoints.
 */

import { useEffect, useState, useCallback } from "react";
import { useIam, useOrganizations } from "@hanzo/iam/react";
import { IamClient } from "@hanzo/iam";
import type { IamUser } from "@hanzo/iam";
import { Button } from "@/components/ui/button";
import { Users, EnvelopeSimple } from "@/components/ui/icon-bridge";
import { InviteMemberDialog } from "@/components/org/InviteMemberDialog";

interface OrgDetails {
  owner: string;
  name: string;
  displayName?: string;
  logo?: string;
  logoDark?: string;
  websiteUrl?: string;
  createdTime?: string;
}

export function OrgSettingsPage() {
  const { config, accessToken } = useIam();
  const { currentOrgId, currentOrg } = useOrganizations();

  const [orgDetails, setOrgDetails] = useState<OrgDetails | null>(null);
  const [members, setMembers] = useState<IamUser[]>([]);
  const [loadingOrg, setLoadingOrg] = useState(true);
  const [loadingMembers, setLoadingMembers] = useState(true);
  const [inviteOpen, setInviteOpen] = useState(false);

  const fetchOrgDetails = useCallback(async () => {
    if (!currentOrgId || !accessToken) {
      setOrgDetails(null);
      setLoadingOrg(false);
      return;
    }

    setLoadingOrg(true);
    try {
      const client = new IamClient({
        serverUrl: config.serverUrl,
        clientId: config.clientId,
      });

      const org = await client.apiRequest<{ status: string; data?: OrgDetails }>(
        "/api/get-organization",
        {
          params: { id: `admin/${currentOrgId}` },
          token: accessToken,
        },
      );
      setOrgDetails(org.data ?? null);
    } catch {
      // If get-organization fails, fall back to data from useOrganizations
      if (currentOrg) {
        setOrgDetails({
          owner: currentOrg.owner,
          name: currentOrg.name,
          displayName: currentOrg.displayName,
          logo: currentOrg.logo,
        });
      }
    } finally {
      setLoadingOrg(false);
    }
  }, [currentOrgId, accessToken, config.serverUrl, config.clientId, currentOrg]);

  const fetchMembers = useCallback(async () => {
    if (!currentOrgId || !accessToken) {
      setMembers([]);
      setLoadingMembers(false);
      return;
    }

    setLoadingMembers(true);
    try {
      const client = new IamClient({
        serverUrl: config.serverUrl,
        clientId: config.clientId,
      });

      // Casdoor API: get users belonging to the org
      const result = await client.apiRequest<{ status: string; data?: IamUser[] }>(
        "/api/get-users",
        {
          params: { owner: currentOrgId },
          token: accessToken,
        },
      );
      setMembers(result.data ?? []);
    } catch {
      setMembers([]);
    } finally {
      setLoadingMembers(false);
    }
  }, [currentOrgId, accessToken, config.serverUrl, config.clientId]);

  useEffect(() => {
    fetchOrgDetails();
  }, [fetchOrgDetails]);

  useEffect(() => {
    fetchMembers();
  }, [fetchMembers]);

  if (!currentOrgId) {
    return (
      <div className="max-w-3xl mx-auto px-4 sm:px-0">
        <div className="text-center py-12 text-muted-foreground">
          <p className="text-lg mb-2">No organization selected</p>
          <p className="text-sm">Select an organization from the sidebar to view settings.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-3xl mx-auto px-4 sm:px-0">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-6">
        <div>
          <h1 className="text-heading-1">Organization</h1>
          <p className="text-body text-muted-foreground">
            Manage your organization settings and members
          </p>
        </div>
      </div>

      {/* Org Details Card */}
      {loadingOrg ? (
        <div className="border rounded-lg p-6 mb-6 bg-card">
          <p className="text-sm text-muted-foreground">Loading organization details...</p>
        </div>
      ) : orgDetails ? (
        <div className="border rounded-lg p-6 mb-6 bg-card">
          <h2 className="text-sm font-medium text-muted-foreground mb-4">Details</h2>

          <div className="flex items-start gap-4">
            {orgDetails.logo && (
              <img
                src={orgDetails.logo}
                alt={orgDetails.displayName || orgDetails.name}
                className="w-12 h-12 rounded-lg object-contain border bg-background"
              />
            )}
            <div className="flex-1 min-w-0">
              <div className="grid gap-3">
                <div>
                  <p className="text-xs text-muted-foreground">Name</p>
                  <p className="text-sm font-medium font-mono">{orgDetails.name}</p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">Display Name</p>
                  <p className="text-sm font-medium">
                    {orgDetails.displayName || orgDetails.name}
                  </p>
                </div>
                {orgDetails.websiteUrl && (
                  <div>
                    <p className="text-xs text-muted-foreground">Website</p>
                    <a
                      href={orgDetails.websiteUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-sm text-primary hover:underline"
                    >
                      {orgDetails.websiteUrl}
                    </a>
                  </div>
                )}
                {orgDetails.createdTime && (
                  <div>
                    <p className="text-xs text-muted-foreground">Created</p>
                    <p className="text-sm">{new Date(orgDetails.createdTime).toLocaleDateString()}</p>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      ) : (
        <div className="border rounded-lg p-6 mb-6 bg-card">
          <p className="text-sm text-muted-foreground">Could not load organization details.</p>
        </div>
      )}

      {/* Members Section */}
      <div className="border rounded-lg p-6 bg-card">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Users size={16} className="text-muted-foreground" />
            <h2 className="text-sm font-medium text-muted-foreground">Members</h2>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setInviteOpen(true)}
          >
            <EnvelopeSimple size={14} className="mr-1.5" />
            Invite Member
          </Button>
        </div>

        {loadingMembers ? (
          <p className="text-sm text-muted-foreground">Loading members...</p>
        ) : members.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            <p className="text-sm">No members found for this organization.</p>
          </div>
        ) : (
          <div className="space-y-2">
            {members.map((member) => (
              <div
                key={member.name}
                className="flex items-center gap-3 rounded-lg border p-3 transition-colors hover:bg-accent/50"
              >
                {member.avatar ? (
                  <img
                    src={member.avatar}
                    alt={member.displayName || member.name}
                    className="w-8 h-8 rounded-full object-cover"
                  />
                ) : (
                  <div className="w-8 h-8 rounded-full bg-muted flex items-center justify-center text-xs font-medium text-muted-foreground">
                    {(member.displayName || member.name).charAt(0).toUpperCase()}
                  </div>
                )}
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium truncate">
                    {member.displayName || member.name}
                  </p>
                  <p className="text-xs text-muted-foreground truncate">
                    {member.email || member.name}
                  </p>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  {member.isAdmin && (
                    <span className="text-xs bg-primary/10 text-primary px-2 py-0.5 rounded">
                      Admin
                    </span>
                  )}
                  {member.isGlobalAdmin && (
                    <span className="text-xs bg-blue-500/10 text-blue-600 dark:text-blue-400 px-2 py-0.5 rounded">
                      Global Admin
                    </span>
                  )}
                  <span className="text-xs text-muted-foreground">
                    {member.type || "user"}
                  </span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <InviteMemberDialog open={inviteOpen} onOpenChange={setInviteOpen} />
    </div>
  );
}
