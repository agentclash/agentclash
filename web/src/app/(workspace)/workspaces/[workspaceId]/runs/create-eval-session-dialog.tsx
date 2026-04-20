"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { Loader2, Sigma } from "lucide-react";
import { toast } from "sonner";

import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  AgentDeployment,
  ChallengeInputSetSummary,
  ChallengePack,
  ChallengePackVersion,
  CreateEvalSessionRequest,
  CreateEvalSessionResponse,
  EvalSessionValidationDetail,
  EvalSessionValidationEnvelope,
  EvalSessionTaskProperties,
} from "@/lib/api/types";
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

interface CreateEvalSessionDialogProps {
  workspaceId: string;
}

function collectTaskProperties(
  input: EvalSessionTaskProperties,
): EvalSessionTaskProperties | undefined {
  const properties: EvalSessionTaskProperties = {};
  if (input.has_side_effects) properties.has_side_effects = true;
  if (input.autonomy) properties.autonomy = input.autonomy;
  if (input.step_count != null) properties.step_count = input.step_count;
  if (input.output_type) properties.output_type = input.output_type;
  return Object.keys(properties).length > 0 ? properties : undefined;
}

function formatValidationMessage(
  errors: EvalSessionValidationDetail[],
): string | null {
  if (
    errors.some(
      (error) => error.code === "participants.agent_deployment_id.unresolved",
    )
  ) {
    return "One or more selected deployments are no longer active or available in this workspace. Refresh the dialog and choose another deployment.";
  }

  return null;
}

export function CreateEvalSessionDialog({
  workspaceId,
}: CreateEvalSessionDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();

  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [selectedPackId, setSelectedPackId] = useState("");
  const [selectedVersionId, setSelectedVersionId] = useState("");
  const [inputSetId, setInputSetId] = useState("");
  const [selectedDeploymentIds, setSelectedDeploymentIds] = useState<string[]>([]);
  const [participantLabels, setParticipantLabels] = useState<Record<string, string>>(
    {},
  );
  const [repetitions, setRepetitions] = useState("5");
  const [aggregationMethod, setAggregationMethod] = useState<"median" | "mean">(
    "median",
  );
  const [reportVariance, setReportVariance] = useState(true);
  const [confidenceInterval, setConfidenceInterval] = useState("0.95");
  const [reliabilityWeight, setReliabilityWeight] = useState("");
  const [minPassRate, setMinPassRate] = useState("");
  const [hasSideEffects, setHasSideEffects] = useState(false);
  const [autonomy, setAutonomy] = useState<"" | "human" | "semi" | "full">("");
  const [stepCount, setStepCount] = useState("");
  const [outputType, setOutputType] = useState<"" | "artifact" | "action">("");
  const [submitting, setSubmitting] = useState(false);
  const [loading, setLoading] = useState(false);
  const [loadingInputSets, setLoadingInputSets] = useState(false);

  const [packs, setPacks] = useState<ChallengePack[]>([]);
  const [runnableVersions, setRunnableVersions] = useState<ChallengePackVersion[]>(
    [],
  );
  const [inputSets, setInputSets] = useState<ChallengeInputSetSummary[]>([]);
  const [deployments, setDeployments] = useState<AgentDeployment[]>([]);

  const deploymentById = Object.fromEntries(
    deployments.map((deployment) => [deployment.id, deployment]),
  );

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const [packsResponse, deploymentsResponse] = await Promise.all([
        api.get<{ items: ChallengePack[] }>(
          `/v1/workspaces/${workspaceId}/challenge-packs`,
        ),
        api.get<{ items: AgentDeployment[] }>(
          `/v1/workspaces/${workspaceId}/agent-deployments`,
        ),
      ]);
      setPacks(packsResponse.items);
      setDeployments(
        deploymentsResponse.items.filter((deployment) => deployment.status === "active"),
      );
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load data");
    } finally {
      setLoading(false);
    }
  }, [getAccessToken, workspaceId]);

  useEffect(() => {
    if (open) void loadData();
  }, [loadData, open]);

  useEffect(() => {
    if (!open || !selectedVersionId) {
      setInputSetId("");
      setInputSets([]);
      setLoadingInputSets(false);
      return;
    }

    let cancelled = false;
    setInputSetId("");
    setInputSets([]);
    setLoadingInputSets(true);

    void (async () => {
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const response = await api.get<{ items: ChallengeInputSetSummary[] }>(
          `/v1/workspaces/${workspaceId}/challenge-pack-versions/${selectedVersionId}/input-sets`,
        );

        if (cancelled) return;
        setInputSets(response.items);
        if (response.items.length === 1) {
          setInputSetId(response.items[0].id);
        }
      } catch (err) {
        if (cancelled) return;
        toast.error(
          err instanceof ApiError ? err.message : "Failed to load input sets",
        );
      } finally {
        if (!cancelled) setLoadingInputSets(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [getAccessToken, open, selectedVersionId, workspaceId]);

  function resetForm() {
    setName("");
    setSelectedPackId("");
    setSelectedVersionId("");
    setInputSetId("");
    setSelectedDeploymentIds([]);
    setParticipantLabels({});
    setRepetitions("5");
    setAggregationMethod("median");
    setReportVariance(true);
    setConfidenceInterval("0.95");
    setReliabilityWeight("");
    setMinPassRate("");
    setHasSideEffects(false);
    setAutonomy("");
    setStepCount("");
    setOutputType("");
    setRunnableVersions([]);
    setInputSets([]);
  }

  function handlePackChange(packId: string) {
    setSelectedPackId(packId);
    setSelectedVersionId("");
    setInputSetId("");
    setInputSets([]);
    if (!packId) {
      setRunnableVersions([]);
      return;
    }
    const pack = packs.find((candidate) => candidate.id === packId);
    const versions = (pack?.versions ?? []).filter(
      (version) => version.lifecycle_status === "runnable",
    );
    setRunnableVersions(versions);
    if (versions.length === 1) {
      setSelectedVersionId(versions[0].id);
    }
  }

  function toggleDeployment(deploymentId: string) {
    const deployment = deploymentById[deploymentId];
    if (!deployment) return;

    setSelectedDeploymentIds((current) =>
      current.includes(deploymentId)
        ? current.filter((id) => id !== deploymentId)
        : [...current, deploymentId],
    );
    setParticipantLabels((current) => ({
      ...current,
      [deploymentId]: current[deploymentId] ?? deployment.name,
    }));
  }

  async function handleCreate() {
    if (!selectedVersionId || selectedDeploymentIds.length === 0) return;

    const parsedRepetitions = Number.parseInt(repetitions, 10);
    const parsedConfidenceInterval = Number.parseFloat(confidenceInterval);
    if (
      Number.isNaN(parsedRepetitions) ||
      parsedRepetitions < 1 ||
      parsedRepetitions > 100
    ) {
      toast.error("Repetitions must be between 1 and 100.");
      return;
    }
    if (
      Number.isNaN(parsedConfidenceInterval) ||
      parsedConfidenceInterval <= 0 ||
      parsedConfidenceInterval >= 1
    ) {
      toast.error("Confidence interval must be between 0 and 1.");
      return;
    }

    const parsedReliabilityWeight =
      reliabilityWeight.trim() === ""
        ? undefined
        : Number.parseFloat(reliabilityWeight);
    if (
      parsedReliabilityWeight != null &&
      (Number.isNaN(parsedReliabilityWeight) ||
        parsedReliabilityWeight < 0 ||
        parsedReliabilityWeight > 1)
    ) {
      toast.error("Reliability weight must be between 0 and 1.");
      return;
    }

    const parsedMinPassRate =
      minPassRate.trim() === "" ? undefined : Number.parseFloat(minPassRate);
    if (
      parsedMinPassRate != null &&
      (Number.isNaN(parsedMinPassRate) ||
        parsedMinPassRate < 0 ||
        parsedMinPassRate > 1)
    ) {
      toast.error("Success threshold must be between 0 and 1.");
      return;
    }

    const parsedStepCount =
      stepCount.trim() === "" ? undefined : Number.parseInt(stepCount, 10);
    if (parsedStepCount != null && (Number.isNaN(parsedStepCount) || parsedStepCount < 1)) {
      toast.error("Step count must be at least 1 when provided.");
      return;
    }

    const taskProperties = collectTaskProperties({
      has_side_effects: hasSideEffects,
      autonomy: autonomy || undefined,
      step_count: parsedStepCount,
      output_type: outputType || undefined,
    });

    const request: CreateEvalSessionRequest = {
      workspace_id: workspaceId,
      challenge_pack_version_id: selectedVersionId,
      challenge_input_set_id: inputSetId.trim() || undefined,
      participants: selectedDeploymentIds.map((deploymentId) => ({
        agent_deployment_id: deploymentId,
        label: participantLabels[deploymentId]?.trim() || deploymentById[deploymentId].name,
      })),
      execution_mode:
        selectedDeploymentIds.length > 1 ? "comparison" : "single_agent",
      name: name.trim() || undefined,
      eval_session: {
        repetitions: parsedRepetitions,
        aggregation: {
          method: aggregationMethod,
          report_variance: reportVariance,
          confidence_interval: parsedConfidenceInterval,
          reliability_weight: parsedReliabilityWeight,
        },
        success_threshold:
          parsedMinPassRate != null
            ? {
                min_pass_rate: parsedMinPassRate,
              }
            : null,
        routing_task_snapshot: {
          routing: {},
          task: taskProperties ? { task_properties: taskProperties } : {},
        },
        schema_version: 1,
      },
    };

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const response = await api.postWithMeta<
        CreateEvalSessionResponse | EvalSessionValidationEnvelope
      >("/v1/eval-sessions", request, {
        allowedStatuses: [422],
      });

      if (response.status === 422) {
        const body = response.data as EvalSessionValidationEnvelope;
        const message =
          formatValidationMessage(body.errors) ??
          body.errors
            .slice(0, 3)
            .map((error) => error.message)
            .join(" ");
        toast.error(message || "Eval session configuration is invalid.");
        return;
      }

      const body = response.data as CreateEvalSessionResponse;
      toast.success("Eval session created");
      setOpen(false);
      resetForm();
      router.push(
        `/workspaces/${workspaceId}/eval-sessions/${body.eval_session.id}`,
      );
      router.refresh();
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to create eval session",
      );
    } finally {
      setSubmitting(false);
    }
  }

  const requiresInputSetSelection = inputSets.length > 1;
  const canSubmit =
    Boolean(selectedVersionId) &&
    selectedDeploymentIds.length > 0 &&
    !loadingInputSets &&
    (!requiresInputSetSelection || Boolean(inputSetId));

  const selectClass =
    "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50 disabled:opacity-50";
  const inputClass =
    "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" variant="outline" />}>
        <Sigma data-icon="inline-start" className="size-4" />
        New Eval Session
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>New Eval Session</DialogTitle>
          <DialogDescription>
            Create a repeated eval session that fans out child runs and aggregates the result statistically.
          </DialogDescription>
        </DialogHeader>

        <div className="max-h-[70vh] space-y-4 overflow-y-auto py-2">
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Name{" "}
              <span className="font-normal text-muted-foreground">(optional)</span>
            </label>
            <input
              type="text"
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="Auto-generated if empty"
              className={inputClass}
            />
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium">Challenge Pack</label>
            <select
              value={selectedPackId}
              onChange={(event) => handlePackChange(event.target.value)}
              className={selectClass}
              disabled={loading}
            >
              <option value="">Select a challenge pack</option>
              {packs.map((pack) => (
                <option key={pack.id} value={pack.id}>
                  {pack.name}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium">Runnable Version</label>
            <select
              value={selectedVersionId}
              onChange={(event) => setSelectedVersionId(event.target.value)}
              className={selectClass}
              disabled={!selectedPackId || runnableVersions.length === 0}
            >
              <option value="">Select a runnable version</option>
              {runnableVersions.map((version) => (
                <option key={version.id} value={version.id}>
                  Version {version.version_number}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Input Set{" "}
              <span className="font-normal text-muted-foreground">
                ({loadingInputSets ? "loading" : inputSets.length || "default"})
              </span>
            </label>
            <select
              value={inputSetId}
              onChange={(event) => setInputSetId(event.target.value)}
              className={selectClass}
              disabled={!selectedVersionId || loadingInputSets || inputSets.length === 0}
            >
              <option value="">
                {inputSets.length === 0 ? "Default / auto-select" : "Select an input set"}
              </option>
              {inputSets.map((inputSet) => (
                <option key={inputSet.id} value={inputSet.id}>
                  {inputSet.name}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-3 rounded-lg border border-border p-4">
            <div>
              <h3 className="text-sm font-medium">Participants</h3>
              <p className="mt-1 text-sm text-muted-foreground">
                Select one or more active deployments. The exact deployments you pick here will be used as eval-session participants.
              </p>
            </div>

            <div className="space-y-3">
              {deployments.map((deployment) => {
                const checked = selectedDeploymentIds.includes(deployment.id);
                return (
                  <div
                    key={deployment.id}
                    className="rounded-lg border border-border bg-background/60 p-3"
                  >
                    <label className="flex items-center gap-3 text-sm font-medium">
                      <input
                        type="checkbox"
                        checked={checked}
                        onChange={() => toggleDeployment(deployment.id)}
                        className="size-4 rounded border-border accent-primary"
                      />
                      <span>{deployment.name}</span>
                    </label>
                    {checked ? (
                      <div className="mt-3">
                        <label className="mb-1.5 block text-xs font-medium uppercase tracking-[0.12em] text-muted-foreground">
                          Participant label
                        </label>
                        <input
                          type="text"
                          value={participantLabels[deployment.id] ?? deployment.name}
                          onChange={(event) =>
                            setParticipantLabels((current) => ({
                              ...current,
                              [deployment.id]: event.target.value,
                            }))
                          }
                          className={inputClass}
                        />
                      </div>
                    ) : null}
                  </div>
                );
              })}
            </div>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <div>
              <label className="mb-1.5 block text-sm font-medium">Repetitions</label>
              <input
                type="number"
                min="1"
                max="100"
                value={repetitions}
                onChange={(event) => setRepetitions(event.target.value)}
                className={inputClass}
              />
            </div>
            <div>
              <label className="mb-1.5 block text-sm font-medium">Aggregation method</label>
              <select
                value={aggregationMethod}
                onChange={(event) =>
                  setAggregationMethod(event.target.value as "median" | "mean")
                }
                className={selectClass}
              >
                <option value="median">Median</option>
                <option value="mean">Mean</option>
              </select>
            </div>
            <div>
              <label className="mb-1.5 block text-sm font-medium">Confidence interval</label>
              <input
                type="number"
                min="0.01"
                max="0.99"
                step="0.01"
                value={confidenceInterval}
                onChange={(event) => setConfidenceInterval(event.target.value)}
                className={inputClass}
              />
            </div>
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Reliability weight override{" "}
                <span className="font-normal text-muted-foreground">(optional)</span>
              </label>
              <input
                type="number"
                min="0"
                max="1"
                step="0.05"
                value={reliabilityWeight}
                onChange={(event) => setReliabilityWeight(event.target.value)}
                placeholder="Leave blank to infer from task properties"
                className={inputClass}
              />
            </div>
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Success threshold{" "}
                <span className="font-normal text-muted-foreground">(optional)</span>
              </label>
              <input
                type="number"
                min="0"
                max="1"
                step="0.05"
                value={minPassRate}
                onChange={(event) => setMinPassRate(event.target.value)}
                placeholder="Example: 0.8"
                className={inputClass}
              />
            </div>
            <div className="flex items-end">
              <label className="flex items-center gap-3 rounded-lg border border-border bg-background/60 px-3 py-2 text-sm">
                <input
                  type="checkbox"
                  checked={reportVariance}
                  onChange={(event) => setReportVariance(event.target.checked)}
                  className="size-4 rounded border-border accent-primary"
                />
                Report variance and high-variance dimensions
              </label>
            </div>
          </div>

          <div className="space-y-3 rounded-lg border border-border p-4">
            <div>
              <h3 className="text-sm font-medium">Metric routing hints</h3>
              <p className="mt-1 text-sm text-muted-foreground">
                These optional task properties help the backend decide whether pass@k or pass^k should be emphasized.
              </p>
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <label className="flex items-center gap-3 rounded-lg border border-border bg-background/60 px-3 py-2 text-sm">
                <input
                  type="checkbox"
                  checked={hasSideEffects}
                  onChange={(event) => setHasSideEffects(event.target.checked)}
                  className="size-4 rounded border-border accent-primary"
                />
                Task has side effects
              </label>

              <div>
                <label className="mb-1.5 block text-sm font-medium">Autonomy</label>
                <select
                  value={autonomy}
                  onChange={(event) =>
                    setAutonomy(event.target.value as "" | "human" | "semi" | "full")
                  }
                  className={selectClass}
                >
                  <option value="">Unspecified</option>
                  <option value="human">Human-reviewed</option>
                  <option value="semi">Semi-autonomous</option>
                  <option value="full">Fully autonomous</option>
                </select>
              </div>

              <div>
                <label className="mb-1.5 block text-sm font-medium">Step count</label>
                <input
                  type="number"
                  min="1"
                  value={stepCount}
                  onChange={(event) => setStepCount(event.target.value)}
                  placeholder="Optional"
                  className={inputClass}
                />
              </div>

              <div>
                <label className="mb-1.5 block text-sm font-medium">Output type</label>
                <select
                  value={outputType}
                  onChange={(event) =>
                    setOutputType(event.target.value as "" | "artifact" | "action")
                  }
                  className={selectClass}
                >
                  <option value="">Unspecified</option>
                  <option value="artifact">Artifact</option>
                  <option value="action">Action</option>
                </select>
              </div>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)} disabled={submitting}>
            Cancel
          </Button>
          <Button onClick={handleCreate} disabled={!canSubmit || submitting}>
            {submitting ? <Loader2 className="size-4 animate-spin" /> : null}
            Create Eval Session
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
