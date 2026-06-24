"use client";

import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { Loader2, Plus } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";

import { createDraft } from "@/components/eval-packs/lib/api";
import { emptyComposition } from "@/components/eval-packs/lib/draft";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api/errors";

export function NewPackButton({ workspaceId }: { workspaceId: string }) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [creating, setCreating] = useState(false);

  const onClick = async () => {
    setCreating(true);
    try {
      const token = await getAccessToken();
      const draft = await createDraft(token, workspaceId, {
        name: "Untitled pack",
        execution_mode: "native",
        composition: emptyComposition(),
      });
      router.push(`/workspaces/${workspaceId}/eval-packs/builder/${draft.id}`);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Couldn't start a new pack");
      setCreating(false);
    }
  };

  return (
    <Button size="sm" onClick={onClick} disabled={creating}>
      {creating ? <Loader2 className="size-4 animate-spin" /> : <Plus className="size-4" />}
      New pack
    </Button>
  );
}
