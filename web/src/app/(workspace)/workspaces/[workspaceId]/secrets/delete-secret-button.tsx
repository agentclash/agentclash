"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { Loader2, Trash2 } from "lucide-react";

interface DeleteSecretButtonProps {
  workspaceId: string;
  secretKey: string;
}

export function DeleteSecretButton({
  workspaceId,
  secretKey,
}: DeleteSecretButtonProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [deleting, setDeleting] = useState(false);

  async function handleDelete() {
    if (!confirm(`Delete secret "${secretKey}"? This cannot be undone.`)) return;

    setDeleting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await api.del(
        `/v1/workspaces/${workspaceId}/secrets/${encodeURIComponent(secretKey)}`,
      );
      toast.success(`Deleted "${secretKey}"`);
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError && err.code === "secret_not_found") {
        toast.error("Secret not found — it may have already been deleted");
      } else {
        toast.error("Failed to delete secret");
      }
    } finally {
      setDeleting(false);
    }
  }

  return (
    <Button
      variant="ghost"
      size="icon-sm"
      onClick={handleDelete}
      disabled={deleting}
      className="text-muted-foreground hover:text-destructive"
    >
      {deleting ? (
        <Loader2 className="size-3.5 animate-spin" />
      ) : (
        <Trash2 className="size-3.5" />
      )}
    </Button>
  );
}
