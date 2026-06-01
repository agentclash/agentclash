"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2, Sparkles } from "lucide-react";

import {
  getDatasetGenerationJob,
  startDatasetGeneration,
} from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  DatasetGenerationJob,
  ModelAlias,
  ProviderAccount,
} from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
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

interface StartGenerationDialogProps {
  workspaceId: string;
  datasetId: string;
}

export function StartGenerationDialog({
  workspaceId,
  datasetId,
}: StartGenerationDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [providerAccounts, setProviderAccounts] = useState<ProviderAccount[]>(
    [],
  );
  const [modelAliases, setModelAliases] = useState<ModelAlias[]>([]);
  const [providerAccountId, setProviderAccountId] = useState("");
  const [modelAliasId, setModelAliasId] = useState("");
  const [targetCount, setTargetCount] = useState("10");
  const [seedsTag, setSeedsTag] = useState("");
  const [createVersion, setCreateVersion] = useState(true);
  const [versionLabel, setVersionLabel] = useState("");
  const [job, setJob] = useState<DatasetGenerationJob | null>(null);

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  const loadOptions = useCallback(async () => {
    setLoading(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const [accounts, aliases] = await Promise.all([
        api.paginated<ProviderAccount>(
          `/v1/workspaces/${workspaceId}/provider-accounts`,
          { limit: 100 },
        ),
        api.paginated<ModelAlias>(
          `/v1/workspaces/${workspaceId}/model-aliases`,
          { limit: 100 },
        ),
      ]);
      setProviderAccounts(
        accounts.items.filter((a) => a.status === "active"),
      );
      setModelAliases(aliases.items.filter((a) => a.status === "active"));
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to load generation options",
      );
    } finally {
      setLoading(false);
    }
  }, [getAccessToken, workspaceId]);

  useEffect(() => {
    if (open) {
      void loadOptions();
      setJob(null);
    } else {
      stopPolling();
    }
    return stopPolling;
  }, [open, loadOptions, stopPolling]);

  function startPolling(jobId: string) {
    stopPolling();
    pollRef.current = setInterval(async () => {
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const next = await getDatasetGenerationJob(
          api,
          workspaceId,
          datasetId,
          jobId,
        );
        setJob(next);
        if (next.status === "completed" || next.status === "failed") {
          stopPolling();
          if (next.status === "completed") {
            toast.success(
              `Generated ${next.accepted_count} examples (${next.rejected_count} rejected)`,
            );
            router.refresh();
          } else {
            toast.error(next.error_message ?? "Generation failed");
          }
        }
      } catch {
        stopPolling();
      }
    }, 3000);
  }

  async function handleStart() {
    const count = Number(targetCount);
    if (!providerAccountId || !modelAliasId) {
      toast.error("Select a provider account and model alias");
      return;
    }
    if (!Number.isInteger(count) || count < 1 || count > 100) {
      toast.error("Target count must be between 1 and 100");
      return;
    }

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const queued = await startDatasetGeneration(
        api,
        workspaceId,
        datasetId,
        {
          strategy: "self_instruct",
          target_count: count,
          provider_account_id: providerAccountId,
          model_alias_id: modelAliasId,
          seeds_tag: seedsTag.trim() || undefined,
          create_version: createVersion,
          version_label: versionLabel.trim() || undefined,
        },
      );
      setJob(queued);
      startPolling(queued.id);
      toast.success("Generation job queued");
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to start generation",
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" variant="outline" />}>
        <Sparkles data-icon="inline-start" className="size-4" />
        Generate
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Synthetic generation</DialogTitle>
          <DialogDescription>
            Self-instruct generation using a workspace model alias.
          </DialogDescription>
        </DialogHeader>
        {job ? (
          <div className="space-y-3 rounded-lg border border-border p-4 text-sm">
            <div className="flex items-center justify-between">
              <span className="font-medium">Job {job.id.slice(0, 8)}</span>
              <Badge
                variant={
                  job.status === "completed"
                    ? "default"
                    : job.status === "failed"
                      ? "destructive"
                      : "secondary"
                }
              >
                {job.status}
              </Badge>
            </div>
            <p className="text-muted-foreground">
              Accepted {job.accepted_count} / {job.target_count}
              {job.rejected_count > 0
                ? ` (${job.rejected_count} rejected)`
                : ""}
            </p>
            {(job.status === "queued" || job.status === "running") && (
              <div className="flex items-center gap-2 text-muted-foreground">
                <Loader2 className="size-4 animate-spin" />
                Polling for completion…
              </div>
            )}
          </div>
        ) : loading ? (
          <div className="flex justify-center py-8">
            <Loader2 className="size-6 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <div className="space-y-4">
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Provider account
              </label>
              <select
                value={providerAccountId}
                onChange={(e) => setProviderAccountId(e.target.value)}
                className={inputClass}
              >
                <option value="">Select account...</option>
                {providerAccounts.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name} ({a.provider_key})
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Model alias
              </label>
              <select
                value={modelAliasId}
                onChange={(e) => setModelAliasId(e.target.value)}
                className={inputClass}
              >
                <option value="">Select alias...</option>
                {modelAliases.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.display_name} ({a.alias_key})
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Target count
              </label>
              <input
                type="number"
                min={1}
                max={100}
                value={targetCount}
                onChange={(e) => setTargetCount(e.target.value)}
                className={inputClass}
              />
            </div>
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Seeds tag{" "}
                <span className="font-normal text-muted-foreground">
                  (optional)
                </span>
              </label>
              <input
                value={seedsTag}
                onChange={(e) => setSeedsTag(e.target.value)}
                className={inputClass}
              />
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={createVersion}
                onChange={(e) => setCreateVersion(e.target.checked)}
              />
              Create version snapshot when complete
            </label>
            {createVersion && (
              <div>
                <label className="mb-1.5 block text-sm font-medium">
                  Version label
                </label>
                <input
                  value={versionLabel}
                  onChange={(e) => setVersionLabel(e.target.value)}
                  className={inputClass}
                />
              </div>
            )}
          </div>
        )}
        <DialogFooter>
          {!job && (
            <Button onClick={handleStart} disabled={submitting || loading}>
              {submitting && (
                <Loader2 data-icon="inline-start" className="size-4 animate-spin" />
              )}
              Start generation
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
