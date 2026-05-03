/**
 * InviteMemberDialog -- modal for inviting a member to the current organization.
 *
 * Uses the Casdoor POST /api/add-invitation + POST /api/send-invitation endpoints
 * via IamClient.apiRequest. Shows the invite code after successful creation.
 */

import { useState } from "react";
import { useIam, useOrganizations } from "@hanzo/iam/react";
import { IamClient } from "@hanzo/iam";
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
import { Copy } from "@/components/ui/icon-bridge";

interface InviteMemberDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function InviteMemberDialog({ open, onOpenChange }: InviteMemberDialogProps) {
  const { config, accessToken } = useIam();
  const { currentOrgId } = useOrganizations();

  const [email, setEmail] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [inviteCode, setInviteCode] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const resetForm = () => {
    setEmail("");
    setError(null);
    setSubmitting(false);
    setInviteCode(null);
    setCopied(false);
  };

  const handleClose = (nextOpen: boolean) => {
    if (!nextOpen) {
      resetForm();
    }
    onOpenChange(nextOpen);
  };

  const handleCopy = async () => {
    if (!inviteCode) return;
    try {
      await navigator.clipboard.writeText(inviteCode);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback: select the text for manual copy
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    const trimmedEmail = email.trim();
    if (!trimmedEmail) {
      setError("Email address is required.");
      return;
    }

    // Basic email validation
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmedEmail)) {
      setError("Please enter a valid email address.");
      return;
    }

    if (!currentOrgId) {
      setError("No organization selected.");
      return;
    }

    setSubmitting(true);
    setError(null);

    try {
      const client = new IamClient({
        serverUrl: config.serverUrl,
        clientId: config.clientId,
      });

      // Generate a unique invitation name
      const inviteName = `invite-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

      const invitationPayload = {
        owner: currentOrgId,
        name: inviteName,
        displayName: `Invitation for ${trimmedEmail}`,
        email: trimmedEmail,
        organization: currentOrgId,
        state: "Active",
      };

      // Create the invitation
      const addResult = await client.apiRequest<{ status: string; msg?: string; data?: string }>(
        "/api/add-invitation",
        {
          method: "POST",
          body: invitationPayload,
          token: accessToken ?? undefined,
        },
      );

      if (addResult.status === "error") {
        throw new Error(addResult.msg || "Failed to create invitation");
      }

      // Send the invitation email
      try {
        await client.apiRequest("/api/send-invitation", {
          method: "POST",
          body: {
            owner: currentOrgId,
            name: inviteName,
          },
          token: accessToken ?? undefined,
        });
      } catch {
        // Sending email may fail if SMTP is not configured; invitation is still valid
      }

      // The invite code is typically the invitation name or returned in the response
      setInviteCode(addResult.data || inviteName);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(`Failed to create invitation: ${msg}`);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-[425px]">
        {inviteCode ? (
          <>
            <DialogHeader>
              <DialogTitle>Invitation Created</DialogTitle>
              <DialogDescription>
                Share this invite code with the new member. They can use it to join your organization.
              </DialogDescription>
            </DialogHeader>

            <div className="py-4">
              <Label>Invite Code</Label>
              <div className="flex items-center gap-2 mt-2">
                <Input
                  readOnly
                  value={inviteCode}
                  className="font-mono text-sm"
                />
                <Button
                  type="button"
                  variant="outline"
                  size="icon"
                  onClick={handleCopy}
                  title="Copy invite code"
                >
                  <Copy size={16} />
                </Button>
              </div>
              {copied && (
                <p className="text-xs text-muted-foreground mt-1">Copied to clipboard</p>
              )}
              <p className="text-xs text-muted-foreground mt-3">
                An invitation email has been sent to <strong>{email}</strong> if SMTP is configured.
              </p>
            </div>

            <DialogFooter>
              <Button onClick={() => handleClose(false)}>
                Done
              </Button>
            </DialogFooter>
          </>
        ) : (
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>Invite Member</DialogTitle>
              <DialogDescription>
                Send an invitation to join <strong>{currentOrgId}</strong>.
              </DialogDescription>
            </DialogHeader>

            <div className="grid gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="invite-email">Email Address</Label>
                <Input
                  id="invite-email"
                  type="email"
                  placeholder="colleague@example.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  autoFocus
                  disabled={submitting}
                />
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
              <Button type="submit" disabled={submitting || !email.trim()}>
                {submitting ? "Sending..." : "Send Invitation"}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
