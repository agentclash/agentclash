"use client";

import { useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { AgentHarness, CreateAgentHarnessRequest } from "@/lib/api/types";
import { useApiMutator } from "@/lib/api/swr";
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
import { JsonField } from "@/components/ui/json-field";
import { toast } from "sonner";
import { Loader2, Plus } from "lucide-react";

const defaultEvaluationConfig = `{
  "validators": [
    {
      "type": "command",
      "command": "go test ./..."
    }
  ],
  "llm_judges": [
    {
      "key": "autonomy",
      "rubric": "Did the coding agent complete the task with coherent, tested changes?"
    }
  ]
}`;

export function CreateAgentHarnessDialog({
  workspaceId,
}: {
  workspaceId: string;
}) {
  const { getAccessToken } = useAccessToken();
  const { mutate } = useApiMutator();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [taskPrompt, setTaskPrompt] = useState("");
  const [openAISecret, setOpenAISecret] = useState("");
  const [repositoryURL, setRepositoryURL] = useState("");
  const [baseBranch, setBaseBranch] = useState("main");
  const [codexTemplate, setCodexTemplate] = useState("codex");
  const [codexModel, setCodexModel] = useState("");
  const [evaluationConfig, setEvaluationConfig] = useState(
    defaultEvaluationConfig,
  );
  const [jsonError, setJsonError] = useState<string | undefined>();
  const [submitting, setSubmitting] = useState(false);

  async function handleCreate() {
    const parsedEvaluationConfig = parseEvaluationConfig();
    if (parsedEvaluationConfig === undefined) return;
    if (!name.trim() || !taskPrompt.trim()) return;
    if (!openAISecret.trim()) {
      toast.error("OpenAI secret is required for API-key auth");
      return;
    }

    const payload: CreateAgentHarnessRequest = {
      name: name.trim(),
      description: description.trim() || undefined,
      task_prompt: taskPrompt.trim(),
      codex_template: codexTemplate.trim() || "codex",
      codex_model: codexModel.trim() || undefined,
      auth_mode: "api_key_secret",
      openai_api_key_secret_name: openAISecret.trim() || undefined,
      repository_url: repositoryURL.trim() || undefined,
      base_branch: baseBranch.trim() || undefined,
      evaluation_config: parsedEvaluationConfig,
    };

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await api.post<AgentHarness>(
        `/v1/workspaces/${workspaceId}/agent-harnesses`,
        payload,
      );
      toast.success(`Created "${name.trim()}"`);
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

  function parseEvaluationConfig(): unknown | undefined {
    if (!evaluationConfig.trim()) {
      setJsonError(undefined);
      return {};
    }
    try {
      const parsed = JSON.parse(evaluationConfig);
      setJsonError(undefined);
      return parsed;
    } catch {
      setJsonError("Evaluation config must be valid JSON.");
      return undefined;
    }
  }

  function resetForm() {
    setName("");
    setDescription("");
    setTaskPrompt("");
    setOpenAISecret("");
    setRepositoryURL("");
    setBaseBranch("main");
    setCodexTemplate("codex");
    setCodexModel("");
    setEvaluationConfig(defaultEvaluationConfig);
    setJsonError(undefined);
  }

  const canSubmit =
    name.trim() &&
    taskPrompt.trim() &&
    openAISecret.trim();

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
            Define a Codex-on-E2B coding task and its evaluation hooks.
          </DialogDescription>
        </DialogHeader>

        <div className="max-h-[68vh] space-y-4 overflow-y-auto py-2">
          <div className="grid gap-4 md:grid-cols-2">
            <div>
              <label className="mb-1.5 block text-sm font-medium">Name</label>
              <Input
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="Codex repo autonomy check"
                autoFocus
              />
            </div>
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                OpenAI Secret
              </label>
              <Input
                value={openAISecret}
                onChange={(event) => setOpenAISecret(event.target.value)}
                placeholder="OPENAI_API_KEY"
              />
            </div>
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Description
            </label>
            <Input
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder="Long-running task harness for repository changes"
            />
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium">Task</label>
            <textarea
              value={taskPrompt}
              onChange={(event) => setTaskPrompt(event.target.value)}
              spellCheck={false}
              className="min-h-28 w-full resize-y rounded-lg border border-input bg-transparent px-3 py-2 text-sm leading-relaxed focus:outline-none focus:ring-2 focus:ring-ring/50"
              placeholder="Clone the repository, implement the requested change, run tests, and summarize the diff."
            />
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Repository URL
              </label>
              <Input
                value={repositoryURL}
                onChange={(event) => setRepositoryURL(event.target.value)}
                placeholder="https://github.com/org/repo"
              />
            </div>
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Base Branch
              </label>
              <Input
                value={baseBranch}
                onChange={(event) => setBaseBranch(event.target.value)}
                placeholder="main"
              />
            </div>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                E2B Template
              </label>
              <Input
                value={codexTemplate}
                onChange={(event) => setCodexTemplate(event.target.value)}
                placeholder="codex"
              />
            </div>
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Codex Model
              </label>
              <Input
                value={codexModel}
                onChange={(event) => setCodexModel(event.target.value)}
                placeholder="Use Codex default"
              />
            </div>
          </div>

          <JsonField
            label="Evaluation Config"
            value={evaluationConfig}
            onChange={setEvaluationConfig}
            error={jsonError}
            rows={8}
            description="Validators and LLM judges stored with the harness."
          />
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
