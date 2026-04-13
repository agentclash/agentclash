"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { WorkspaceDetail } from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Loader2, AlertTriangle } from "lucide-react";
import { toast } from "sonner";
import Link from "next/link";

interface WsGeneralSettingsProps {
  workspace: WorkspaceDetail;
}

export function WsGeneralSettings({ workspace: initialWs }: WsGeneralSettingsProps) {
  const { getAccessToken } = useAccessToken();
  const router = useRouter();
  const [ws, setWs] = useState(initialWs);
  const [name, setName] = useState(ws.name);
  const [saving, setSaving] = useState(false);
  const [archiveOpen, setArchiveOpen] = useState(false);
  const [archiveConfirm, setArchiveConfirm] = useState("");
  const [archiving, setArchiving] = useState(false);

  async function handleSaveName() {
    if (!name.trim() || name === ws.name) return;
    setSaving(true);
    try {
      const token = await getAccessToken();
      if (!token) return;
      const api = createApiClient(token);
      const updated = await api.patch<WorkspaceDetail>(
        `/v1/workspaces/${ws.id}/details`,
        { name: name.trim() },
      );
      toast.success("Workspace name updated");
      setWs(updated);
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to update name",
      );
    } finally {
      setSaving(false);
    }
  }

  async function handleArchive() {
    setArchiving(true);
    try {
      const token = await getAccessToken();
      if (!token) return;
      const api = createApiClient(token);
      await api.patch(`/v1/workspaces/${ws.id}/details`, {
        status: "archived",
      });
      toast.success("Workspace archived");
      router.push("/dashboard");
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to archive",
      );
    } finally {
      setArchiving(false);
    }
  }

  return (
    <div className="space-y-8">
      {/* Name editor */}
      <div className="rounded-lg border border-border p-4">
        <label className="block text-sm font-medium mb-1.5">
          Workspace Name
        </label>
        <div className="flex gap-3">
          <Input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={saving}
            className="flex-1"
          />
          <Button
            onClick={handleSaveName}
            disabled={saving || !name.trim() || name === ws.name}
            size="sm"
          >
            {saving && (
              <Loader2
                data-icon="inline-start"
                className="size-4 animate-spin"
              />
            )}
            Save
          </Button>
        </div>
        <p className="mt-1.5 text-xs text-muted-foreground">
          Slug:{" "}
          <code className="font-[family-name:var(--font-mono)]">
            {ws.slug}
          </code>
        </p>
      </div>

      {/* Workspace ID (for API/CLI) */}
      <div className="rounded-lg border border-border p-4">
        <label className="block text-sm font-medium mb-1.5">
          Workspace ID
        </label>
        <code className="text-xs font-[family-name:var(--font-mono)] text-muted-foreground select-all">
          {ws.id}
        </code>
        <p className="mt-1.5 text-xs text-muted-foreground">
          Use this ID for API calls and CLI commands.
        </p>
      </div>

      {/* Members link */}
      <div className="rounded-lg border border-border p-4">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-sm font-medium">Members</h3>
            <p className="text-xs text-muted-foreground mt-0.5">
              Manage who has access to this workspace.
            </p>
          </div>
          <Link href={`/workspaces/${ws.id}/settings/members`}>
            <Button variant="outline" size="sm">
              Manage Members
            </Button>
          </Link>
        </div>
      </div>

      {/* Danger zone */}
      <div className="rounded-lg border border-destructive/20 p-4">
        <h3 className="text-sm font-medium text-destructive mb-1">
          Danger Zone
        </h3>
        <p className="text-xs text-muted-foreground mb-3">
          Archiving this workspace will remove all member access. This action
          cannot be easily undone.
        </p>
        <Dialog
          open={archiveOpen}
          onOpenChange={(next) => {
            setArchiveOpen(next);
            if (next) setArchiveConfirm("");
          }}
        >
          <DialogTrigger render={<Button variant="destructive" size="sm" />}>
            Archive Workspace
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Archive Workspace</DialogTitle>
              <DialogDescription>
                This will archive the workspace and remove all member access.
              </DialogDescription>
            </DialogHeader>
            <div className="rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2 text-xs text-destructive flex items-center gap-2">
              <AlertTriangle className="size-4 shrink-0" />
              This action cascades to all workspace memberships.
            </div>
            <div>
              <label className="block text-sm font-medium mb-1.5">
                Type <strong>{ws.name}</strong> to confirm
              </label>
              <Input
                type="text"
                value={archiveConfirm}
                onChange={(e) => setArchiveConfirm(e.target.value)}
                disabled={archiving}
                placeholder={ws.name}
              />
            </div>
            <DialogFooter>
              <Button
                variant="destructive"
                onClick={handleArchive}
                disabled={archiving || archiveConfirm !== ws.name}
              >
                {archiving && (
                  <Loader2
                    data-icon="inline-start"
                    className="size-4 animate-spin"
                  />
                )}
                {archiving ? "Archiving..." : "Yes, archive"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </div>
  );
}
