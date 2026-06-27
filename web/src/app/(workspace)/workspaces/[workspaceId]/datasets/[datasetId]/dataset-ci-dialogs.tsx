"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Flag, FlaskConical, Link2, Loader2, RefreshCw } from "lucide-react";

import {
  createDatasetBaseline,
  evaluateDatasetGate,
  startDatasetEval,
  syncDatasetRegressionSuite,
} from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { downloadDatasetGateJUnit } from "@/lib/datasets/gate-junit";
import { waitForRunCompletion } from "@/lib/datasets/wait-for-run";
import type {
  AgentDeployment,
  ChallengePack,
  ChallengePackVersion,
  CreateRunResponse,
  DatasetBaseline,
  DatasetRegressionSuiteLink,
  DatasetVersion,
  EvaluateDatasetGateResponse,
  Run,
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
import { JsonField } from "@/components/ui/json-field";
import { GateResultPanel } from "../dataset-ui-shared";

const inputClass =
  "block w-full rounded-lg border border-input bg-transparent [&>option]:bg-popover [&>option]:text-popover-foreground px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

interface DatasetCiDialogsProps {
  workspaceId: string;
  datasetId: string;
  versions: DatasetVersion[];
  baselines: DatasetBaseline[];
  regressionLink?: DatasetRegressionSuiteLink;
}

export function SyncRegressionDialog({
  workspaceId,
  datasetId,
  versions,
  regressionLink,
}: Pick<
  DatasetCiDialogsProps,
  "workspaceId" | "datasetId" | "versions" | "regressionLink"
>) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [packs, setPacks] = useState<ChallengePack[]>([]);
  const [packVersions, setPackVersions] = useState<ChallengePackVersion[]>([]);
  const [versionId, setVersionId] = useState("");
  const [packId, setPackId] = useState("");
  const [packVersionId, setPackVersionId] = useState("");
  const [challengeKey, setChallengeKey] = useState("");
  const [suiteName, setSuiteName] = useState("");

  useEffect(() => {
    if (!open) return;
    void (async () => {
      setLoading(true);
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const res = await api.get<{ items: ChallengePack[] }>(
          `/v1/workspaces/${workspaceId}/challenge-packs`,
        );
        setPacks(res.items);
        if (versions.length > 0) {
          setVersionId(versions[versions.length - 1].id);
        }
      } catch (err) {
        toast.error(
          err instanceof ApiError ? err.message : "Failed to load challenge packs",
        );
      } finally {
        setLoading(false);
      }
    })();
  }, [open, getAccessToken, workspaceId, versions]);

  useEffect(() => {
    if (!packId) return;
    const pack = packs.find((p) => p.id === packId);
    const runnable = (pack?.versions ?? []).filter(
      (v) => v.lifecycle_status === "runnable",
    );
    setPackVersions(runnable);
    if (runnable.length > 0) {
      setPackVersionId(runnable[runnable.length - 1].id);
    }
  }, [packId, packs]);

  async function handleSync() {
    if (!versionId || !packVersionId || !challengeKey.trim()) {
      toast.error("Fill in version, pack version, and challenge key");
      return;
    }
    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const result = await syncDatasetRegressionSuite(
        api,
        workspaceId,
        datasetId,
        {
          version_id: versionId,
          challenge_pack_version_id: packVersionId,
          challenge_key: challengeKey.trim(),
          regression_suite_id: regressionLink?.regression_suite_id,
          suite_name: suiteName.trim() || undefined,
        },
      );
      toast.success(
        `Synced ${result.created_cases} cases (${result.skipped_cases} skipped)`,
      );
      setOpen(false);
      router.refresh();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Sync failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" variant="outline" />}>
        <RefreshCw data-icon="inline-start" className="size-4" />
        Sync regression suite
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Sync regression suite</DialogTitle>
          <DialogDescription>
            Promote dataset examples into a linked regression suite for CI.
          </DialogDescription>
        </DialogHeader>
        {loading ? (
          <Loader2 className="mx-auto size-6 animate-spin" />
        ) : (
          <div className="space-y-3">
            <FieldSelect
              label="Dataset version"
              value={versionId}
              onChange={setVersionId}
              options={versions.map((v) => ({
                value: v.id,
                label: `v${v.version_number}${v.label ? ` — ${v.label}` : ""}`,
              }))}
            />
            <FieldSelect
              label="Challenge pack"
              value={packId}
              onChange={setPackId}
              options={packs.map((p) => ({ value: p.id, label: p.name }))}
            />
            <FieldSelect
              label="Pack version"
              value={packVersionId}
              onChange={setPackVersionId}
              options={packVersions.map((v) => ({
                value: v.id,
                label: `v${v.version_number}`,
              }))}
              disabled={!packId}
            />
            <TextField
              label="Challenge key"
              value={challengeKey}
              onChange={setChallengeKey}
            />
            {!regressionLink && (
              <TextField
                label="New suite name"
                value={suiteName}
                onChange={setSuiteName}
                placeholder="Optional when linking existing suite"
              />
            )}
          </div>
        )}
        <DialogFooter>
          <Button onClick={handleSync} disabled={submitting || loading}>
            {submitting && (
              <Loader2 data-icon="inline-start" className="size-4 animate-spin" />
            )}
            Sync
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function CreateBaselineDialog({
  workspaceId,
  datasetId,
}: Pick<DatasetCiDialogsProps, "workspaceId" | "datasetId">) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [runId, setRunId] = useState("");
  const [label, setLabel] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function handleCreate() {
    if (!runId.trim()) {
      toast.error("Run ID is required");
      return;
    }
    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await createDatasetBaseline(api, workspaceId, datasetId, {
        run_id: runId.trim(),
        label: label.trim() || undefined,
      });
      toast.success("Baseline recorded");
      setOpen(false);
      router.refresh();
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to create baseline",
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" variant="outline" />}>
        <Link2 data-icon="inline-start" className="size-4" />
        Record baseline
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Record baseline</DialogTitle>
          <DialogDescription>
            Capture pass rate from a completed dataset eval run.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <TextField label="Run ID" value={runId} onChange={setRunId} />
          <TextField
            label="Label"
            value={label}
            onChange={setLabel}
            optional
          />
        </div>
        <DialogFooter>
          <Button onClick={handleCreate} disabled={submitting}>
            {submitting && (
              <Loader2 data-icon="inline-start" className="size-4 animate-spin" />
            )}
            Record
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function EvaluateGateDialog({
  workspaceId,
  datasetId,
  baselines,
}: Pick<DatasetCiDialogsProps, "workspaceId" | "datasetId" | "baselines">) {
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [baselineId, setBaselineId] = useState("");
  const [runId, setRunId] = useState("");
  const [minPassRate, setMinPassRate] = useState("");
  const [maxRegressions, setMaxRegressions] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [gateResponse, setGateResponse] =
    useState<EvaluateDatasetGateResponse | null>(null);

  useEffect(() => {
    if (open && baselines.length > 0 && !baselineId) {
      setBaselineId(baselines[0].id);
    }
  }, [open, baselines, baselineId]);

  async function handleEvaluate() {
    if (!baselineId || !runId.trim()) {
      toast.error("Select a baseline and enter a candidate run ID");
      return;
    }
    setSubmitting(true);
    setGateResponse(null);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const res = await evaluateDatasetGate(api, workspaceId, datasetId, {
        baseline_id: baselineId,
        run_id: runId.trim(),
        min_pass_rate: minPassRate ? Number(minPassRate) : undefined,
        max_regressions: maxRegressions ? Number(maxRegressions) : undefined,
      });
      setGateResponse(res);
      toast.success(res.gate.pass ? "Gate passed" : "Gate failed");
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Gate evaluation failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" variant="outline" />}>
        <Flag data-icon="inline-start" className="size-4" />
        Evaluate gate
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Evaluate CI gate</DialogTitle>
          <DialogDescription>
            Compare a candidate run against a recorded baseline.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3 max-h-[70vh] overflow-y-auto">
          <FieldSelect
            label="Baseline"
            value={baselineId}
            onChange={setBaselineId}
            options={baselines.map((b) => ({
              value: b.id,
              label: b.label ?? b.id.slice(0, 8),
            }))}
          />
          <TextField label="Candidate run ID" value={runId} onChange={setRunId} />
          <TextField
            label="Min pass rate (0–1)"
            value={minPassRate}
            onChange={setMinPassRate}
            optional
          />
          <TextField
            label="Max regressions"
            value={maxRegressions}
            onChange={setMaxRegressions}
            optional
          />
          {gateResponse ? (
            <GateResultPanel
              gate={gateResponse.gate}
              onDownloadJUnit={() =>
                downloadDatasetGateJUnit(gateResponse, datasetId)
              }
            />
          ) : null}
        </div>
        <DialogFooter>
          <Button onClick={handleEvaluate} disabled={submitting}>
            {submitting && (
              <Loader2 data-icon="inline-start" className="size-4 animate-spin" />
            )}
            Evaluate
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function DatasetTestDialog({
  workspaceId,
  datasetId,
  versions,
  baselines,
}: Pick<
  DatasetCiDialogsProps,
  "workspaceId" | "datasetId" | "versions" | "baselines"
>) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [packs, setPacks] = useState<ChallengePack[]>([]);
  const [packVersions, setPackVersions] = useState<ChallengePackVersion[]>([]);
  const [deployments, setDeployments] = useState<AgentDeployment[]>([]);
  const [versionId, setVersionId] = useState("");
  const [packId, setPackId] = useState("");
  const [packVersionId, setPackVersionId] = useState("");
  const [challengeKey, setChallengeKey] = useState("");
  const [deploymentIds, setDeploymentIds] = useState<string[]>([]);
  const [runName, setRunName] = useState("");
  const [mappingJson, setMappingJson] = useState("");
  const [mappingError, setMappingError] = useState<string>();
  const [baselineId, setBaselineId] = useState("");
  const [candidateRunId, setCandidateRunId] = useState("");
  const [minPassRate, setMinPassRate] = useState("");
  const [maxRegressions, setMaxRegressions] = useState("");
  const [runExistingCandidate, setRunExistingCandidate] = useState(false);
  const [phase, setPhase] = useState<
    "config" | "eval" | "gate" | "done"
  >("config");
  const [activeRun, setActiveRun] = useState<Run | CreateRunResponse | null>(
    null,
  );
  const [gateResponse, setGateResponse] =
    useState<EvaluateDatasetGateResponse | null>(null);

  useEffect(() => {
    if (!open) return;
    setPhase("config");
    setActiveRun(null);
    setGateResponse(null);
    setCandidateRunId("");
    setRunExistingCandidate(false);
    void (async () => {
      setLoading(true);
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const [packsRes, deploymentsRes] = await Promise.all([
          api.get<{ items: ChallengePack[] }>(
            `/v1/workspaces/${workspaceId}/challenge-packs`,
          ),
          api.get<{ items: AgentDeployment[] }>(
            `/v1/workspaces/${workspaceId}/agent-deployments`,
          ),
        ]);
        setPacks(packsRes.items);
        setDeployments(
          deploymentsRes.items.filter((item) => item.status === "active"),
        );
        if (versions.length > 0) {
          setVersionId(versions[versions.length - 1].id);
        }
        if (baselines.length > 0) {
          setBaselineId(baselines[0].id);
        }
      } finally {
        setLoading(false);
      }
    })();
  }, [baselines, getAccessToken, open, versions, workspaceId]);

  useEffect(() => {
    if (!packId) return;
    const pack = packs.find((item) => item.id === packId);
    const runnable = (pack?.versions ?? []).filter(
      (item) => item.lifecycle_status === "runnable",
    );
    setPackVersions(runnable);
    setPackVersionId(runnable[runnable.length - 1]?.id ?? "");
  }, [packId, packs]);

  function toggleDeployment(id: string) {
    setDeploymentIds((prev) =>
      prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id],
    );
  }

  async function handleRunTest() {
    if (!baselineId) {
      toast.error("Select a baseline");
      return;
    }
    if (runExistingCandidate) {
      if (!candidateRunId.trim()) {
        toast.error("Enter a candidate run ID");
        return;
      }
    } else if (
      !versionId ||
      !packVersionId ||
      !challengeKey.trim() ||
      deploymentIds.length === 0
    ) {
      toast.error("Fill in eval version, pack, challenge key, and deployments");
      return;
    }

    let mapping: Record<string, unknown> | undefined;
    if (!runExistingCandidate && mappingJson.trim()) {
      try {
        const parsed = JSON.parse(mappingJson) as unknown;
        if (
          parsed == null ||
          typeof parsed !== "object" ||
          Array.isArray(parsed)
        ) {
          setMappingError("Mapping must be a JSON object");
          return;
        }
        mapping = parsed as Record<string, unknown>;
      } catch {
        setMappingError("Invalid JSON");
        return;
      }
    }

    setSubmitting(true);
    setGateResponse(null);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      let runId = candidateRunId.trim();

      if (!runExistingCandidate) {
        setPhase("eval");
        const evalResult = await startDatasetEval(api, workspaceId, datasetId, {
          version_id: versionId,
          challenge_pack_version_id: packVersionId,
          challenge_key: challengeKey.trim(),
          agent_deployment_ids: deploymentIds,
          name: runName.trim() || undefined,
          mapping,
        });
        setActiveRun(evalResult.run);
        const finished = await waitForRunCompletion(api, evalResult.run.id, {
          onStatus: setActiveRun,
        });
        setActiveRun(finished);
        if (finished.status !== "completed") {
          toast.error(`Eval run ended with status ${finished.status}`);
          return;
        }
        runId = finished.id;
      }

      setPhase("gate");
      const gateResult = await evaluateDatasetGate(api, workspaceId, datasetId, {
        baseline_id: baselineId,
        run_id: runId,
        min_pass_rate: minPassRate ? Number(minPassRate) : undefined,
        max_regressions: maxRegressions ? Number(maxRegressions) : undefined,
      });
      setGateResponse(gateResult);
      setPhase("done");
      toast.success(gateResult.gate.pass ? "Dataset test passed" : "Dataset test failed");
      router.refresh();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Dataset test failed");
      setPhase("config");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" variant="outline" />}>
        <FlaskConical data-icon="inline-start" className="size-4" />
        Run CI test
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Run dataset CI test</DialogTitle>
          <DialogDescription>
            Start an eval, wait for completion, then evaluate the gate and export
            JUnit if needed.
          </DialogDescription>
        </DialogHeader>
        {loading ? (
          <div className="flex justify-center py-8">
            <Loader2 className="size-6 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <div className="space-y-4 max-h-[70vh] overflow-y-auto">
            <FieldSelect
              label="Baseline"
              value={baselineId}
              onChange={setBaselineId}
              options={baselines.map((baseline) => ({
                value: baseline.id,
                label: baseline.label ?? baseline.id.slice(0, 8),
              }))}
            />
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={runExistingCandidate}
                onChange={(e) => setRunExistingCandidate(e.target.checked)}
              />
              Use an existing candidate run instead of starting a new eval
            </label>
            {runExistingCandidate ? (
              <TextField
                label="Candidate run ID"
                value={candidateRunId}
                onChange={setCandidateRunId}
              />
            ) : (
              <>
                <FieldSelect
                  label="Dataset version"
                  value={versionId}
                  onChange={setVersionId}
                  options={versions.map((version) => ({
                    value: version.id,
                    label: `v${version.version_number}${version.label ? ` — ${version.label}` : ""}`,
                  }))}
                />
                <FieldSelect
                  label="Challenge pack"
                  value={packId}
                  onChange={setPackId}
                  options={packs.map((pack) => ({
                    value: pack.id,
                    label: pack.name,
                  }))}
                />
                <FieldSelect
                  label="Pack version"
                  value={packVersionId}
                  onChange={setPackVersionId}
                  disabled={!packId}
                  options={packVersions.map((version) => ({
                    value: version.id,
                    label: `v${version.version_number}`,
                  }))}
                />
                <TextField
                  label="Challenge key"
                  value={challengeKey}
                  onChange={setChallengeKey}
                />
                <TextField
                  label="Run name"
                  value={runName}
                  onChange={setRunName}
                  optional
                />
                <JsonField
                  label="Field mapping"
                  value={mappingJson}
                  onChange={setMappingJson}
                  error={mappingError}
                  rows={4}
                />
                <div>
                  <label className="mb-1.5 block text-sm font-medium">
                    Agent deployments
                  </label>
                  <div className="max-h-28 space-y-2 overflow-y-auto rounded-lg border border-border p-2">
                    {deployments.map((deployment) => (
                      <label
                        key={deployment.id}
                        className="flex items-center gap-2 text-sm"
                      >
                        <input
                          type="checkbox"
                          checked={deploymentIds.includes(deployment.id)}
                          onChange={() => toggleDeployment(deployment.id)}
                        />
                        {deployment.name}
                      </label>
                    ))}
                  </div>
                </div>
              </>
            )}
            <TextField
              label="Min pass rate (0–1)"
              value={minPassRate}
              onChange={setMinPassRate}
              optional
            />
            <TextField
              label="Max regressions"
              value={maxRegressions}
              onChange={setMaxRegressions}
              optional
            />
            {phase !== "config" && activeRun ? (
              <div className="rounded-lg border border-border p-3 text-sm">
                <p className="font-medium">Eval run {activeRun.id.slice(0, 8)}</p>
                <p className="mt-1 text-muted-foreground">
                  Status: {activeRun.status}
                </p>
              </div>
            ) : null}
            {gateResponse ? (
              <GateResultPanel
                gate={gateResponse.gate}
                onDownloadJUnit={() =>
                  downloadDatasetGateJUnit(gateResponse, datasetId)
                }
              />
            ) : null}
          </div>
        )}
        <DialogFooter>
          <Button onClick={handleRunTest} disabled={submitting || loading}>
            {submitting && (
              <Loader2 data-icon="inline-start" className="size-4 animate-spin" />
            )}
            {phase === "eval"
              ? "Running eval…"
              : phase === "gate"
                ? "Evaluating gate…"
                : "Run test"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function TextField({
  label,
  value,
  onChange,
  optional,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  optional?: boolean;
  placeholder?: string;
}) {
  return (
    <div>
      <label className="mb-1.5 block text-sm font-medium">
        {label}
        {optional && (
          <span className="font-normal text-muted-foreground"> (optional)</span>
        )}
      </label>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className={inputClass}
      />
    </div>
  );
}

function FieldSelect({
  label,
  value,
  onChange,
  options,
  disabled,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
  disabled?: boolean;
}) {
  return (
    <div>
      <label className="mb-1.5 block text-sm font-medium">{label}</label>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        className={inputClass}
      >
        <option value="">Select...</option>
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </div>
  );
}
