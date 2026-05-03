/**
 * CreateOrgDialog -- modal for creating a new IAM organization.
 *
 * Uses the playground backend POST /v1/orgs endpoint which proxies to IAM
 * with proper service-level credentials. This ensures org creation succeeds
 * even when the user's token doesn't have IAM admin privileges.
 *
 * On success refreshes the org list and switches to the newly created org.
 */

import { useState } from "react";
import { useIam, useOrganizations } from "@hanzo/iam/react";
import { useTenantStore } from "@/stores/tenantStore";
import type { KnownOrg } from "@/stores/tenantStore";
import { getGlobalIamToken } from "@/services/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

interface CreateOrgDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CreateOrgDialog({ open, onOpenChange }: CreateOrgDialogProps) {
  const { accessToken } = useIam();
  const orgState = useOrganizations();
  const setTenantOrg = useTenantStore((s) => s.setOrg);
  const addKnownOrg = useTenantStore((s) => s.addKnownOrg);

  const [name, setName] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const resetForm = () => {
    setName("");
    setDisplayName("");
    setError(null);
    setSubmitting(false);
  };

  const handleClose = (nextOpen: boolean) => {
    if (!nextOpen) {
      resetForm();
    }
    onOpenChange(nextOpen);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    const trimmedName = name.trim();
    if (!trimmedName) {
      setError("Organization name is required.");
      return;
    }

    // Validate name format: lowercase alphanumeric and hyphens only
    if (!/^[a-z0-9][a-z0-9-]*[a-z0-9]$/.test(trimmedName) && trimmedName.length > 1) {
      setError("Name must be lowercase alphanumeric with hyphens, starting and ending with a letter or number.");
      return;
    }
    if (trimmedName.length === 1 && !/^[a-z0-9]$/.test(trimmedName)) {
      setError("Name must be lowercase alphanumeric.");
      return;
    }

    setSubmitting(true);
    setError(null);

    try {
      // Use the playground backend API which proxies to IAM with service credentials.
      // This avoids the issue where regular user tokens lack admin privileges on IAM.
      const apiBase = import.meta.env.VITE_API_BASE_URL || "/v1";
      const token = getGlobalIamToken() || accessToken;

      const res = await fetch(`${apiBase}/orgs`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({
          name: trimmedName,
          displayName: displayName.trim() || trimmedName,
        }),
      });

      if (!res.ok) {
        const errBody = await res.json().catch(() => ({}));
        throw new Error(errBody.error || errBody.msg || `HTTP ${res.status}`);
      }

      // Persist the new org locally so it shows in the switcher immediately,
      // even if the IAM API doesn't return it for non-admin users.
      const newOrg: KnownOrg = { name: trimmedName, displayName: displayName.trim() || trimmedName };
      addKnownOrg(newOrg);

      // Switch to the new org and persist the selection
      orgState.switchOrg(trimmedName);
      setTenantOrg(trimmedName);

      resetForm();
      onOpenChange(false);

      // Reload to re-fetch the org list in all useOrganizations() instances
      window.location.reload();
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(`Failed to create organization: ${msg}`);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-[425px]">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Create Organization</DialogTitle>
            <DialogDescription>
              Create a new organization to manage teams, projects, and resources.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="org-name">Name</Label>
              <Input
                id="org-name"
                placeholder="my-org"
                value={name}
                onChange={(e) => setName(e.target.value)}
                autoFocus
                disabled={submitting}
              />
              <p className="text-xs text-muted-foreground">
                Lowercase alphanumeric and hyphens. Used as the org identifier.
              </p>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="org-display-name">Display Name</Label>
              <Input
                id="org-display-name"
                placeholder="My Organization"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                disabled={submitting}
              />
              <p className="text-xs text-muted-foreground">
                Human-readable name shown in the UI.
              </p>
            </div>

            {error && (
              <p className="text-sm text-destructive">{error}</p>
            )}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => handleClose(false)}
              disabled={submitting}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={submitting || !name.trim()}>
              {submitting ? "Creating..." : "Create"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
