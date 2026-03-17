"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/lib/stores/auth";
import {
  api,
  type AgentDeployment,
  type ChallengePack,
} from "@/lib/api/client";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ArrowLeft, Loader2, Check, Plus, X } from "lucide-react";
import Link from "next/link";

export default function CreateRunPage() {
  const router = useRouter();
  const { activeWorkspaceId } = useAuthStore();

  const [name, setName] = useState("");
  const [deployments, setDeployments] = useState<AgentDeployment[]>([]);
  const [challengePacks, setChallengePacks] = useState<ChallengePack[]>([]);
  const [selectedDeploymentIds, setSelectedDeploymentIds] = useState<string[]>([]);
  const [selectedPackVersionId, setSelectedPackVersionId] = useState("");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [loadError, setLoadError] = useState("");

  useEffect(() => {
    async function loadFormData() {
      if (!activeWorkspaceId) return;
      setLoading(true);
      setLoadError("");
      try {
        const [deploymentsRes, packsRes] = await Promise.allSettled([
          api.listAgentDeployments(activeWorkspaceId),
          api.listChallengePacks(activeWorkspaceId),
        ]);
        if (deploymentsRes.status === "fulfilled") {
          setDeployments(deploymentsRes.value.items);
        }
        if (packsRes.status === "fulfilled") {
          setChallengePacks(packsRes.value.items);
        }
      } catch (err) {
        setLoadError(err instanceof Error ? err.message : "Failed to load form data");
      } finally {
        setLoading(false);
      }
    }
    loadFormData();
  }, [activeWorkspaceId]);

  function toggleDeployment(id: string) {
    setSelectedDeploymentIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    );
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!activeWorkspaceId) return;

    setError("");
    if (!selectedPackVersionId) {
      setError("Please select a challenge pack version");
      return;
    }
    if (selectedDeploymentIds.length === 0) {
      setError("Select at least one agent deployment");
      return;
    }

    setSubmitting(true);
    try {
      const result = await api.createRun({
        workspace_id: activeWorkspaceId,
        challenge_pack_version_id: selectedPackVersionId,
        name: name || undefined,
        agent_deployment_ids: selectedDeploymentIds,
      });
      router.push(`/runs/${result.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create run");
    } finally {
      setSubmitting(false);
    }
  }

  if (loading) {
    return (
      <div className="max-w-3xl space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-64" />
      </div>
    );
  }

  return (
    <div className="max-w-3xl">
      <div className="mb-4">
        <Link
          href="/"
          className="inline-flex items-center gap-1.5 text-xs text-text-3 hover:text-text-1 transition-colors"
        >
          <ArrowLeft className="size-3" />
          Back to runs
        </Link>
      </div>

      <PageHeader
        eyebrow="Create"
        title="New Run"
        description="Set up an agent evaluation run"
      />

      {loadError && (
        <div className="rounded-lg border border-status-warn/20 bg-status-warn/5 p-4 mb-6">
          <p className="text-sm text-status-warn">{loadError}</p>
          <p className="text-xs text-text-3 mt-1">
            You can still create a run by entering IDs manually below.
          </p>
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-6">
        {/* Run Name */}
        <div className="space-y-2">
          <Label className="text-xs text-text-2">Run Name (optional)</Label>
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g., fix-auth-bug baseline"
            className="max-w-md"
          />
        </div>

        {/* Challenge Pack Selection */}
        <div className="space-y-3">
          <Label className="text-xs text-text-2">Challenge Pack Version</Label>
          {challengePacks.length > 0 ? (
            <div className="grid grid-cols-1 gap-2">
              {challengePacks.map((pack) => (
                <Card key={pack.id} className="bg-card">
                  <CardHeader className="pb-2">
                    <CardTitle className="text-sm">{pack.name}</CardTitle>
                    {pack.description && (
                      <p className="text-xs text-text-3">{pack.description}</p>
                    )}
                  </CardHeader>
                  <CardContent>
                    <div className="flex flex-wrap gap-2">
                      {pack.versions.map((v) => (
                        <button
                          key={v.id}
                          type="button"
                          onClick={() => setSelectedPackVersionId(v.id)}
                          className={`
                            inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs
                            font-[family-name:var(--font-mono)] border cursor-pointer transition-colors
                            ${selectedPackVersionId === v.id
                              ? "border-ds-accent text-ds-accent bg-ds-accent/5"
                              : "border-border text-text-2 hover:border-text-3"
                            }
                          `}
                        >
                          {selectedPackVersionId === v.id && <Check className="size-3" />}
                          {v.version_label}
                        </button>
                      ))}
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          ) : (
            <div className="space-y-2">
              <Input
                value={selectedPackVersionId}
                onChange={(e) => setSelectedPackVersionId(e.target.value)}
                placeholder="Enter challenge pack version UUID"
                className="font-[family-name:var(--font-mono)] text-xs max-w-md"
              />
              <p className="text-[11px] text-text-3">
                No challenge packs found. Enter a version ID manually.
              </p>
            </div>
          )}
        </div>

        {/* Agent Deployments */}
        <div className="space-y-3">
          <Label className="text-xs text-text-2">
            Agent Deployments ({selectedDeploymentIds.length} selected)
          </Label>
          {deployments.length > 0 ? (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              {deployments.map((dep) => {
                const isSelected = selectedDeploymentIds.includes(dep.id);
                return (
                  <button
                    key={dep.id}
                    type="button"
                    onClick={() => toggleDeployment(dep.id)}
                    className={`
                      flex items-center gap-3 p-3 rounded-lg border text-left cursor-pointer transition-colors
                      ${isSelected
                        ? "border-ds-accent bg-ds-accent/5"
                        : "border-border hover:border-text-3"
                      }
                    `}
                  >
                    <div className={`
                      w-5 h-5 rounded border flex items-center justify-center shrink-0
                      ${isSelected ? "bg-ds-accent border-ds-accent" : "border-text-3"}
                    `}>
                      {isSelected && <Check className="size-3 text-bg" />}
                    </div>
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-text-1 truncate">{dep.name}</p>
                      <p className="text-[10px] text-text-3 font-[family-name:var(--font-mono)]">
                        {dep.id.slice(0, 8)}
                      </p>
                    </div>
                  </button>
                );
              })}
            </div>
          ) : (
            <div className="space-y-2">
              <div className="flex flex-wrap gap-2">
                {selectedDeploymentIds.map((id, i) => (
                  <span key={i} className="inline-flex items-center gap-1 px-2 py-1 rounded bg-surface text-xs font-[family-name:var(--font-mono)] text-text-2">
                    {id.slice(0, 8)}
                    <button type="button" onClick={() => setSelectedDeploymentIds((prev) => prev.filter((_, j) => j !== i))}>
                      <X className="size-3 text-text-3 hover:text-text-1" />
                    </button>
                  </span>
                ))}
              </div>
              <div className="flex gap-2">
                <Input
                  placeholder="Enter agent deployment UUID"
                  className="font-[family-name:var(--font-mono)] text-xs max-w-md"
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      const value = (e.target as HTMLInputElement).value.trim();
                      if (value && !selectedDeploymentIds.includes(value)) {
                        setSelectedDeploymentIds((prev) => [...prev, value]);
                        (e.target as HTMLInputElement).value = "";
                      }
                    }
                  }}
                />
              </div>
              <p className="text-[11px] text-text-3">
                No deployments found. Enter UUIDs manually (press Enter to add).
              </p>
            </div>
          )}
        </div>

        {error && (
          <div className="rounded-lg border border-status-fail/20 bg-status-fail/5 p-3">
            <p className="text-xs text-status-fail">{error}</p>
          </div>
        )}

        <div className="flex items-center gap-3 pt-2">
          <Button type="submit" disabled={submitting}>
            {submitting ? (
              <>
                <Loader2 className="size-3.5 animate-spin" />
                Creating...
              </>
            ) : (
              <>
                <Plus className="size-3.5" data-icon="inline-start" />
                Create Run
              </>
            )}
          </Button>
          <Link href="/">
            <Button type="button" variant="outline">
              Cancel
            </Button>
          </Link>
        </div>
      </form>
    </div>
  );
}
