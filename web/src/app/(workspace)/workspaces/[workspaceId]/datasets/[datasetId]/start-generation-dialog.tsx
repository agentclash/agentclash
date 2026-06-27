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
  ProviderConnectionModel,
  ProviderAccount,
} from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  readStoredGenerationJobIds,
  storeGenerationJobId,
} from "../dataset-ui-shared";
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
  const modelRequestRef = useRef(0);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [providerAccounts, setProviderAccounts] = useState<ProviderAccount[]>(
    [],
  );
  const [models, setModels] = useState<ProviderConnectionModel[]>([]);
  const [loadingModels, setLoadingModels] = useState(false);
  const [strategy, setStrategy] = useState<
    "self_instruct" | "agentic_self_instruct"
  >("self_instruct");
  const [providerAccountId, setProviderAccountId] = useState("");
  const [model, setModel] = useState("");
  const [judgeProviderAccountId, setJudgeProviderAccountId] = useState("");
  const [judgeModel, setJudgeModel] = useState("");
  const [maxRoundsPerExample, setMaxRoundsPerExample] = useState("3");
  const [acceptanceMode, setAcceptanceMode] = useState<"judge" | "threshold">(
    "judge",
  );
  const [minGap, setMinGap] = useState("0.2");
  const [maxWeakScore, setMaxWeakScore] = useState("0.65");
  const [minStrongScore, setMinStrongScore] = useState("0.75");
  const [targetCount, setTargetCount] = useState("10");
  const [seedsTag, setSeedsTag] = useState("");
  const [createVersion, setCreateVersion] = useState(true);
  const [versionLabel, setVersionLabel] = useState("");
  const [job, setJob] = useState<DatasetGenerationJob | null>(null);
  const [recentJobs, setRecentJobs] = useState<DatasetGenerationJob[]>([]);
  const [loadingHistory, setLoadingHistory] = useState(false);

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
      const accounts = await api.paginated<ProviderAccount>(
        `/v1/workspaces/${workspaceId}/provider-accounts`,
        { limit: 100 },
      );
      setProviderAccounts(
        accounts.items.filter((a) => a.status === "active"),
      );
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to load generation options",
      );
    } finally {
      setLoading(false);
    }
  }, [getAccessToken, workspaceId]);

  const loadModels = useCallback(
    async (accountId: string) => {
      const requestId = ++modelRequestRef.current;
      setLoadingModels(true);
      setModels([]);
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const res = await api.get<{ items: ProviderConnectionModel[] }>(
          `/v1/provider-accounts/${accountId}/models`,
        );
        if (modelRequestRef.current === requestId) setModels(res.items);
      } catch {
        // Live model list is optional — fall back to free-form model entry.
        if (modelRequestRef.current === requestId) setModels([]);
      } finally {
        if (modelRequestRef.current === requestId) setLoadingModels(false);
      }
    },
    [getAccessToken],
  );

  function handleProviderAccountChange(accountId: string) {
    setProviderAccountId(accountId);
    setModel("");
    setModels([]);
    if (accountId) void loadModels(accountId);
    else {
      modelRequestRef.current += 1;
      setLoadingModels(false);
    }
  }

  const loadRecentJobs = useCallback(async () => {
    const ids = readStoredGenerationJobIds(datasetId);
    if (ids.length === 0) {
      setRecentJobs([]);
      return;
    }
    setLoadingHistory(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const jobs = await Promise.all(
        ids.map(async (jobId) => {
          try {
            return await getDatasetGenerationJob(
              api,
              workspaceId,
              datasetId,
              jobId,
            );
          } catch {
            return null;
          }
        }),
      );
      setRecentJobs(
        jobs.filter((item): item is DatasetGenerationJob => item != null),
      );
    } finally {
      setLoadingHistory(false);
    }
  }, [datasetId, getAccessToken, workspaceId]);

  useEffect(() => {
    if (open) {
      void loadOptions();
      void loadRecentJobs();
      setJob(null);
    } else {
      stopPolling();
    }
    return stopPolling;
  }, [loadOptions, loadRecentJobs, open, stopPolling]);

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
    if (!providerAccountId || !model.trim()) {
      toast.error("Select a provider account and model");
      return;
    }
    if (!Number.isInteger(count) || count < 1 || count > 100) {
      toast.error("Target count must be between 1 and 100");
      return;
    }
    if (strategy === "agentic_self_instruct") {
      const rounds = Number(maxRoundsPerExample);
      if (!judgeProviderAccountId || !judgeModel.trim()) {
        toast.error("Select a judge provider account and model");
        return;
      }
      if (!Number.isInteger(rounds) || rounds < 1 || rounds > 15) {
        toast.error("Max rounds per example must be between 1 and 15");
        return;
      }
      if (acceptanceMode === "threshold") {
        const thresholdValues = [
          Number(minGap),
          Number(maxWeakScore),
          Number(minStrongScore),
        ];
        if (thresholdValues.some((value) => !Number.isFinite(value))) {
          toast.error("Threshold values must be valid numbers");
          return;
        }
        if (thresholdValues.some((value) => value < 0 || value > 1)) {
          toast.error("Threshold values must be between 0 and 1");
          return;
        }
      }
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
          strategy,
          target_count: count,
          provider_account_id: providerAccountId,
          model: model.trim(),
          seeds_tag: seedsTag.trim() || undefined,
          create_version: createVersion,
          version_label: versionLabel.trim() || undefined,
          judge_provider_account_id:
            strategy === "agentic_self_instruct"
              ? judgeProviderAccountId
              : undefined,
          judge_model:
            strategy === "agentic_self_instruct"
              ? judgeModel.trim()
              : undefined,
          max_rounds_per_example:
            strategy === "agentic_self_instruct"
              ? Number(maxRoundsPerExample)
              : undefined,
          acceptance_mode:
            strategy === "agentic_self_instruct" ? acceptanceMode : undefined,
          min_gap:
            strategy === "agentic_self_instruct" &&
            acceptanceMode === "threshold"
              ? Number(minGap)
              : undefined,
          max_weak_score:
            strategy === "agentic_self_instruct" &&
            acceptanceMode === "threshold"
              ? Number(maxWeakScore)
              : undefined,
          min_strong_score:
            strategy === "agentic_self_instruct" &&
            acceptanceMode === "threshold"
              ? Number(minStrongScore)
              : undefined,
        },
      );
      setJob(queued);
      storeGenerationJobId(datasetId, queued.id);
      void loadRecentJobs();
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
            Generate examples from seeds, with optional agentic judge filtering.
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
            {recentJobs.length > 0 ? (
              <div className="rounded-lg border border-border p-3">
                <p className="text-sm font-medium">Recent jobs</p>
                <div className="mt-2 space-y-2">
                  {loadingHistory ? (
                    <Loader2 className="size-4 animate-spin text-muted-foreground" />
                  ) : (
                    recentJobs.map((recentJob) => (
                      <button
                        key={recentJob.id}
                        type="button"
                        onClick={() => {
                          setJob(recentJob);
                          if (
                            recentJob.status === "queued" ||
                            recentJob.status === "running"
                          ) {
                            startPolling(recentJob.id);
                          }
                        }}
                        className="flex w-full items-center justify-between rounded-md border border-border/70 px-3 py-2 text-left text-sm hover:bg-muted/30"
                      >
                        <span>{recentJob.id.slice(0, 8)}</span>
                        <Badge variant="secondary">{recentJob.status}</Badge>
                      </button>
                    ))
                  )}
                </div>
              </div>
            ) : null}
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Generation mode
              </label>
              <select
                value={strategy}
                onChange={(e) =>
                  setStrategy(
                    e.target.value as
                      | "self_instruct"
                      | "agentic_self_instruct",
                  )
                }
                className={inputClass}
              >
                <option value="self_instruct">Fast Self-Instruct</option>
                <option value="agentic_self_instruct">
                  Agentic Self-Instruct
                </option>
              </select>
            </div>
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Provider account
              </label>
              <select
                value={providerAccountId}
                onChange={(e) => handleProviderAccountChange(e.target.value)}
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
                Model
              </label>
              {models.length > 0 ? (
                <select
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                  className={inputClass}
                  disabled={loadingModels}
                >
                  <option value="">Select model...</option>
                  {models.map((m) => (
                    <option key={m.id} value={m.id}>
                      {m.display_name} ({m.id})
                    </option>
                  ))}
                </select>
              ) : (
                <input
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                  placeholder={
                    loadingModels
                      ? "Loading models..."
                      : !providerAccountId
                        ? "Select a provider account first"
                        : "e.g. gpt-4.1, claude-sonnet-4-6"
                  }
                  disabled={loadingModels || !providerAccountId}
                  className={inputClass}
                />
              )}
            </div>
            {strategy === "agentic_self_instruct" ? (
              <div className="space-y-4 rounded-lg border border-border p-3">
                <div>
                  <label className="mb-1.5 block text-sm font-medium">
                    Judge provider account
                  </label>
                  <select
                    value={judgeProviderAccountId}
                    onChange={(e) => setJudgeProviderAccountId(e.target.value)}
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
                    Judge model
                  </label>
                  <input
                    value={judgeModel}
                    onChange={(e) => setJudgeModel(e.target.value)}
                    placeholder="e.g. gpt-4.1, claude-sonnet-4-6"
                    disabled={!judgeProviderAccountId}
                    className={inputClass}
                  />
                </div>
                <div>
                  <label className="mb-1.5 block text-sm font-medium">
                    Max rounds per example
                  </label>
                  <input
                    type="number"
                    min={1}
                    max={15}
                    value={maxRoundsPerExample}
                    onChange={(e) => setMaxRoundsPerExample(e.target.value)}
                    className={inputClass}
                  />
                </div>
                <div>
                  <label className="mb-1.5 block text-sm font-medium">
                    Acceptance mode
                  </label>
                  <select
                    value={acceptanceMode}
                    onChange={(e) =>
                      setAcceptanceMode(e.target.value as "judge" | "threshold")
                    }
                    className={inputClass}
                  >
                    <option value="judge">Judge</option>
                    <option value="threshold">Threshold</option>
                  </select>
                </div>
                {acceptanceMode === "threshold" ? (
                  <div className="grid gap-3 sm:grid-cols-3">
                    <div>
                      <label className="mb-1.5 block text-sm font-medium">
                        Min gap
                      </label>
                      <input
                        type="number"
                        min={0}
                        max={1}
                        step={0.01}
                        value={minGap}
                        onChange={(e) => setMinGap(e.target.value)}
                        className={inputClass}
                      />
                    </div>
                    <div>
                      <label className="mb-1.5 block text-sm font-medium">
                        Max weak
                      </label>
                      <input
                        type="number"
                        min={0}
                        max={1}
                        step={0.01}
                        value={maxWeakScore}
                        onChange={(e) => setMaxWeakScore(e.target.value)}
                        className={inputClass}
                      />
                    </div>
                    <div>
                      <label className="mb-1.5 block text-sm font-medium">
                        Min strong
                      </label>
                      <input
                        type="number"
                        min={0}
                        max={1}
                        step={0.01}
                        value={minStrongScore}
                        onChange={(e) => setMinStrongScore(e.target.value)}
                        className={inputClass}
                      />
                    </div>
                  </div>
                ) : null}
              </div>
            ) : null}
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
