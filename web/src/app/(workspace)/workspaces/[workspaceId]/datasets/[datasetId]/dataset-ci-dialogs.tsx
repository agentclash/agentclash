"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Flag, Link2, Loader2, RefreshCw } from "lucide-react";

import {
  createDatasetBaseline,
  evaluateDatasetGate,
  syncDatasetRegressionSuite,
} from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  ChallengePack,
  ChallengePackVersion,
  DatasetBaseline,
  DatasetRegressionSuiteLink,
  DatasetVersion,
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
  const [result, setResult] = useState<{
    pass: boolean;
    pass_rate: number;
    regression_count: number;
  } | null>(null);

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
    setResult(null);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const res = await evaluateDatasetGate(api, workspaceId, datasetId, {
        baseline_id: baselineId,
        run_id: runId.trim(),
        min_pass_rate: minPassRate ? Number(minPassRate) : undefined,
        max_regressions: maxRegressions ? Number(maxRegressions) : undefined,
      });
      setResult(res.gate);
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
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Evaluate CI gate</DialogTitle>
          <DialogDescription>
            Compare a candidate run against a recorded baseline.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
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
          {result && (
            <div className="rounded-md border border-border p-3 text-sm space-y-1">
              <div className="flex items-center gap-2">
                <Badge variant={result.pass ? "default" : "destructive"}>
                  {result.pass ? "PASS" : "FAIL"}
                </Badge>
                <span className="text-muted-foreground">
                  Pass rate {(result.pass_rate * 100).toFixed(1)}%
                </span>
              </div>
              <p className="text-muted-foreground">
                {result.regression_count} regressions
              </p>
            </div>
          )}
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
