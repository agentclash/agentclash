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
import type {
  AgentDeployment,
  ChallengePack,
  ChallengePackVersion,
  DatasetVersion,
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

const inputClass =
  "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

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
      setDeployments(deploymentsRes.items.filter((d) => d.status === "active"));
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load eval options");
    } finally {
      setLoading(false);
    }
  }, [getAccessToken, workspaceId]);

  useEffect(() => {
    if (open) {
      void loadData();
      if (versions.length > 0 && !versionId) {
        setVersionId(versions[versions.length - 1].id);
      }
    }
  }, [open, loadData, versionId, versions]);

  useEffect(() => {
    if (!packId) {
      setPackVersions([]);
      setPackVersionId("");
      return;
    }
    const pack = packs.find((p) => p.id === packId);
    const runnable = (pack?.versions ?? []).filter(
      (v) => v.lifecycle_status === "runnable",
    );
    setPackVersions(runnable);
    if (runnable.length > 0) {
      setPackVersionId(runnable[runnable.length - 1].id);
    }
  }, [packId, packs]);

  function toggleDeployment(id: string) {
    setDeploymentIds((prev) =>
      prev.includes(id) ? prev.filter((d) => d !== id) : [...prev, id],
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
      });
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
            Queue a race over a pinned dataset version and challenge pack.
          </DialogDescription>
        </DialogHeader>
        {loading ? (
          <div className="flex justify-center py-8">
            <Loader2 className="size-6 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <div className="space-y-4 max-h-[60vh] overflow-y-auto">
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Dataset version
              </label>
              <select
                value={versionId}
                onChange={(e) => setVersionId(e.target.value)}
                className={inputClass}
              >
                <option value="">Select version...</option>
                {versions.map((v) => (
                  <option key={v.id} value={v.id}>
                    v{v.version_number}
                    {v.label ? ` — ${v.label}` : ""}
                  </option>
                ))}
              </select>
              {versions.length === 0 && (
                <p className="mt-1 text-xs text-muted-foreground">
                  Create a version snapshot first.
                </p>
              )}
            </div>
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
                {packs.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
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
                className={inputClass}
                disabled={!packId}
              >
                <option value="">Select version...</option>
                {packVersions.map((v) => (
                  <option key={v.id} value={v.id}>
                    v{v.version_number}
                  </option>
                ))}
              </select>
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
              <label className="mb-1.5 block text-sm font-medium">Run name</label>
              <input
                value={runName}
                onChange={(e) => setRunName(e.target.value)}
                className={inputClass}
              />
            </div>
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
                  deployments.map((d) => (
                    <label
                      key={d.id}
                      className="flex items-center gap-2 text-sm"
                    >
                      <input
                        type="checkbox"
                        checked={deploymentIds.includes(d.id)}
                        onChange={() => toggleDeployment(d.id)}
                      />
                      {d.name}
                    </label>
                  ))
                )}
              </div>
            </div>
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
