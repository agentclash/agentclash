"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2, Play } from "lucide-react";

import { startDatasetEval } from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { waitForRunCompletion } from "@/lib/datasets/wait-for-run";
import type {
  AgentDeployment,
  ChallengePack,
  ChallengePackVersion,
  CreateRunResponse,
  DatasetVersion,
  Run,
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
import { JsonField } from "@/components/ui/json-field";

const inputClass =
  "block w-full rounded-lg border border-input bg-transparent [&>option]:bg-popover [&>option]:text-popover-foreground px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

interface StartEvalDialogProps {
  workspaceId: string;
  datasetId: string;
  versions: DatasetVersion[];
}

export function StartEvalDialog({
  workspaceId,
  datasetId,
  versions,
}: StartEvalDialogProps) {
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
  const [waitForCompletion, setWaitForCompletion] = useState(false);
  const [activeRun, setActiveRun] = useState<Run | CreateRunResponse | null>(
    null,
  );

  const loadData = useCallback(async () => {
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
      setDeployments(deploymentsRes.items.filter((item) => item.status === "active"));
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load eval options");
    } finally {
      setLoading(false);
    }
  }, [getAccessToken, workspaceId]);

  useEffect(() => {
    if (!open) return;
    void loadData();
    setActiveRun(null);
  }, [open, loadData]);

  useEffect(() => {
    if (!open || versions.length === 0) return;
    setVersionId((current) => current || versions[versions.length - 1].id);
  }, [open, versions]);

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
    if (runnable.length > 0) {
      setPackVersionId(runnable[runnable.length - 1].id);
    }
  }, [packId, packs]);

  function toggleDeployment(id: string) {
    setDeploymentIds((prev) =>
      prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id],
    );
  }

  async function handleStart() {
    if (!versionId || !packVersionId || deploymentIds.length === 0) {
      toast.error("Select a version, challenge pack version, and deployments");
      return;
    }
    if (!challengeKey.trim()) {
      toast.error("Challenge key is required");
      return;
    }

    let mapping: Record<string, unknown> | undefined;
    if (mappingJson.trim()) {
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
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const result = await startDatasetEval(api, workspaceId, datasetId, {
        version_id: versionId,
        challenge_pack_version_id: packVersionId,
        challenge_key: challengeKey.trim(),
        agent_deployment_ids: deploymentIds,
        name: runName.trim() || undefined,
        mapping,
      });

      if (waitForCompletion) {
        setActiveRun(result.run);
        toast.success("Eval run queued — waiting for completion");
        const finished = await waitForRunCompletion(api, result.run.id, {
          onStatus: setActiveRun,
        });
        setActiveRun(finished);
        if (finished.status === "completed") {
          toast.success("Eval run completed");
        } else {
          toast.error(`Eval run ended with status ${finished.status}`);
        }
        router.push(`/workspaces/${workspaceId}/runs/${finished.id}`);
        setOpen(false);
        return;
      }

      toast.success("Dataset eval run queued");
      setOpen(false);
      router.push(`/workspaces/${workspaceId}/runs/${result.run.id}`);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to start eval");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" />}>
        <Play data-icon="inline-start" className="size-4" />
        Run eval
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Run dataset eval</DialogTitle>
          <DialogDescription>
            Queue an eval over a pinned dataset version and challenge pack.
          </DialogDescription>
        </DialogHeader>
        {activeRun && submitting ? (
          <div className="rounded-lg border border-border p-4 text-sm">
            <div className="flex items-center gap-2">
              <Loader2 className="size-4 animate-spin text-muted-foreground" />
              <span>
                Waiting for run {activeRun.id.slice(0, 8)} —{" "}
                <Badge variant="secondary">{activeRun.status}</Badge>
              </span>
            </div>
          </div>
        ) : loading ? (
          <div className="flex justify-center py-8">
            <Loader2 className="size-6 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <div className="space-y-4 max-h-[60vh] overflow-y-auto">
            <SelectField
              label="Dataset version"
              value={versionId}
              onChange={setVersionId}
              options={versions.map((version) => ({
                value: version.id,
                label: `v${version.version_number}${version.label ? ` — ${version.label}` : ""}`,
              }))}
              emptyHint={
                versions.length === 0 ? "Create a version snapshot first." : undefined
              }
            />
            <SelectField
              label="Challenge pack"
              value={packId}
              onChange={setPackId}
              options={packs.map((pack) => ({
                value: pack.id,
                label: pack.name,
              }))}
            />
            <SelectField
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
              placeholder="e.g. refund-recovery"
            />
            <TextField label="Run name" value={runName} onChange={setRunName} optional />
            <JsonField
              label="Field mapping"
              description='Optional JSON mapping for dataset example fields, e.g. {"input_key":"prompt"}.'
              value={mappingJson}
              onChange={setMappingJson}
              error={mappingError}
              rows={4}
            />
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Agent deployments
              </label>
              <div className="max-h-32 space-y-2 overflow-y-auto rounded-lg border border-border p-2">
                {deployments.length === 0 ? (
                  <p className="text-xs text-muted-foreground">
                    No active deployments.{" "}
                    <Link
                      href={`/workspaces/${workspaceId}/deployments`}
                      className="underline"
                    >
                      Create one
                    </Link>
                  </p>
                ) : (
                  deployments.map((deployment) => (
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
                  ))
                )}
              </div>
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={waitForCompletion}
                onChange={(e) => setWaitForCompletion(e.target.checked)}
              />
              Wait for run completion before continuing
            </label>
          </div>
        )}
        <DialogFooter>
          <Button onClick={handleStart} disabled={submitting || loading}>
            {submitting && (
              <Loader2 data-icon="inline-start" className="size-4 animate-spin" />
            )}
            Start eval
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
  onChange: (value: string) => void;
  optional?: boolean;
  placeholder?: string;
}) {
  return (
    <div>
      <label className="mb-1.5 block text-sm font-medium">
        {label}
        {optional ? (
          <span className="font-normal text-muted-foreground"> (optional)</span>
        ) : null}
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

function SelectField({
  label,
  value,
  onChange,
  options,
  disabled,
  emptyHint,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
  disabled?: boolean;
  emptyHint?: string;
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
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      {emptyHint ? (
        <p className="mt-1 text-xs text-muted-foreground">{emptyHint}</p>
      ) : null}
    </div>
  );
}
