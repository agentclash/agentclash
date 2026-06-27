"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2, Sparkles } from "lucide-react";

import {
  getDatasetGenerationJob,
  listDatasetGenerationRejections,
  startDatasetGeneration,
} from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  AgentDeployment,
  ChallengePack,
  ChallengePackVersion,
  DatasetGenerationJob,
  DatasetGenerationRejection,
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
  const [packs, setPacks] = useState<ChallengePack[]>([]);
  const [packVersions, setPackVersions] = useState<ChallengePackVersion[]>([]);
  const [deployments, setDeployments] = useState<AgentDeployment[]>([]);
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
  const [solverMode, setSolverMode] = useState<"judge_only" | "direct_provider">(
    "judge_only",
  );
  const [weakProviderAccountId, setWeakProviderAccountId] = useState("");
  const [weakModel, setWeakModel] = useState("");
  const [strongProviderAccountId, setStrongProviderAccountId] = useState("");
  const [strongModel, setStrongModel] = useState("");
  const [weakRollouts, setWeakRollouts] = useState("1");
  const [strongRollouts, setStrongRollouts] = useState("1");
  const [weakDeploymentId, setWeakDeploymentId] = useState("");
  const [strongDeploymentId, setStrongDeploymentId] = useState("");
  const [packId, setPackId] = useState("");
  const [packVersionId, setPackVersionId] = useState("");
  const [challengeKey, setChallengeKey] = useState("");
  const [fieldMappingJson, setFieldMappingJson] = useState("");
  const [fieldMappingError, setFieldMappingError] = useState<string>();
  const [targetCount, setTargetCount] = useState("10");
  const [seedsTag, setSeedsTag] = useState("");
  const [createVersion, setCreateVersion] = useState(true);
  const [versionLabel, setVersionLabel] = useState("");
  const [job, setJob] = useState<DatasetGenerationJob | null>(null);
  const [jobRejections, setJobRejections] = useState<
    DatasetGenerationRejection[]
  >([]);
  const [loadingJobRejections, setLoadingJobRejections] = useState(false);
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
      const [accounts, packsRes, deploymentsRes] = await Promise.all([
        api.paginated<ProviderAccount>(
          `/v1/workspaces/${workspaceId}/provider-accounts`,
          { limit: 100 },
        ),
        api.get<{ items: ChallengePack[] }>(
          `/v1/workspaces/${workspaceId}/challenge-packs`,
        ),
        api.get<{ items: AgentDeployment[] }>(
          `/v1/workspaces/${workspaceId}/agent-deployments`,
        ),
      ]);
      setProviderAccounts(
        accounts.items.filter((a) => a.status === "active"),
      );
      setPacks(packsRes.items);
      setDeployments(
        deploymentsRes.items.filter((item) => item.status === "active"),
      );
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to load generation options",
      );
    } finally {
      setLoading(false);
    }
  }, [getAccessToken, workspaceId]);

  useEffect(() => {
    if (!packId) {
      setPackVersions([]);
      setPackVersionId("");
      return;
    }
    const pack = packs.find((item) => item.id === packId);
    const runnable = (pack?.versions ?? []).filter(
      (item) => item.lifecycle_status === "runnable",
    );
    setPackVersions(runnable);
    setPackVersionId(runnable.length > 0 ? runnable[runnable.length - 1].id : "");
  }, [packId, packs]);

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

  const loadJobRejections = useCallback(
    async (jobId: string) => {
      setLoadingJobRejections(true);
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const result = await listDatasetGenerationRejections(
          api,
          workspaceId,
          datasetId,
          jobId,
        );
        setJobRejections(result.items);
      } catch {
        setJobRejections([]);
      } finally {
        setLoadingJobRejections(false);
      }
    },
    [datasetId, getAccessToken, workspaceId],
  );

  useEffect(() => {
    if (open) {
      void loadOptions();
      void loadRecentJobs();
      setJob(null);
      setJobRejections([]);
    } else {
      stopPolling();
    }
    return stopPolling;
  }, [loadOptions, loadRecentJobs, open, stopPolling]);

  useEffect(() => {
    const selectedJobId = job?.id;
    if (!open || !selectedJobId) {
      setJobRejections([]);
      return;
    }
    void loadJobRejections(selectedJobId);
  }, [job?.id, loadJobRejections, open]);

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
          void loadJobRejections(next.id);
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
      if (solverMode === "direct_provider") {
        const rolloutValues = [Number(weakRollouts), Number(strongRollouts)];
        if (
          !weakProviderAccountId ||
          !weakModel.trim() ||
          !strongProviderAccountId ||
          !strongModel.trim()
        ) {
          toast.error("Select weak and strong solver provider accounts and models");
          return;
        }
        if (
          rolloutValues.some(
            (value) => !Number.isInteger(value) || value < 1 || value > 5,
          )
        ) {
          toast.error("Solver rollouts must be between 1 and 5");
          return;
        }
      }
    }

    const hasDeploymentContext =
      strategy === "agentic_self_instruct" &&
      (Boolean(weakDeploymentId) ||
        Boolean(strongDeploymentId) ||
        Boolean(packVersionId) ||
        Boolean(challengeKey.trim()) ||
        Boolean(fieldMappingJson.trim()));
    let fieldMapping: Record<string, unknown> | undefined;
    if (hasDeploymentContext) {
      if (
        !weakDeploymentId ||
        !strongDeploymentId ||
        !packVersionId ||
        !challengeKey.trim()
      ) {
        toast.error("Select weak and strong deployments, pack version, and challenge key");
        return;
      }
      if (fieldMappingJson.trim()) {
        try {
          const parsed = JSON.parse(fieldMappingJson) as unknown;
          if (
            parsed == null ||
            typeof parsed !== "object" ||
            Array.isArray(parsed)
          ) {
            setFieldMappingError("Mapping must be a JSON object");
            return;
          }
          fieldMapping = parsed as Record<string, unknown>;
          setFieldMappingError(undefined);
        } catch {
          setFieldMappingError("Invalid JSON");
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
          solver_mode:
            strategy === "agentic_self_instruct" ? solverMode : undefined,
          weak_provider_account_id:
            strategy === "agentic_self_instruct" &&
            solverMode === "direct_provider"
              ? weakProviderAccountId
              : undefined,
          weak_model:
            strategy === "agentic_self_instruct" &&
            solverMode === "direct_provider"
              ? weakModel.trim()
              : undefined,
          strong_provider_account_id:
            strategy === "agentic_self_instruct" &&
            solverMode === "direct_provider"
              ? strongProviderAccountId
              : undefined,
          strong_model:
            strategy === "agentic_self_instruct" &&
            solverMode === "direct_provider"
              ? strongModel.trim()
              : undefined,
          weak_rollouts:
            strategy === "agentic_self_instruct" &&
            solverMode === "direct_provider"
              ? Number(weakRollouts)
              : undefined,
          strong_rollouts:
            strategy === "agentic_self_instruct" &&
            solverMode === "direct_provider"
              ? Number(strongRollouts)
              : undefined,
          weak_deployment_id:
            strategy === "agentic_self_instruct" && hasDeploymentContext
              ? weakDeploymentId
              : undefined,
          strong_deployment_id:
            strategy === "agentic_self_instruct" && hasDeploymentContext
              ? strongDeploymentId
              : undefined,
          challenge_pack_version_id:
            strategy === "agentic_self_instruct" && hasDeploymentContext
              ? packVersionId
              : undefined,
          challenge_key:
            strategy === "agentic_self_instruct" && hasDeploymentContext
              ? challengeKey.trim()
              : undefined,
          field_mapping:
            strategy === "agentic_self_instruct" && hasDeploymentContext
              ? fieldMapping
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
      <DialogContent className="sm:max-w-2xl">
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
            {generationSummaryStats(job).length > 0 ? (
              <div className="grid gap-2 sm:grid-cols-3">
                {generationSummaryStats(job).map((stat) => (
                  <div
                    key={stat.label}
                    className="rounded-md border border-border/70 px-2 py-1.5"
                  >
                    <p className="text-[11px] font-medium uppercase text-muted-foreground">
                      {stat.label}
                    </p>
                    <p className="mt-0.5 truncate text-sm">{stat.value}</p>
                  </div>
                ))}
              </div>
            ) : null}
            {job.rejected_count > 0 ? (
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-muted-foreground">
                  {loadingJobRejections ? (
                    <Loader2 className="size-3.5 animate-spin" />
                  ) : null}
                  <span>Rejections</span>
                </div>
                {jobRejections.length > 0 ? (
                  <div className="max-h-36 space-y-1 overflow-y-auto">
                    {jobRejections.slice(0, 5).map((rejection) => (
                      <div
                        key={rejection.id}
                        className="rounded-md border border-border/70 px-2 py-1.5"
                      >
                        <div className="flex items-center justify-between gap-2">
                          <span className="font-medium">
                            {formatReasonCode(rejection.reason_code)}
                          </span>
                          <span className="text-xs text-muted-foreground">
                            {new Date(rejection.created_at).toLocaleTimeString()}
                          </span>
                        </div>
                        {rejection.reason_detail ? (
                          <p className="mt-1 line-clamp-2 text-muted-foreground">
                            {rejection.reason_detail}
                          </p>
                        ) : null}
                      </div>
                    ))}
                  </div>
                ) : loadingJobRejections ? null : (
                  <p className="text-muted-foreground">
                    No rejection records yet.
                  </p>
                )}
              </div>
            ) : null}
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
          <div className="max-h-[65vh] space-y-4 overflow-y-auto pr-1">
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
                        className="flex w-full items-center justify-between gap-3 rounded-md border border-border/70 px-3 py-2 text-left text-sm hover:bg-muted/30"
                      >
                        <span>
                          <span className="font-medium">
                            {recentJob.id.slice(0, 8)}
                          </span>
                          <span className="mt-0.5 block text-xs text-muted-foreground">
                            {recentJob.accepted_count}/{recentJob.target_count} accepted
                            {recentJob.rejected_count > 0
                              ? `, ${recentJob.rejected_count} rejected`
                              : ""}
                            {topRejectionReason(recentJob)
                              ? `, ${topRejectionReason(recentJob)}`
                              : ""}
                          </span>
                        </span>
                        <Badge variant="secondary" className="shrink-0">
                          {recentJob.status}
                        </Badge>
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
                    Solver mode
                  </label>
                  <select
                    value={solverMode}
                    onChange={(e) =>
                      setSolverMode(
                        e.target.value as "judge_only" | "direct_provider",
                      )
                    }
                    className={inputClass}
                  >
                    <option value="judge_only">Judge only</option>
                    <option value="direct_provider">Direct weak/strong</option>
                  </select>
                </div>
                {solverMode === "direct_provider" ? (
                  <div className="grid gap-3 sm:grid-cols-2">
                    <div className="space-y-3">
                      <div>
                        <label className="mb-1.5 block text-sm font-medium">
                          Weak provider account
                        </label>
                        <select
                          value={weakProviderAccountId}
                          onChange={(e) =>
                            setWeakProviderAccountId(e.target.value)
                          }
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
                          Weak model
                        </label>
                        <input
                          value={weakModel}
                          onChange={(e) => setWeakModel(e.target.value)}
                          placeholder="e.g. gpt-4.1-nano"
                          disabled={!weakProviderAccountId}
                          className={inputClass}
                        />
                      </div>
                      <div>
                        <label className="mb-1.5 block text-sm font-medium">
                          Weak rollouts
                        </label>
                        <input
                          type="number"
                          min={1}
                          max={5}
                          value={weakRollouts}
                          onChange={(e) => setWeakRollouts(e.target.value)}
                          className={inputClass}
                        />
                      </div>
                    </div>
                    <div className="space-y-3">
                      <div>
                        <label className="mb-1.5 block text-sm font-medium">
                          Strong provider account
                        </label>
                        <select
                          value={strongProviderAccountId}
                          onChange={(e) =>
                            setStrongProviderAccountId(e.target.value)
                          }
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
                          Strong model
                        </label>
                        <input
                          value={strongModel}
                          onChange={(e) => setStrongModel(e.target.value)}
                          placeholder="e.g. gpt-4.1"
                          disabled={!strongProviderAccountId}
                          className={inputClass}
                        />
                      </div>
                      <div>
                        <label className="mb-1.5 block text-sm font-medium">
                          Strong rollouts
                        </label>
                        <input
                          type="number"
                          min={1}
                          max={5}
                          value={strongRollouts}
                          onChange={(e) => setStrongRollouts(e.target.value)}
                          className={inputClass}
                        />
                      </div>
                    </div>
                  </div>
                ) : null}
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
                <div className="space-y-3 rounded-md border border-border/70 p-3">
                  <div className="grid gap-3 sm:grid-cols-2">
                    <div>
                      <label className="mb-1.5 block text-sm font-medium">
                        Weak deployment
                      </label>
                      <select
                        value={weakDeploymentId}
                        onChange={(e) => setWeakDeploymentId(e.target.value)}
                        className={inputClass}
                      >
                        <option value="">Select deployment...</option>
                        {deployments.map((deployment) => (
                          <option key={deployment.id} value={deployment.id}>
                            {deployment.name}
                          </option>
                        ))}
                      </select>
                    </div>
                    <div>
                      <label className="mb-1.5 block text-sm font-medium">
                        Strong deployment
                      </label>
                      <select
                        value={strongDeploymentId}
                        onChange={(e) => setStrongDeploymentId(e.target.value)}
                        className={inputClass}
                      >
                        <option value="">Select deployment...</option>
                        {deployments.map((deployment) => (
                          <option key={deployment.id} value={deployment.id}>
                            {deployment.name}
                          </option>
                        ))}
                      </select>
                    </div>
                  </div>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <div>
                      <label className="mb-1.5 block text-sm font-medium">
                        Challenge pack
                      </label>
                      <select
                        value={packId}
                        onChange={(e) => setPackId(e.target.value)}
                        className={inputClass}
                      >
                        <option value="">Select pack...</option>
                        {packs.map((pack) => (
                          <option key={pack.id} value={pack.id}>
                            {pack.name}
                          </option>
                        ))}
                      </select>
                    </div>
                    <div>
                      <label className="mb-1.5 block text-sm font-medium">
                        Pack version
                      </label>
                      <select
                        value={packVersionId}
                        onChange={(e) => setPackVersionId(e.target.value)}
                        disabled={!packId}
                        className={inputClass}
                      >
                        <option value="">Select version...</option>
                        {packVersions.map((version) => (
                          <option key={version.id} value={version.id}>
                            v{version.version_number}
                          </option>
                        ))}
                      </select>
                    </div>
                  </div>
                  <div>
                    <label className="mb-1.5 block text-sm font-medium">
                      Challenge key
                    </label>
                    <input
                      value={challengeKey}
                      onChange={(e) => setChallengeKey(e.target.value)}
                      placeholder="e.g. refund-recovery"
                      className={inputClass}
                    />
                  </div>
                  <div>
                    <label className="mb-1.5 block text-sm font-medium">
                      Field mapping
                    </label>
                    <textarea
                      value={fieldMappingJson}
                      onChange={(e) => {
                        setFieldMappingJson(e.target.value);
                        setFieldMappingError(undefined);
                      }}
                      rows={3}
                      className={inputClass}
                    />
                    {fieldMappingError ? (
                      <p className="mt-1 text-xs text-destructive">
                        {fieldMappingError}
                      </p>
                    ) : null}
                  </div>
                </div>
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

function generationSummaryStats(job: DatasetGenerationJob) {
  const stats: { label: string; value: string }[] = [];
  const solverMode = summaryString(job, "solver_mode");
  if (solverMode && solverMode !== "judge_only") {
    stats.push({ label: "Solver", value: formatReasonCode(solverMode) });
  }
  const avgGap = summaryNumber(job, "avg_gap");
  if (avgGap != null) {
    stats.push({ label: "Avg gap", value: formatScore(avgGap) });
  }
  const avgWeak = summaryNumber(job, "avg_weak_score");
  if (avgWeak != null) {
    stats.push({ label: "Weak", value: formatScore(avgWeak) });
  }
  const avgStrong = summaryNumber(job, "avg_strong_score");
  if (avgStrong != null) {
    stats.push({ label: "Strong", value: formatScore(avgStrong) });
  }
  const topReason = topRejectionReason(job);
  if (topReason) {
    stats.push({ label: "Top reject", value: topReason });
  }
  return stats.slice(0, 6);
}

function summaryString(job: DatasetGenerationJob, key: string) {
  const value = job.summary[key];
  return typeof value === "string" ? value : undefined;
}

function summaryNumber(job: DatasetGenerationJob, key: string) {
  const value = job.summary[key];
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function topRejectionReason(job: DatasetGenerationJob) {
  const reasons = job.summary["rejection_reasons"];
  if (reasons == null || typeof reasons !== "object" || Array.isArray(reasons)) {
    return undefined;
  }
  let top: { reason: string; count: number } | undefined;
  for (const [reason, count] of Object.entries(reasons)) {
    if (typeof count !== "number") continue;
    if (!top || count > top.count) top = { reason, count };
  }
  return top ? `${formatReasonCode(top.reason)} x${top.count}` : undefined;
}

function formatReasonCode(value: string) {
  return value.replace(/_/g, " ");
}

function formatScore(value: number) {
  return value.toFixed(2);
}
