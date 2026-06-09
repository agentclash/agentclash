"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2, RotateCcw } from "lucide-react";

import { rerunAgentTryout } from "@/lib/api/agent-tryouts";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { AgentTryout, AgentTryoutModelPolicy } from "@/lib/api/types";
import { useApiMutator } from "@/lib/api/swr";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
import { tryoutIsActive } from "../status";
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

// Mirrors knownTryoutProviderKeys on the backend.
const PROVIDERS = [
  "openai",
  "anthropic",
  "gemini",
  "xai",
  "openrouter",
  "mistral",
];

export function RerunTryoutDialog({
  workspaceId,
  tryout,
}: {
  workspaceId: string;
  tryout: AgentTryout;
}) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const { mutateMany } = useApiMutator();
  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [mode, setMode] = useState<"hosted_default" | "explicit">(
    "hosted_default",
  );
  const [provider, setProvider] = useState(PROVIDERS[0]);
  const [model, setModel] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (submitting) return;

    let policy: AgentTryoutModelPolicy;
    if (mode === "hosted_default") {
      policy = { mode: "hosted_default", max_models: 1 };
    } else {
      if (!model.trim()) {
        toast.error("Model id is required");
        return;
      }
      policy = { models: [{ provider, model: model.trim() }] };
    }

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token ?? undefined);
      const rerun = await rerunAgentTryout(api, tryout.id, {
        selected_model_policy: policy,
      });
      toast.success("Rerun launched");
      setOpen(false);
      await mutateMany([workspaceResourceKeys.agentTryouts(workspaceId)]);
      router.push(`/workspaces/${workspaceId}/agent-tryouts/${rerun.id}`);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to rerun");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {/* Rerunning a tryout that is still queued/running just races it —
          wait for a terminal status, matching the Promote gating style. */}
      <DialogTrigger
        render={
          <Button
            size="sm"
            variant="outline"
            disabled={tryoutIsActive(tryout.status)}
          />
        }
      >
        <RotateCcw data-icon="inline-start" className="size-4" />
        Rerun
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Rerun with a different model</DialogTitle>
            <DialogDescription>
              Runs the same task and input again under a new model policy. The
              rerun is linked to this tryout so you can compare them
              side-by-side.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Model policy
              </label>
              <select
                value={mode}
                onChange={(e) =>
                  setMode(e.target.value as "hosted_default" | "explicit")
                }
                className={inputClass}
              >
                <option value="hosted_default">Hosted default</option>
                <option value="explicit">Pick a provider and model</option>
              </select>
            </div>
            {mode === "explicit" ? (
              <>
                <div>
                  <label className="mb-1.5 block text-sm font-medium">
                    Provider
                  </label>
                  <select
                    value={provider}
                    onChange={(e) => setProvider(e.target.value)}
                    className={inputClass}
                  >
                    {PROVIDERS.map((key) => (
                      <option key={key} value={key}>
                        {key}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="mb-1.5 block text-sm font-medium">
                    Model id
                  </label>
                  <input
                    value={model}
                    onChange={(e) => setModel(e.target.value)}
                    placeholder="e.g. claude-fable-5"
                    className={inputClass}
                  />
                </div>
              </>
            ) : null}
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
                "Rerun"
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
