"use client";

import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { Loader2, Sparkles } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";

import { instantiateCatalogPack } from "@/components/challenge-packs/lib/api";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api/errors";

/**
 * Clones a catalog template into the workspace and routes to the new pack so
 * the user can run it. Idempotent on the backend — clicking twice reopens the
 * existing copy instead of erroring.
 */
export function UseTemplateButton({
  workspaceId,
  slug,
  className,
}: {
  workspaceId: string;
  slug: string;
  className?: string;
}) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [busy, setBusy] = useState(false);

  const onClick = async () => {
    setBusy(true);
    try {
      const token = await getAccessToken();
      const result = await instantiateCatalogPack(token, workspaceId, slug);
      toast.success(
        result.already_existed
          ? "Already in your workspace — opening it"
          : "Template added to your workspace",
      );
      router.push(`/workspaces/${workspaceId}/challenge-packs/${result.challenge_pack_id}`);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Couldn't add this template");
      setBusy(false);
    }
  };

  return (
    <Button size="sm" onClick={onClick} disabled={busy} className={className}>
      {busy ? <Loader2 className="size-4 animate-spin" /> : <Sparkles className="size-4" />}
      Use template
    </Button>
  );
}
