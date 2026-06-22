"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { useApiMutator } from "@/lib/api/swr";
import { ApiError } from "@/lib/api/errors";
import { workspaceMutationKeys } from "@/lib/workspace-resource";
import { quickCreateAgent } from "@/lib/api/quick-create-agent";
import type {
  RuntimeProfile,
  ProviderAccount,
  ProviderConnectionModel,
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
import { Loader2, Sparkles } from "lucide-react";

const inputClass =
  "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50 disabled:opacity-50";
const selectClass =
  "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50 disabled:opacity-50";

/**
 * The one-step path to a runnable agent. It hides the build/version/ready/
 * deployment plumbing: the user names the agent, writes its instructions, and
 * picks a model — the backend's quick-create endpoint does the rest. The
 * granular Builds and Deployments screens remain for power users.
 */
export function QuickCreateAgentDialog({
  workspaceId,
}: {
  workspaceId: string;
}) {
  const { getAccessToken } = useAccessToken();
  const { mutateMany } = useApiMutator();

  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [instructions, setInstructions] = useState("");
  const [selectedProfileId, setSelectedProfileId] = useState("");
  const [selectedAccountId, setSelectedAccountId] = useState("");
  const [model, setModel] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const [profiles, setProfiles] = useState<RuntimeProfile[]>([]);
  const [accounts, setAccounts] = useState<ProviderAccount[]>([]);
  const [models, setModels] = useState<ProviderConnectionModel[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadingModels, setLoadingModels] = useState(false);
  const modelRequestRef = useRef(0);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const [profilesRes, accountsRes] = await Promise.all([
        api.get<{ items: RuntimeProfile[] }>(
          `/v1/workspaces/${workspaceId}/runtime-profiles`,
        ),
        api.get<{ items: ProviderAccount[] }>(
          `/v1/workspaces/${workspaceId}/provider-accounts`,
        ),
      ]);
      setProfiles(profilesRes.items);
      setAccounts(accountsRes.items);
      // The runtime profile is plumbing — default to the first one so the user
      // never has to think about it. Power users can change it under Advanced.
      if (profilesRes.items.length > 0) {
        setSelectedProfileId((current) => current || profilesRes.items[0].id);
      }
    } catch {
      toast.error("Failed to load workspace settings");
    } finally {
      setLoading(false);
    }
  }, [getAccessToken, workspaceId]);

  useEffect(() => {
    if (open) loadData();
  }, [open, loadData]);

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

  function handleAccountChange(accountId: string) {
    setSelectedAccountId(accountId);
    setModel("");
    setModels([]);
    if (accountId) loadModels(accountId);
    else {
      modelRequestRef.current += 1;
      setLoadingModels(false);
    }
  }

  function resetForm() {
    modelRequestRef.current += 1;
    setName("");
    setInstructions("");
    setSelectedProfileId("");
    setSelectedAccountId("");
    setModel("");
    setModels([]);
    setLoadingModels(false);
  }

  const needsAccount = !loading && accounts.length === 0;
  const needsProfile = !loading && profiles.length === 0;
  const canSubmit =
    name.trim() &&
    instructions.trim() &&
    selectedAccountId &&
    selectedProfileId &&
    model.trim();

  async function handleCreate() {
    if (!canSubmit) return;
    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const result = await quickCreateAgent(api, workspaceId, {
        name: name.trim(),
        instructions: instructions.trim(),
        runtime_profile_id: selectedProfileId,
        provider_account_id: selectedAccountId,
        model: model.trim(),
      });
      toast.success(`Created and deployed "${result.build.name}"`);
      setOpen(false);
      resetForm();
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to create agent",
      );
      return;
    } finally {
      setSubmitting(false);
    }
    // Revalidate the lists in the background — a refresh failure must not
    // surface as a create failure (the create already succeeded above).
    void mutateMany(workspaceMutationKeys.quickCreateAgentDialog(workspaceId));
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" />}>
        <Sparkles data-icon="inline-start" className="size-4" />
        New agent
      </DialogTrigger>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>New agent</DialogTitle>
          <DialogDescription>
            Name it, tell it what to do, and pick a model. We build, version, and
            deploy it for you — ready to race.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2 max-h-[60vh] overflow-y-auto">
          <div>
            <label className="mb-1.5 block text-sm font-medium">Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Refund resolver"
              autoFocus
              className={inputClass}
            />
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Instructions
            </label>
            <textarea
              value={instructions}
              onChange={(e) => setInstructions(e.target.value)}
              placeholder="Describe how the agent should behave. This becomes its system prompt."
              rows={5}
              className={`${inputClass} resize-y`}
            />
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Provider account
            </label>
            <select
              value={selectedAccountId}
              onChange={(e) => handleAccountChange(e.target.value)}
              disabled={loading}
              className={selectClass}
            >
              <option value="">
                {loading
                  ? "Loading..."
                  : needsAccount
                    ? "No accounts — create one first"
                    : "Select a provider account"}
              </option>
              {accounts.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name} ({a.provider_key})
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium">Model</label>
            {models.length > 0 ? (
              <select
                value={model}
                onChange={(e) => setModel(e.target.value)}
                disabled={loadingModels}
                className={selectClass}
              >
                <option value="">Select a model</option>
                {models.map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.display_name} ({m.id})
                  </option>
                ))}
              </select>
            ) : (
              <input
                type="text"
                value={model}
                onChange={(e) => setModel(e.target.value)}
                placeholder="e.g. gpt-4.1, claude-sonnet-4-6"
                disabled={loadingModels}
                className={inputClass}
              />
            )}
            <p className="mt-1 text-xs text-muted-foreground">
              {loadingModels
                ? "Loading available models…"
                : !selectedAccountId
                  ? "Select a provider account to load its models."
                  : models.length > 0
                    ? "Pick a model from this provider connection."
                    : "Enter the provider model ID."}
            </p>
          </div>

          <details className="rounded-lg border border-border/60 px-3 py-2">
            <summary className="cursor-pointer text-sm font-medium text-muted-foreground">
              Advanced
            </summary>
            <div className="mt-3">
              <label className="mb-1.5 block text-sm font-medium">
                Runtime profile
              </label>
              <select
                value={selectedProfileId}
                onChange={(e) => setSelectedProfileId(e.target.value)}
                disabled={loading}
                className={selectClass}
              >
                <option value="">
                  {loading
                    ? "Loading..."
                    : needsProfile
                      ? "No profiles — create one first"
                      : "Select a profile"}
                </option>
                {profiles.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name} ({p.execution_target})
                  </option>
                ))}
              </select>
              <p className="mt-1 text-xs text-muted-foreground">
                Controls iteration and timeout limits. Defaults to your first
                profile.
              </p>
            </div>
          </details>

          {needsAccount || needsProfile ? (
            <p className="text-xs text-destructive">
              This workspace needs a{needsAccount ? " provider account" : ""}
              {needsAccount && needsProfile ? " and a" : ""}
              {needsProfile ? " runtime profile" : ""} before you can create an
              agent.
            </p>
          ) : null}
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
              "Create agent"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
