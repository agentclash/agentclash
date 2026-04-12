"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  AgentDeployment,
  ChallengePack,
  ChallengePackVersion,
  CreateRunResponse,
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
import { toast } from "sonner";
import { Loader2, Plus } from "lucide-react";

interface CreateRunDialogProps {
  workspaceId: string;
}

export function CreateRunDialog({ workspaceId }: CreateRunDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();

  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [selectedPackId, setSelectedPackId] = useState("");
  const [selectedVersionId, setSelectedVersionId] = useState("");
  const [inputSetId, setInputSetId] = useState("");
  const [selectedDeploymentIds, setSelectedDeploymentIds] = useState<string[]>(
    [],
  );
  const [submitting, setSubmitting] = useState(false);

  const [packs, setPacks] = useState<ChallengePack[]>([]);
  const [runnableVersions, setRunnableVersions] = useState<
    ChallengePackVersion[]
  >([]);
  const [deployments, setDeployments] = useState<AgentDeployment[]>([]);
  const [loading, setLoading] = useState(false);

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
    } catch {
      toast.error("Failed to load data");
    } finally {
      setLoading(false);
    }
  }, [getAccessToken, workspaceId]);

  useEffect(() => {
    if (open) loadData();
  }, [open, loadData]);

  function handlePackChange(packId: string) {
    setSelectedPackId(packId);
    setSelectedVersionId("");
    if (packId) {
      const pack = packs.find((p) => p.id === packId);
      const runnable = (pack?.versions ?? []).filter(
        (v) => v.lifecycle_status === "runnable",
      );
      setRunnableVersions(runnable);
      if (runnable.length === 1) setSelectedVersionId(runnable[0].id);
    } else {
      setRunnableVersions([]);
    }
  }

  function toggleDeployment(id: string) {
    setSelectedDeploymentIds((prev) =>
      prev.includes(id) ? prev.filter((d) => d !== id) : [...prev, id],
    );
  }

  async function handleCreate() {
    if (!selectedVersionId || selectedDeploymentIds.length === 0) return;

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const result = await api.post<CreateRunResponse>("/v1/runs", {
        workspace_id: workspaceId,
        challenge_pack_version_id: selectedVersionId,
        challenge_input_set_id: inputSetId.trim() || undefined,
        name: name.trim() || undefined,
        agent_deployment_ids: selectedDeploymentIds,
      });
      toast.success("Run created");
      setOpen(false);
      resetForm();
      router.push(`/workspaces/${workspaceId}/runs/${result.id}`);
      router.refresh();
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to create run",
      );
    } finally {
      setSubmitting(false);
    }
  }

  function resetForm() {
    setName("");
    setSelectedPackId("");
    setSelectedVersionId("");
    setInputSetId("");
    setSelectedDeploymentIds([]);
    setRunnableVersions([]);
  }

  const executionMode =
    selectedDeploymentIds.length > 1 ? "comparison" : "single_agent";
  const canSubmit = selectedVersionId && selectedDeploymentIds.length > 0;

  const selectClass =
    "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50 disabled:opacity-50";

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" />}>
        <Plus data-icon="inline-start" className="size-4" />
        New Run
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>New Run</DialogTitle>
          <DialogDescription>
            Run agent deployments against a challenge pack.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2 max-h-[60vh] overflow-y-auto">
          {/* Name */}
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Name{" "}
              <span className="text-muted-foreground font-normal">
                (optional)
              </span>
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Auto-generated if empty"
              className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50"
            />
          </div>

          {/* Challenge Pack */}
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Challenge Pack
            </label>
            <select
              value={selectedPackId}
              onChange={(e) => handlePackChange(e.target.value)}
              disabled={loading}
              className={selectClass}
            >
              <option value="">
                {loading ? "Loading..." : "Select a challenge pack"}
              </option>
              {packs.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          </div>

          {/* Pack Version */}
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Version{" "}
              <span className="text-muted-foreground font-normal">
                (runnable only)
              </span>
            </label>
            <select
              value={selectedVersionId}
              onChange={(e) => setSelectedVersionId(e.target.value)}
              disabled={!selectedPackId}
              className={selectClass}
            >
              <option value="">
                {runnableVersions.length === 0 && selectedPackId
                  ? "No runnable versions"
                  : "Select a version"}
              </option>
              {runnableVersions.map((v) => (
                <option key={v.id} value={v.id}>
                  v{v.version_number}
                </option>
              ))}
            </select>
          </div>

          {/* Input Set ID (optional) */}
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Input Set ID{" "}
              <span className="text-muted-foreground font-normal">
                (optional)
              </span>
            </label>
            <input
              type="text"
              value={inputSetId}
              onChange={(e) => setInputSetId(e.target.value)}
              placeholder="UUID — leave empty for all input sets"
              className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm font-[family-name:var(--font-mono)] placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50"
            />
          </div>

          {/* Agent Deployments (multi-select) */}
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Agent Deployments
            </label>
            {loading ? (
              <p className="text-sm text-muted-foreground">Loading...</p>
            ) : deployments.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                No active deployments — create one first.
              </p>
            ) : (
              <div className="space-y-1.5 rounded-lg border border-input p-2 max-h-40 overflow-y-auto">
                {deployments.map((d) => (
                  <label
                    key={d.id}
                    className="flex items-center gap-2 rounded px-2 py-1.5 text-sm hover:bg-muted/50 cursor-pointer"
                  >
                    <input
                      type="checkbox"
                      checked={selectedDeploymentIds.includes(d.id)}
                      onChange={() => toggleDeployment(d.id)}
                      className="rounded border-input"
                    />
                    <span className="truncate">{d.name}</span>
                  </label>
                ))}
              </div>
            )}
          </div>

          {/* Preview */}
          {canSubmit && (
            <div className="rounded-lg border border-border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
              This will run{" "}
              <span className="text-foreground font-medium">
                {selectedDeploymentIds.length} agent
                {selectedDeploymentIds.length !== 1 ? "s" : ""}
              </span>{" "}
              against the selected challenge pack in{" "}
              <span className="text-foreground font-medium">
                {executionMode === "comparison"
                  ? "comparison"
                  : "single-agent"}{" "}
                mode
              </span>
              .
            </div>
          )}
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => setOpen(false)}
            disabled={submitting}
          >
            Cancel
          </Button>
          <Button disabled={!canSubmit || submitting} onClick={handleCreate}>
            {submitting ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              "Create Run"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
