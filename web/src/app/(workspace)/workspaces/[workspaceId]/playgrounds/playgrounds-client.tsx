"use client";

import Link from "next/link";
import { useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import type { Playground } from "@/lib/api/types";
import { useApiListQuery, useApiMutator } from "@/lib/api/swr";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { PageHeader } from "@/components/ui/page-header";
import { FlaskConical, Loader2, Plus } from "lucide-react";
import { EvalSpecBuilder } from "./[playgroundId]/components/eval-spec-builder";

const defaultEvalSpec = {
  name: "playground-default",
  version_number: 1,
  judge_mode: "deterministic",
  validators: [
    {
      key: "contains-1",
      type: "contains",
      target: "final_output",
      expected_from: "case.expectations.expected_output",
    },
  ],
  metrics: [
    {
      key: "run_total_latency_ms",
      type: "numeric",
      collector: "run_total_latency_ms",
      unit: "ms",
    },
    {
      key: "run_model_cost_usd",
      type: "numeric",
      collector: "run_model_cost_usd",
      unit: "usd",
    },
  ],
  scorecard: {
    dimensions: ["correctness", "latency", "cost"],
    normalization: {
      latency: { target_ms: 500, max_ms: 5000 },
      cost: { target_usd: 0.001, max_usd: 0.1 },
    },
  },
};

export function PlaygroundsClient({
  workspaceId,
}: {
  workspaceId: string;
}) {
  const { getAccessToken } = useAccessToken();
  const { mutate } = useApiMutator();
  const {
    data,
    error: loadError,
    isLoading,
  } = useApiListQuery<Playground>(`/v1/workspaces/${workspaceId}/playgrounds`);
  const playgrounds = data?.items ?? [];
  const [name, setName] = useState("");
  const [promptTemplate, setPromptTemplate] = useState(
    "Summarize {{topic}} in one sentence.",
  );
  const [systemPrompt, setSystemPrompt] = useState("Be precise and concise.");
  const [evalSpec, setEvalSpec] = useState<unknown>(defaultEvalSpec);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleCreatePlayground(
    event: React.FormEvent<HTMLFormElement>,
  ) {
    event.preventDefault();
    setError(null);
    setIsSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await api.post(`/v1/workspaces/${workspaceId}/playgrounds`, {
        name: name.trim(),
        prompt_template: promptTemplate,
        system_prompt: systemPrompt,
        evaluation_spec: evalSpec,
      });
      setName("");
      setPromptTemplate("Summarize {{topic}} in one sentence.");
      setSystemPrompt("Be precise and concise.");
      setEvalSpec(defaultEvalSpec);
      await mutate(workspaceResourceKeys.playgrounds(workspaceId));
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to create playground",
      );
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="space-y-8">
      <PageHeader
        title="Playgrounds"
        breadcrumbs={[{ label: "Playgrounds" }]}
      />

      <form
        onSubmit={handleCreatePlayground}
        className="rounded-lg border border-border bg-card p-5 space-y-4"
      >
        <h2 className="text-sm font-medium">Create Playground</h2>
        <div className="grid gap-4 md:grid-cols-2">
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-muted-foreground">
              Name
            </label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Prompt sandbox"
            />
          </div>
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-muted-foreground">
              System Prompt
            </label>
            <Input
              value={systemPrompt}
              onChange={(e) => setSystemPrompt(e.target.value)}
              placeholder="Be precise."
            />
          </div>
        </div>
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-muted-foreground">
            Prompt Template
          </label>
          <textarea
            value={promptTemplate}
            onChange={(e) => setPromptTemplate(e.target.value)}
            spellCheck={false}
            className="min-h-24 w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm leading-relaxed focus:outline-none focus:ring-2 focus:ring-ring/50 resize-y"
          />
        </div>
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-muted-foreground">
            Evaluation
          </label>
          <EvalSpecBuilder value={evalSpec} onChange={setEvalSpec} />
        </div>
        {error && <p className="text-sm text-destructive">{error}</p>}
        <div className="flex justify-end">
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? (
              <Loader2 className="mr-2 size-4 animate-spin" />
            ) : (
              <Plus className="mr-2 size-4" />
            )}
            {isSubmitting ? "Creating..." : "Create Playground"}
          </Button>
        </div>
      </form>

      {isLoading && !data ? (
        <WorkspaceListLoading rows={6} />
      ) : loadError ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load playgrounds.
        </div>
      ) : playgrounds.length === 0 ? (
        <EmptyState
          icon={<FlaskConical className="size-10" />}
          title="No playgrounds yet"
          description="Create a playground to iterate on prompt variants, run inline test cases, and compare experiment outputs."
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Prompt Preview</TableHead>
                <TableHead>Updated</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {playgrounds.map((playground) => (
                <TableRow key={playground.id}>
                  <TableCell>
                    <Link
                      href={`/workspaces/${workspaceId}/playgrounds/${playground.id}`}
                      className="font-medium text-foreground hover:underline underline-offset-4"
                    >
                      {playground.name}
                    </Link>
                  </TableCell>
                  <TableCell className="max-w-xl truncate text-sm text-muted-foreground">
                    {playground.prompt_template}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {new Date(playground.updated_at).toLocaleString()}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
