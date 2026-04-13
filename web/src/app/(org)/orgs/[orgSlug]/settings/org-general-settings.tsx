"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { Organization } from "@/lib/api/types";
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

interface OrgGeneralSettingsProps {
  org: Organization;
  orgSlug: string;
  onOrgUpdated?: (org: Organization) => void;
}

export function OrgGeneralSettings({ org, orgSlug, onOrgUpdated }: OrgGeneralSettingsProps) {
  const { getAccessToken } = useAccessToken();
  const router = useRouter();
  const [name, setName] = useState(org.name);
  const [saving, setSaving] = useState(false);
  const [archiveOpen, setArchiveOpen] = useState(false);
  const [archiveConfirm, setArchiveConfirm] = useState("");
  const [archiving, setArchiving] = useState(false);

  async function handleSaveName() {
    if (!name.trim() || name === org.name) return;
    setSaving(true);
    try {
      const token = await getAccessToken();
      if (!token) return;
      const api = createApiClient(token);
      const updated = await api.patch<Organization>(`/v1/organizations/${org.id}`, { name: name.trim() });
      toast.success("Organization name updated");
      onOrgUpdated?.(updated);
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
      await api.patch(`/v1/organizations/${org.id}`, { status: "archived" });
      toast.success("Organization archived");
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
          Organization Name
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
            disabled={saving || !name.trim() || name === org.name}
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
          Slug: <code className="font-[family-name:var(--font-mono)]">{orgSlug}</code>
        </p>
      </div>

      {/* Danger zone */}
      <div className="rounded-lg border border-destructive/20 p-4">
        <h3 className="text-sm font-medium text-destructive mb-1">
          Danger Zone
        </h3>
        <p className="text-xs text-muted-foreground mb-3">
          Archiving this organization will cascade to all workspaces and
          memberships. This action cannot be easily undone.
        </p>
        <Dialog
          open={archiveOpen}
          onOpenChange={(next) => {
            setArchiveOpen(next);
            if (next) setArchiveConfirm("");
          }}
        >
          <DialogTrigger render={<Button variant="destructive" size="sm" />}>
            Archive Organization
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Archive Organization</DialogTitle>
              <DialogDescription>
                This will archive all workspaces and remove all member access.
                This action cannot be easily undone.
              </DialogDescription>
            </DialogHeader>
            <div className="rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2 text-xs text-destructive flex items-center gap-2">
              <AlertTriangle className="size-4 shrink-0" />
              This action cascades to all workspaces and memberships.
            </div>
            <div>
              <label className="block text-sm font-medium mb-1.5">
                Type <strong>{org.name}</strong> to confirm
              </label>
              <Input
                type="text"
                value={archiveConfirm}
                onChange={(e) => setArchiveConfirm(e.target.value)}
                disabled={archiving}
                placeholder={org.name}
              />
            </div>
            <DialogFooter>
              <Button
                variant="destructive"
                onClick={handleArchive}
                disabled={archiving || archiveConfirm !== org.name}
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
