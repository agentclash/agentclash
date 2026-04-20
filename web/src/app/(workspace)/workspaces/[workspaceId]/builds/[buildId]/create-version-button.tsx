"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { AgentBuildVersion } from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { toast } from "sonner";
import { Loader2, Plus, Sparkles } from "lucide-react";

import { guidedTemplates, versionPayloadFromTemplate } from "./versions/[versionId]/guided-authoring";

interface CreateVersionButtonProps {
  buildId: string;
  workspaceId: string;
}

export function CreateVersionButton({
  buildId,
  workspaceId,
}: CreateVersionButtonProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [creatingTemplateID, setCreatingTemplateID] = useState<string | null>(null);

  async function handleCreate(templateID: string) {
    setCreatingTemplateID(templateID);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const version = await api.post<AgentBuildVersion>(
        `/v1/agent-builds/${buildId}/versions`,
        versionPayloadFromTemplate(templateID),
      );
      toast.success(`Created version ${version.version_number}`);
      setOpen(false);
      router.push(
        `/workspaces/${workspaceId}/builds/${buildId}/versions/${version.id}`,
      );
    } catch (err) {
      if (err instanceof ApiError) {
        toast.error(err.message);
      } else {
        toast.error("Failed to create version");
      }
    } finally {
      setCreatingTemplateID(null);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          <Button size="sm">
            <Plus data-icon="inline-start" className="size-4" />
            New Version
          </Button>
        }
      />
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Start With a Guided Version Template</DialogTitle>
          <DialogDescription>
            Beginners should not have to invent a full spec sheet from scratch.
            Pick a starter, land in the guided editor, and drop to JSON only if
            you need the advanced layer.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-3 md:grid-cols-2">
          {guidedTemplates.map((template) => {
            const creating = creatingTemplateID === template.id;
            const isBlank = template.id === "blank";

            return (
              <button
                key={template.id}
                type="button"
                onClick={() => handleCreate(template.id)}
                disabled={creatingTemplateID !== null}
                className="rounded-xl border border-white/[0.08] bg-white/[0.02] p-4 text-left transition hover:border-white/[0.16] hover:bg-white/[0.04] disabled:cursor-not-allowed disabled:opacity-50"
              >
                <div className="flex items-center justify-between gap-3">
                  <div className="flex items-center gap-2">
                    {!isBlank ? <Sparkles className="size-4" /> : null}
                    <span className="text-sm font-medium">{template.name}</span>
                  </div>
                  {creating ? <Loader2 className="size-4 animate-spin" /> : null}
                </div>
                <p className="mt-2 text-sm text-muted-foreground">
                  {template.summary}
                </p>
                <p className="mt-3 text-xs uppercase tracking-[0.14em] text-muted-foreground">
                  {isBlank ? "Start blank" : "Use starter"}
                </p>
              </button>
            );
          })}
        </div>
      </DialogContent>
    </Dialog>
  );
}
