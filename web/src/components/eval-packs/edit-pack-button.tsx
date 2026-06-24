"use client";

import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { Loader2, Pencil } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";

import { createDraftFromVersion } from "@/components/eval-packs/lib/api";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api/errors";

/**
 * Opens an already-published pack version in the visual builder by hydrating a
 * fresh draft from it (the server decompiles the manifest into an editable
 * composition). The universal "customize this pack" entry point — works for
 * library templates the user added, hand-built packs, anything runnable.
 */
export function EditPackInBuilderButton({
  workspaceId,
  versionId,
  packName,
}: {
  workspaceId: string;
  versionId: string;
  packName?: string;
}) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [busy, setBusy] = useState(false);

  const onClick = async () => {
    setBusy(true);
    try {
      const token = await getAccessToken();
      const draft = await createDraftFromVersion(token, workspaceId, {
        from_eval_pack_version_id: versionId,
        name: packName,
      });
      router.push(`/workspaces/${workspaceId}/eval-packs/builder/${draft.id}`);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Couldn't open this pack in the builder");
      setBusy(false);
    }
  };

  return (
    <Button size="sm" variant="outline" onClick={onClick} disabled={busy}>
      {busy ? <Loader2 className="size-4 animate-spin" /> : <Pencil className="size-4" />}
      Edit in builder
    </Button>
  );
}
