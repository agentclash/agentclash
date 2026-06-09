"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { ArrowUpRight, Loader2 } from "lucide-react";

import { promoteAgentTryoutToEval } from "@/lib/api/agent-tryouts";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { AgentTryout } from "@/lib/api/types";
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
  "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

export function PromoteTryoutDialog({
  workspaceId,
  tryout,
}: {
  workspaceId: string;
  tryout: AgentTryout;
}) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [title, setTitle] = useState("");

  const promotable = tryout.status === "completed";

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (submitting) return;

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token ?? undefined);
      await promoteAgentTryoutToEval(api, tryout.id, {
        target: "vibe_eval",
        title: title.trim() || undefined,
      });
      toast.success("Promoted to a Vibe Eval draft");
      setOpen(false);
      router.push(`/workspaces/${workspaceId}/eval-sessions`);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to promote");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={<Button size="sm" disabled={!promotable} />}
      >
        <ArrowUpRight data-icon="inline-start" className="size-4" />
        Promote to eval
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Promote to a repeatable eval</DialogTitle>
            <DialogDescription>
              Saves this tryout&apos;s task, input, and validators as a Vibe
              Eval draft so you can run it on every model and gate releases on
              it.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Title{" "}
                <span className="font-normal text-muted-foreground">
                  (optional)
                </span>
              </label>
              <input
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder={`Tryout: ${tryout.template_slug}`}
                className={inputClass}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setOpen(false)}
              disabled={submitting}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                "Promote"
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
