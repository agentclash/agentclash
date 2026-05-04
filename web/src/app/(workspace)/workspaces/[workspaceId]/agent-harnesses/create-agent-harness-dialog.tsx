"use client";

import { useState } from "react";
import Link from "next/link";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  AgentHarness,
  CreateAgentHarnessRequest,
  GitHubInstallation,
  GitHubRepository,
  StartGitHubInstallationResponse,
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

type HarnessKind = "codex_e2b" | "openclaw_e2b";

const harnessOptions: Record<
  HarnessKind,
  {
    label: string;
    template: string;
    secretCandidates: string[];
  }
> = {
  codex_e2b: {
    label: "Codex",
    template: "codex",
    secretCandidates: ["OPENAI_API_KEY"],
  },
  openclaw_e2b: {
    label: "OpenClaw",
    template: "agentclash-openclaw-fullstack",
    secretCandidates: ["OPENAI_API_KEY", "ANTHROPIC_API_KEY", "OPENROUTER_API_KEY"],
  },
};

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
  const { data: installationsData } = useApiListQuery<GitHubInstallation>(
    `/v1/workspaces/${workspaceId}/github/installations`,
  );
  const { data: repositoriesData } = useApiListQuery<GitHubRepository>(
    `/v1/workspaces/${workspaceId}/github/repositories`,
  );
  const [open, setOpen] = useState(false);
  const [harnessKind, setHarnessKind] = useState<HarnessKind>("codex_e2b");
  const [taskPrompt, setTaskPrompt] = useState("");
  const [sourceMode, setSourceMode] = useState<"github" | "url">("github");
  const [selectedRepositoryID, setSelectedRepositoryID] = useState("");
  const [repositoryURL, setRepositoryURL] = useState("");
  const [baseBranch, setBaseBranch] = useState("main");
  const [submitting, setSubmitting] = useState(false);
  const [connectingGitHub, setConnectingGitHub] = useState(false);

  async function handleConnectGitHub() {
    setConnectingGitHub(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const result = await api.post<StartGitHubInstallationResponse>(
        `/v1/workspaces/${workspaceId}/github/installations/start`,
        { return_path: `/workspaces/${workspaceId}/agent-harnesses` },
      );
      window.location.assign(result.install_url);
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to connect GitHub",
      );
      setConnectingGitHub(false);
    }
  }

  async function handleCreate() {
    const selectedRepository = githubRepositories.find(
      (repo) => String(repo.github_repository_id) === selectedRepositoryID,
    );
    if (sourceMode === "github" && !selectedRepository) return;
    if (sourceMode === "url" && !repositoryURL.trim()) return;
    if (!taskPrompt.trim()) return;
    if (!apiKeySecret) {
      toast.error(`Add ${runner.secretCandidates[0]} under workspace Secrets first`);
      return;
    }

    const payload: CreateAgentHarnessRequest = {
      name: buildHarnessName(
        sourceMode === "github" ? selectedRepository?.full_name ?? "" : repositoryURL,
        taskPrompt,
        runner.label,
      ),
      harness_kind: harnessKind,
      task_prompt: taskPrompt.trim(),
      codex_template: runner.template,
      auth_mode: "api_key_secret",
      openai_api_key_secret_name: apiKeySecret,
      base_branch:
        baseBranch.trim() ||
        (sourceMode === "github" ? selectedRepository?.default_branch : undefined),
      evaluation_config: defaultEvaluationConfig,
    };
    if (sourceMode === "github" && selectedRepository) {
      payload.repository_provider = "github";
      payload.github_repository_id = selectedRepository.github_repository_id;
      payload.github_installation_id = selectedRepository.github_installation_id;
      payload.repository_url = selectedRepository.html_url;
    } else {
      payload.repository_url = repositoryURL.trim() || undefined;
    }

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
    setHarnessKind("codex_e2b");
    setTaskPrompt("");
    setSourceMode("github");
    setSelectedRepositoryID("");
    setRepositoryURL("");
    setBaseBranch("main");
  }

  const runner = harnessOptions[harnessKind];
  const apiKeySecret = inferRunnerSecret(secretsData?.items ?? [], runner.secretCandidates);
  const githubInstallations = installationsData?.items ?? [];
  const githubRepositories = repositoriesData?.items ?? [];
  const selectedRepository = githubRepositories.find(
    (repo) => String(repo.github_repository_id) === selectedRepositoryID,
  );
  const hasGitHubRepositories = githubRepositories.length > 0;
  const canSubmit =
    (sourceMode === "github"
      ? Boolean(selectedRepository)
      : Boolean(repositoryURL.trim())) &&
    taskPrompt.trim() &&
    apiKeySecret &&
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
            Point a coding harness at a repo and describe the work.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div>
            <label className="mb-1.5 block text-sm font-medium">Runner</label>
            <select
              value={harnessKind}
              onChange={(event) => setHarnessKind(event.target.value as HarnessKind)}
              className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring/50"
            >
              {Object.entries(harnessOptions).map(([kind, option]) => (
                <option key={kind} value={kind}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>

          <div className="flex gap-2">
            <Button
              type="button"
              variant={sourceMode === "github" ? "default" : "outline"}
              onClick={() => setSourceMode("github")}
            >
              <Github data-icon="inline-start" className="size-4" />
              GitHub
            </Button>
            <Button
              type="button"
              variant={sourceMode === "url" ? "default" : "outline"}
              onClick={() => setSourceMode("url")}
            >
              URL
            </Button>
          </div>

          {sourceMode === "github" ? (
            <>
              <div className="grid gap-4 md:grid-cols-2">
                <div>
                  <label className="mb-1.5 flex items-center gap-2 text-sm font-medium">
                    <Github className="size-4 text-muted-foreground" />
                    Repository
                  </label>
                  <select
                    value={selectedRepositoryID}
                    onChange={(event) => {
                      const nextID = event.target.value;
                      setSelectedRepositoryID(nextID);
                      const nextRepo = githubRepositories.find(
                        (repo) => String(repo.github_repository_id) === nextID,
                      );
                      if (nextRepo) setBaseBranch(nextRepo.default_branch);
                    }}
                    className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring/50"
                    autoFocus
                  >
                    <option value="">
                      {hasGitHubRepositories
                        ? "Select repository"
                        : githubInstallations.length > 0
                          ? "No repositories connected"
                          : "Connect GitHub first"}
                    </option>
                    {githubRepositories.map((repo) => (
                      <option
                        key={`${repo.github_installation_id}:${repo.github_repository_id}`}
                        value={repo.github_repository_id}
                      >
                        {repo.full_name}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="mb-1.5 flex items-center gap-2 text-sm font-medium">
                    <GitBranch className="size-4 text-muted-foreground" />
                    Base Branch
                  </label>
                  <Input
                    value={baseBranch}
                    onChange={(event) => setBaseBranch(event.target.value)}
                    placeholder={selectedRepository?.default_branch ?? "main"}
                  />
                </div>
              </div>
              {!hasGitHubRepositories ? (
                <Button
                  type="button"
                  variant="outline"
                  onClick={handleConnectGitHub}
                  disabled={connectingGitHub}
                >
                  {connectingGitHub ? (
                    <Loader2 data-icon="inline-start" className="size-4 animate-spin" />
                  ) : (
                    <Github data-icon="inline-start" className="size-4" />
                  )}
                  {connectingGitHub ? "Connecting..." : "Connect GitHub"}
                </Button>
              ) : null}
            </>
          ) : (
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
          )}

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

          {!secretsLoading && !apiKeySecret ? (
            <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-200">
              Add an{" "}
              <code className="font-[family-name:var(--font-mono)]">
                {runner.secretCandidates[0]}
              </code>{" "}
              workspace secret before creating a {runner.label} harness.{" "}
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

function inferRunnerSecret(
  secrets: WorkspaceSecret[],
  candidates: string[],
): string | undefined {
  for (const candidate of candidates) {
    const exact = secrets.find((secret) => secret.key === candidate);
    if (exact) return exact.key;
  }
  return secrets.find((secret) => {
    const key = secret.key.toUpperCase();
    return candidates.some((candidate) => {
      const provider = candidate.split("_")[0];
      return key.includes(provider) && key.includes("KEY");
    });
  })?.key;
}

function buildHarnessName(
  repositoryURL: string,
  taskPrompt: string,
  runnerLabel: string,
): string {
  const repoName = parseRepositoryName(repositoryURL);
  if (repoName) {
    return `${repoName} ${runnerLabel}`;
  }
  return `${taskPrompt.trim().split(/\s+/).slice(0, 4).join(" ")} ${runnerLabel}`;
}

function parseRepositoryName(repositoryURL: string): string | undefined {
  const trimmed = repositoryURL.trim().replace(/\.git$/i, "");
  const scpPath = trimmed.match(/^[^@]+@[^:]+:(.+)$/)?.[1];
  if (scpPath) {
    const segments = scpPath.split("/").filter(Boolean);
    return segments.length >= 2 ? segments.slice(-2).join("/") : undefined;
  }

  try {
    const url = new URL(trimmed);
    const segments = url.pathname.split("/").filter(Boolean);
    return segments.length >= 2 ? segments.slice(-2).join("/") : undefined;
  } catch {
    const segments = trimmed.split("/").filter(Boolean);
    return segments.length >= 2 ? segments.slice(-2).join("/") : undefined;
  }
}
