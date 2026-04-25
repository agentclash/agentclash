"use client";

import { useState, useEffect, useCallback } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { useApiMutator } from "@/lib/api/swr";
import { ApiError } from "@/lib/api/errors";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
import type {
  AgentBuild,
  AgentBuildDetail,
  AgentBuildVersion,
  AgentDeploymentCreateResponse,
  RuntimeProfile,
  ProviderAccount,
  ModelAlias,
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
import { toast } from "sonner";
import { Loader2, Plus } from "lucide-react";

interface CreateDeploymentDialogProps {
  workspaceId: string;
}

export function CreateDeploymentDialog({
  workspaceId,
}: CreateDeploymentDialogProps) {
  const { getAccessToken } = useAccessToken();
  const { mutate } = useApiMutator();

  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [selectedBuildId, setSelectedBuildId] = useState("");
  const [selectedVersionId, setSelectedVersionId] = useState("");
  const [selectedProfileId, setSelectedProfileId] = useState("");
  const [selectedAccountId, setSelectedAccountId] = useState("");
  const [model, setModel] = useState("");
  const [selectedAliasId, setSelectedAliasId] = useState("");
  const [deploymentConfig, setDeploymentConfig] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const [builds, setBuilds] = useState<AgentBuild[]>([]);
  const [readyVersions, setReadyVersions] = useState<AgentBuildVersion[]>([]);
  const [profiles, setProfiles] = useState<RuntimeProfile[]>([]);
  const [accounts, setAccounts] = useState<ProviderAccount[]>([]);
  const [aliases, setAliases] = useState<ModelAlias[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadingVersions, setLoadingVersions] = useState(false);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const [buildsRes, profilesRes, accountsRes, aliasesRes] =
        await Promise.all([
          api.get<{ items: AgentBuild[] }>(`/v1/workspaces/${workspaceId}/agent-builds`),
          api.get<{ items: RuntimeProfile[] }>(`/v1/workspaces/${workspaceId}/runtime-profiles`),
          api.get<{ items: ProviderAccount[] }>(`/v1/workspaces/${workspaceId}/provider-accounts`),
          api.get<{ items: ModelAlias[] }>(`/v1/workspaces/${workspaceId}/model-aliases`),
        ]);
      setBuilds(buildsRes.items);
      setProfiles(profilesRes.items);
      setAccounts(accountsRes.items);
      setAliases(aliasesRes.items);
    } catch {
      toast.error("Failed to load data");
    } finally {
      setLoading(false);
    }
  }, [getAccessToken, workspaceId]);

  useEffect(() => {
    if (open) loadData();
  }, [open, loadData]);

  const loadVersions = useCallback(
    async (buildId: string) => {
      setLoadingVersions(true);
      setReadyVersions([]);
      setSelectedVersionId("");
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const build = await api.get<AgentBuildDetail>(`/v1/agent-builds/${buildId}`);
        const ready = build.versions.filter((v) => v.version_status === "ready");
        setReadyVersions(ready);
        if (ready.length === 1) setSelectedVersionId(ready[0].id);
      } catch {
        toast.error("Failed to load versions");
      } finally {
        setLoadingVersions(false);
      }
    },
    [getAccessToken],
  );

  function handleBuildChange(buildId: string) {
    setSelectedBuildId(buildId);
    if (buildId) loadVersions(buildId);
    else {
      setReadyVersions([]);
      setSelectedVersionId("");
    }
  }

  async function handleCreate() {
    if (!name.trim() || !selectedBuildId || !selectedVersionId || !selectedProfileId || !selectedAccountId) return;
    if (!model.trim() && !selectedAliasId) return;

    let configJson: unknown = undefined;
    if (deploymentConfig.trim()) {
      try {
        configJson = JSON.parse(deploymentConfig);
      } catch {
        toast.error("Invalid JSON in deployment config");
        return;
      }
    }

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await api.post<AgentDeploymentCreateResponse>(
        `/v1/workspaces/${workspaceId}/agent-deployments`,
        {
          name: name.trim(),
          agent_build_id: selectedBuildId,
          build_version_id: selectedVersionId,
          runtime_profile_id: selectedProfileId,
          provider_account_id: selectedAccountId,
          model_alias_id: selectedAliasId || undefined,
          model: model.trim() || undefined,
          deployment_config: configJson,
        },
      );
      toast.success(`Deployed "${name.trim()}"`);
      setOpen(false);
      resetForm();
      await mutate(workspaceResourceKeys.deployments(workspaceId));
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to create deployment");
    } finally {
      setSubmitting(false);
    }
  }

  function resetForm() {
    setName("");
    setSelectedBuildId("");
    setSelectedVersionId("");
    setSelectedProfileId("");
    setSelectedAccountId("");
    setModel("");
    setSelectedAliasId("");
    setDeploymentConfig("");
    setReadyVersions([]);
  }

  const selectClass =
    "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50 disabled:opacity-50";
  const hasModel = model.trim() || selectedAliasId;
  const canSubmit = name.trim() && selectedBuildId && selectedVersionId && selectedProfileId && selectedAccountId && hasModel;

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" />}>
        <Plus data-icon="inline-start" className="size-4" />
        New Deployment
      </DialogTrigger>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>New Deployment</DialogTitle>
          <DialogDescription>
            Deploy a ready agent build version to make it runnable.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2 max-h-[60vh] overflow-y-auto">
          <div>
            <label className="mb-1.5 block text-sm font-medium">Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. code-review-prod"
              autoFocus
              className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50"
            />
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium">Agent Build</label>
            <select value={selectedBuildId} onChange={(e) => handleBuildChange(e.target.value)} disabled={loading} className={selectClass}>
              <option value="">{loading ? "Loading..." : "Select a build"}</option>
              {builds.map((b) => (
                <option key={b.id} value={b.id}>{b.name}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Build Version <span className="text-muted-foreground font-normal">(only ready)</span>
            </label>
            <select value={selectedVersionId} onChange={(e) => setSelectedVersionId(e.target.value)} disabled={!selectedBuildId || loadingVersions} className={selectClass}>
              <option value="">
                {loadingVersions ? "Loading..." : readyVersions.length === 0 && selectedBuildId ? "No ready versions" : "Select a version"}
              </option>
              {readyVersions.map((v) => (
                <option key={v.id} value={v.id}>v{v.version_number} — {v.agent_kind}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium">Runtime Profile</label>
            <select value={selectedProfileId} onChange={(e) => setSelectedProfileId(e.target.value)} disabled={loading} className={selectClass}>
              <option value="">{loading ? "Loading..." : profiles.length === 0 ? "No profiles — create one first" : "Select a profile"}</option>
              {profiles.map((p) => (
                <option key={p.id} value={p.id}>{p.name} ({p.execution_target})</option>
              ))}
            </select>
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium">Provider Account</label>
            <select value={selectedAccountId} onChange={(e) => setSelectedAccountId(e.target.value)} disabled={loading} className={selectClass}>
              <option value="">{loading ? "Loading..." : accounts.length === 0 ? "No accounts — create one first" : "Select a provider account"}</option>
              {accounts.map((a) => (
                <option key={a.id} value={a.id}>{a.name} ({a.provider_key})</option>
              ))}
            </select>
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium">Model</label>
            <input
              type="text"
              value={model}
              onChange={(e) => { setModel(e.target.value); if (e.target.value.trim()) setSelectedAliasId(""); }}
              placeholder="e.g. gpt-4.1, claude-sonnet-4-6"
              disabled={!!selectedAliasId}
              className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50 disabled:opacity-50"
            />
            <p className="mt-1 text-xs text-muted-foreground">
              The provider model ID. Or pick an existing model alias below.
            </p>
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Model Alias <span className="text-muted-foreground font-normal">(advanced)</span>
            </label>
            <select
              value={selectedAliasId}
              onChange={(e) => { setSelectedAliasId(e.target.value); if (e.target.value) setModel(""); }}
              disabled={loading}
              className={selectClass}
            >
              <option value="">None — use model name above</option>
              {aliases.map((a) => (
                <option key={a.id} value={a.id}>{a.display_name} ({a.alias_key})</option>
              ))}
            </select>
          </div>

          <JsonField
            label="Deployment Config (optional)"
            value={deploymentConfig}
            onChange={setDeploymentConfig}
            rows={4}
            description="Free-form JSON configuration for this deployment."
          />
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)} disabled={submitting}>
            Cancel
          </Button>
          <Button disabled={!canSubmit || submitting} onClick={handleCreate}>
            {submitting ? <Loader2 className="size-4 animate-spin" /> : "Deploy"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
