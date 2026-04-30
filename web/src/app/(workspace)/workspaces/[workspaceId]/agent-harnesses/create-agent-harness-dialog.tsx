"use client";

import { useState } from "react";
import Link from "next/link";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  AgentHarness,
  CreateAgentHarnessRequest,
  WorkspaceSecret,
} from "@/lib/api/types";
import { useApiListQuery, useApiMutator } from "@/lib/api/swr";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
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
import { Input } from "@/components/ui/input";
import { toast } from "sonner";
import { GitBranch, Github, Loader2, Plus } from "lucide-react";

const defaultEvaluationConfig = {
  validators: [
    {
      type: "command",
      command: "go test ./...",
    },
  ],
  llm_judges: [
    {
      key: "autonomy",
      rubric:
        "Did the coding agent complete the task with coherent, tested changes?",
    },
  ],
};

export function CreateAgentHarnessDialog({
  workspaceId,
}: {
  workspaceId: string;
}) {
  const { getAccessToken } = useAccessToken();
  const { mutate } = useApiMutator();
  const { data: secretsData, isLoading: secretsLoading } =
    useApiListQuery<WorkspaceSecret>(
      `/v1/workspaces/${workspaceId}/secrets`,
    );
  const [open, setOpen] = useState(false);
  const [taskPrompt, setTaskPrompt] = useState("");
  const [repositoryURL, setRepositoryURL] = useState("");
  const [baseBranch, setBaseBranch] = useState("main");
  const [submitting, setSubmitting] = useState(false);

  async function handleCreate() {
    const openAISecret = inferOpenAISecret(secretsData?.items ?? []);
    if (!repositoryURL.trim() || !taskPrompt.trim()) return;
    if (!openAISecret) {
      toast.error("Add OPENAI_API_KEY under workspace Secrets first");
      return;
    }

    const payload: CreateAgentHarnessRequest = {
      name: buildHarnessName(repositoryURL, taskPrompt),
      task_prompt: taskPrompt.trim(),
      codex_template: "codex",
      auth_mode: "api_key_secret",
      openai_api_key_secret_name: openAISecret,
      repository_url: repositoryURL.trim() || undefined,
      base_branch: baseBranch.trim() || undefined,
      evaluation_config: defaultEvaluationConfig,
    };

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await api.post<AgentHarness>(
        `/v1/workspaces/${workspaceId}/agent-harnesses`,
        payload,
      );
      toast.success(`Created "${payload.name}"`);
      setOpen(false);
      resetForm();
      await mutate(workspaceResourceKeys.agentHarnesses(workspaceId));
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to create harness",
      );
    } finally {
      setSubmitting(false);
    }
  }

  function resetForm() {
    setTaskPrompt("");
    setRepositoryURL("");
    setBaseBranch("main");
  }

  const openAISecret = inferOpenAISecret(secretsData?.items ?? []);
  const canSubmit =
    repositoryURL.trim() &&
    taskPrompt.trim() &&
    openAISecret &&
    !secretsLoading;

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" />}>
        <Plus data-icon="inline-start" className="size-4" />
        New Harness
      </DialogTrigger>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>New Agent Harness</DialogTitle>
          <DialogDescription>
            Point Codex at a repo and describe the work.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="grid gap-4 md:grid-cols-2">
            <div>
              <label className="mb-1.5 flex items-center gap-2 text-sm font-medium">
                <Github className="size-4 text-muted-foreground" />
                Repository URL
              </label>
              <Input
                value={repositoryURL}
                onChange={(event) => setRepositoryURL(event.target.value)}
                placeholder="https://github.com/org/repo"
                autoFocus
              />
            </div>
            <div>
              <label className="mb-1.5 flex items-center gap-2 text-sm font-medium">
                <GitBranch className="size-4 text-muted-foreground" />
                Base Branch
              </label>
              <Input
                value={baseBranch}
                onChange={(event) => setBaseBranch(event.target.value)}
                placeholder="main"
              />
            </div>
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium">Task</label>
            <textarea
              value={taskPrompt}
              onChange={(event) => setTaskPrompt(event.target.value)}
              spellCheck={false}
              className="min-h-36 w-full resize-y rounded-lg border border-input bg-transparent px-3 py-2 text-sm leading-relaxed focus:outline-none focus:ring-2 focus:ring-ring/50"
              placeholder="Implement the requested change, run the relevant tests, and summarize the diff."
            />
          </div>

          {!secretsLoading && !openAISecret ? (
            <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-200">
              Add an <code className="font-[family-name:var(--font-mono)]">OPENAI_API_KEY</code>{" "}
              workspace secret before creating a Codex harness.{" "}
              <Link
                href={`/workspaces/${workspaceId}/secrets`}
                className="font-medium underline underline-offset-4"
              >
                Open Secrets
              </Link>
            </div>
          ) : null}
        </div>

        <DialogFooter>
          <Button onClick={handleCreate} disabled={!canSubmit || submitting}>
            {submitting ? (
              <Loader2 data-icon="inline-start" className="size-4 animate-spin" />
            ) : (
              <Plus data-icon="inline-start" className="size-4" />
            )}
            {submitting ? "Creating..." : "Create Harness"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function inferOpenAISecret(secrets: WorkspaceSecret[]): string | undefined {
  const exact = secrets.find((secret) => secret.key === "OPENAI_API_KEY");
  if (exact) return exact.key;
  return secrets.find((secret) => {
    const key = secret.key.toUpperCase();
    return key.includes("OPENAI") && key.includes("KEY");
  })?.key;
}

function buildHarnessName(repositoryURL: string, taskPrompt: string): string {
  const repoName = repositoryURL
    .trim()
    .replace(/\.git$/i, "")
    .split("/")
    .filter(Boolean)
    .slice(-2)
    .join("/");
  if (repoName) {
    return `${repoName} Codex`;
  }
  return `${taskPrompt.trim().split(/\s+/).slice(0, 4).join(" ")} Codex`;
}
