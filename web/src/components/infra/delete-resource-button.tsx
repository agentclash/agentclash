"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { useApiMutator, type ApiQueryKey } from "@/lib/api/swr";
import { ApiError } from "@/lib/api/errors";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { toast } from "sonner";
import { Loader2, Trash2 } from "lucide-react";

interface DeleteResourceButtonProps {
  endpoint: string;
  resourceName: string;
  invalidateKeys?: ApiQueryKey[];
}

export function DeleteResourceButton({
  endpoint,
  resourceName,
  invalidateKeys,
}: DeleteResourceButtonProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const { mutateMany } = useApiMutator();
  const [open, setOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  async function handleDelete() {
    setDeleting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await api.del(endpoint);
      toast.success(`${resourceName} deleted`);
      setOpen(false);
      if (invalidateKeys && invalidateKeys.length > 0) {
        await mutateMany(invalidateKeys);
      } else {
        router.refresh();
      }
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : `Failed to delete ${resourceName}`,
      );
    } finally {
      setDeleting(false);
    }
  }

  return (
    <>
      <Button
        variant="ghost"
        size="icon-xs"
        onClick={() => setOpen(true)}
        aria-label={`Delete ${resourceName}`}
      >
        <Trash2 className="size-3.5 text-muted-foreground hover:text-destructive" />
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete {resourceName}</DialogTitle>
            <DialogDescription>
              This will archive the {resourceName.toLowerCase()}. It will no longer appear in lists or be usable in new runs.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)} disabled={deleting}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleDelete} disabled={deleting}>
              {deleting ? <Loader2 className="size-4 animate-spin" /> : "Delete"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
