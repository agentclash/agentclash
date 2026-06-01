"use client";

import { useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { WorkspaceMember, WorkspaceRole } from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ToggleGroup } from "@/components/ui/toggle-group";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Copy, UserPlus, Loader2 } from "lucide-react";
import { toast } from "sonner";

const ROLE_OPTIONS: { value: WorkspaceRole; label: string }[] = [
  { value: "workspace_admin", label: "Admin" },
  { value: "workspace_member", label: "Member" },
  { value: "workspace_viewer", label: "Viewer" },
];

const ROLE_DESCRIPTIONS: Record<WorkspaceRole, string> = {
  workspace_admin: "Full control — manage members, infrastructure, and secrets.",
  workspace_member: "Can create agents, runs, and challenge packs.",
  workspace_viewer: "Read-only access to workspace data.",
};

interface WsInviteMemberDialogProps {
  workspaceId: string;
  onInvited: () => void;
}

export function WsInviteMemberDialog({
  workspaceId,
  onInvited,
}: WsInviteMemberDialogProps) {
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<WorkspaceRole>("workspace_member");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string>();
  const [inviteLink, setInviteLink] = useState("");
  const [invitedEmail, setInvitedEmail] = useState("");

  function handleOpenChange(next: boolean) {
    setOpen(next);
    if (next) {
      setEmail("");
      setRole("workspace_member");
      setError(undefined);
      setInviteLink("");
      setInvitedEmail("");
    }
  }

  async function handleInvite() {
    const trimmedEmail = email.trim();
    if (!trimmedEmail) return;
    setError(undefined);
    setSending(true);
    try {
      const token = await getAccessToken();
      if (!token) return;
      const api = createApiClient(token);
      const invited = await api.post<WorkspaceMember>(
        `/v1/workspaces/${workspaceId}/memberships`,
        {
          email: trimmedEmail,
          role,
        },
      );
      toast.success(`Invited ${trimmedEmail}`);
      setInvitedEmail(trimmedEmail);
      setInviteLink(invited.accept_url ?? "");
      onInvited();
      if (!invited.accept_url) {
        setOpen(false);
      }
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : "Failed to send invite",
      );
    } finally {
      setSending(false);
    }
  }

  async function handleCopyInviteLink() {
    if (!inviteLink) return;
    try {
      await navigator.clipboard.writeText(inviteLink);
      toast.success("Copied invite link");
    } catch {
      setError(
        "Could not copy the invite link. Select the link and copy it manually.",
      );
    }
  }

  function handleInviteAnother() {
    setEmail("");
    setRole("workspace_member");
    setInviteLink("");
    setInvitedEmail("");
    setError(undefined);
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger render={<Button size="sm" />}>
        <UserPlus className="size-4 mr-1.5" />
        Invite Member
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Invite Workspace Member</DialogTitle>
          <DialogDescription>
            The user must have an active organization membership first.
            Invites expire after 7 days.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {inviteLink ? (
            <div className="space-y-3">
              <div className="rounded-md border border-border bg-muted/40 px-3 py-2 text-sm">
                Invite sent to{" "}
                <span className="font-medium text-foreground">
                  {invitedEmail}
                </span>
                .
              </div>
              <div>
                <label className="block text-sm font-medium mb-1.5">
                  Invite link
                </label>
                <div className="flex gap-2">
                  <Input
                    readOnly
                    value={inviteLink}
                    className="font-mono text-xs"
                    onFocus={(event) => event.currentTarget.select()}
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    onClick={handleCopyInviteLink}
                    aria-label="Copy invite link"
                  >
                    <Copy className="size-4" />
                  </Button>
                </div>
                <p className="mt-1.5 text-xs text-muted-foreground">
                  Share this if the email does not arrive.
                </p>
              </div>
            </div>
          ) : (
            <>
              <div>
                <label className="block text-sm font-medium mb-1.5">
                  Email
                </label>
                <Input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  disabled={sending}
                  placeholder="colleague@company.com"
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-1.5">
                  Role
                </label>
                <ToggleGroup
                  options={ROLE_OPTIONS}
                  value={role}
                  onChange={setRole}
                  disabled={sending}
                />
                <p className="mt-1 text-xs text-muted-foreground">
                  {ROLE_DESCRIPTIONS[role]}
                </p>
              </div>
            </>
          )}

          {error && (
            <div className="rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2 text-xs text-destructive">
              {error}
            </div>
          )}
        </div>

        <DialogFooter>
          {inviteLink ? (
            <>
              <Button variant="outline" onClick={handleInviteAnother}>
                Invite another
              </Button>
              <Button onClick={() => setOpen(false)}>Done</Button>
            </>
          ) : (
            <Button onClick={handleInvite} disabled={!email.trim() || sending}>
              {sending && (
                <Loader2
                  data-icon="inline-start"
                  className="size-4 animate-spin"
                />
              )}
              {sending ? "Inviting..." : "Send Invite"}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
