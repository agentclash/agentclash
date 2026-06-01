"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Camera, Loader2 } from "lucide-react";

import { createDatasetVersion } from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";

const inputClass =
  "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

interface CreateVersionDialogProps {
  workspaceId: string;
  datasetId: string;
}

export function CreateVersionDialog({
  workspaceId,
  datasetId,
}: CreateVersionDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [label, setLabel] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function handleCreate() {
    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const version = await createDatasetVersion(
        api,
        workspaceId,
        datasetId,
        label.trim() ? { label: label.trim() } : {},
      );
      toast.success(`Snapshot v${version.version_number} created`);
      setOpen(false);
      setLabel("");
      router.refresh();
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to create version",
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" />}>
        <Camera data-icon="inline-start" className="size-4" />
        Snapshot version
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Snapshot version</DialogTitle>
          <DialogDescription>
            Pin the current active examples into an immutable version.
          </DialogDescription>
        </DialogHeader>
        <div>
          <label className="mb-1.5 block text-sm font-medium">
            Label{" "}
            <span className="font-normal text-muted-foreground">(optional)</span>
          </label>
          <input
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            placeholder="e.g. pre-launch baseline"
            className={inputClass}
          />
        </div>
        <DialogFooter>
          <Button onClick={handleCreate} disabled={submitting}>
            {submitting && (
              <Loader2 data-icon="inline-start" className="size-4 animate-spin" />
            )}
            Create snapshot
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
