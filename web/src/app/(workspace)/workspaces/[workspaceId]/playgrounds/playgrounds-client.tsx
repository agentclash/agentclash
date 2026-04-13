"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import type { Playground } from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { FlaskConical } from "lucide-react";

const defaultEvaluationSpec = JSON.stringify(
  {
    name: "playground-default",
    version_number: 1,
    judge_mode: "deterministic",
    validators: [
      {
        key: "contains-expected",
        type: "contains",
        target: "final_output",
        expected_from: "case.expectations.expected_output",
      },
    ],
    metrics: [
      { key: "latency", type: "numeric", collector: "run_total_latency_ms", unit: "ms" },
      { key: "cost", type: "numeric", collector: "run_model_cost_usd", unit: "usd" },
    ],
    scorecard: {
      dimensions: ["correctness", "latency", "cost"],
      normalization: {
        latency: { target_ms: 500, max_ms: 5000 },
        cost: { target_usd: 0.001, max_usd: 0.1 },
      },
    },
  },
  null,
  2,
);

export function PlaygroundsClient({
  workspaceId,
  initialPlaygrounds,
}: {
  workspaceId: string;
  initialPlaygrounds: Playground[];
}) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [name, setName] = useState("");
  const [promptTemplate, setPromptTemplate] = useState("Summarize {{topic}} in one sentence.");
  const [systemPrompt, setSystemPrompt] = useState("Be precise and concise.");
  const [evaluationSpec, setEvaluationSpec] = useState(defaultEvaluationSpec);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleCreatePlayground(event: React.FormEvent<HTMLFormElement>) {
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
        evaluation_spec: JSON.parse(evaluationSpec),
      });
      setName("");
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create playground");
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">Playgrounds</h1>
          <p className="mt-0.5 text-sm text-muted-foreground">
            Fast prompt experiments without publishing a full challenge pack.
          </p>
        </div>
        <Badge variant="outline">A/B prompt testing</Badge>
      </div>

      <form
        onSubmit={handleCreatePlayground}
        className="rounded-lg border border-border bg-card p-5 space-y-4"
      >
        <div className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <label className="text-sm font-medium">Name</label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Prompt sandbox" />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">System Prompt</label>
            <Input value={systemPrompt} onChange={(e) => setSystemPrompt(e.target.value)} placeholder="Be precise." />
          </div>
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium">Prompt Template</label>
          <textarea
            value={promptTemplate}
            onChange={(e) => setPromptTemplate(e.target.value)}
            className="min-h-28 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
          />
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium">Evaluation Spec JSON</label>
          <textarea
            value={evaluationSpec}
            onChange={(e) => setEvaluationSpec(e.target.value)}
            className="min-h-72 w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-xs"
          />
        </div>
        {error ? <p className="text-sm text-destructive">{error}</p> : null}
        <div className="flex justify-end">
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? "Creating..." : "Create Playground"}
          </Button>
        </div>
      </form>

      {initialPlaygrounds.length === 0 ? (
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
              {initialPlaygrounds.map((playground) => (
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
