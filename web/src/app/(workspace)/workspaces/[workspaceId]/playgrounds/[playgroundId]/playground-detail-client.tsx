"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import type {
  ModelAlias,
  Playground,
  PlaygroundExperiment,
  PlaygroundExperimentComparison,
  PlaygroundExperimentResult,
  PlaygroundTestCase,
  ProviderAccount,
} from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";

function prettyJSON(value: unknown) {
  return JSON.stringify(value ?? {}, null, 2);
}

function statusVariant(status: string): "default" | "secondary" | "destructive" | "outline" {
  switch (status) {
    case "completed":
      return "default";
    case "running":
      return "secondary";
    case "failed":
      return "destructive";
    default:
      return "outline";
  }
}

export function PlaygroundDetailClient({
  workspaceId,
  playground,
  testCases,
  experiments,
  providerAccounts,
  modelAliases,
  selectedExperimentResults,
  selectedExperimentId,
  comparison,
  baselineExperimentId,
  candidateExperimentId,
}: {
  workspaceId: string;
  playground: Playground;
  testCases: PlaygroundTestCase[];
  experiments: PlaygroundExperiment[];
  providerAccounts: ProviderAccount[];
  modelAliases: ModelAlias[];
  selectedExperimentResults: PlaygroundExperimentResult[] | null;
  selectedExperimentId: string | null;
  comparison: PlaygroundExperimentComparison | null;
  baselineExperimentId: string | null;
  candidateExperimentId: string | null;
}) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();

  const [name, setName] = useState(playground.name);
  const [promptTemplate, setPromptTemplate] = useState(playground.prompt_template);
  const [systemPrompt, setSystemPrompt] = useState(playground.system_prompt);
  const [evaluationSpec, setEvaluationSpec] = useState(prettyJSON(playground.evaluation_spec));
  const [newCaseKey, setNewCaseKey] = useState("");
  const [newCaseVariables, setNewCaseVariables] = useState('{\n  "topic": "AgentClash"\n}');
  const [newCaseExpectations, setNewCaseExpectations] = useState('{\n  "expected_output": "AgentClash"\n}');
  const [experimentName, setExperimentName] = useState("");
  const [providerAccountId, setProviderAccountId] = useState(providerAccounts[0]?.id ?? "");
  const [modelAliasId, setModelAliasId] = useState(modelAliases[0]?.id ?? "");
  const [baselineId, setBaselineId] = useState(baselineExperimentId ?? experiments[0]?.id ?? "");
  const [candidateId, setCandidateId] = useState(candidateExperimentId ?? experiments[1]?.id ?? experiments[0]?.id ?? "");
  const [error, setError] = useState<string | null>(null);
  const [pendingAction, setPendingAction] = useState<string | null>(null);

  async function withApi<T>(action: string, fn: (api: ReturnType<typeof createApiClient>) => Promise<T>) {
    setError(null);
    setPendingAction(action);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      return await fn(api);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Request failed");
      throw err;
    } finally {
      setPendingAction(null);
    }
  }

  async function handleUpdatePlayground(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await withApi("update-playground", async (api) => {
      await api.patch(`/v1/playgrounds/${playground.id}`, {
        name,
        prompt_template: promptTemplate,
        system_prompt: systemPrompt,
        evaluation_spec: JSON.parse(evaluationSpec),
      });
      router.refresh();
    }).catch(() => undefined);
  }

  async function handleDeletePlayground() {
    if (!window.confirm("Delete this playground and all of its experiments?")) return;
    await withApi("delete-playground", async (api) => {
      await api.del(`/v1/playgrounds/${playground.id}`);
      router.push(`/workspaces/${workspaceId}/playgrounds`);
    }).catch(() => undefined);
  }

  async function handleCreateTestCase(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await withApi("create-test-case", async (api) => {
      await api.post(`/v1/playgrounds/${playground.id}/test-cases`, {
        case_key: newCaseKey,
        variables: JSON.parse(newCaseVariables),
        expectations: JSON.parse(newCaseExpectations),
      });
      setNewCaseKey("");
      router.refresh();
    }).catch(() => undefined);
  }

  async function handleUpdateTestCase(event: React.FormEvent<HTMLFormElement>, testCaseId: string) {
    event.preventDefault();
    const formData = new FormData(event.currentTarget);
    await withApi(`update-test-case-${testCaseId}`, async (api) => {
      await api.patch(`/v1/playground-test-cases/${testCaseId}`, {
        case_key: String(formData.get("case_key") ?? ""),
        variables: JSON.parse(String(formData.get("variables") ?? "{}")),
        expectations: JSON.parse(String(formData.get("expectations") ?? "{}")),
      });
      router.refresh();
    }).catch(() => undefined);
  }

  async function handleDeleteTestCase(testCaseId: string) {
    if (!window.confirm("Delete this test case?")) return;
    await withApi(`delete-test-case-${testCaseId}`, async (api) => {
      await api.del(`/v1/playground-test-cases/${testCaseId}`);
      router.refresh();
    }).catch(() => undefined);
  }

  async function handleLaunchExperiment(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await withApi("launch-experiment", async (api) => {
      await api.post(`/v1/playgrounds/${playground.id}/experiments`, {
        name: experimentName,
        provider_account_id: providerAccountId,
        model_alias_id: modelAliasId,
        request_config: {
          trace_mode: "required",
          step_timeout_ms: 120000,
        },
      });
      setExperimentName("");
      router.refresh();
    }).catch(() => undefined);
  }

  function openResults(experimentId: string) {
    const params = new URLSearchParams();
    params.set("experiment", experimentId);
    router.push(`/workspaces/${workspaceId}/playgrounds/${playground.id}?${params.toString()}`);
  }

  function openComparison() {
    if (!baselineId || !candidateId) return;
    const params = new URLSearchParams();
    params.set("baseline", baselineId);
    params.set("candidate", candidateId);
    router.push(`/workspaces/${workspaceId}/playgrounds/${playground.id}?${params.toString()}`);
  }

  return (
    <div className="space-y-8">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">{playground.name}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Edit the prompt, manage inline cases, launch experiments, and compare outputs.
          </p>
        </div>
        <Button variant="destructive" onClick={handleDeletePlayground}>
          Delete Playground
        </Button>
      </div>

      {error ? (
        <div className="rounded-md border border-destructive/20 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      <form onSubmit={handleUpdatePlayground} className="rounded-lg border border-border bg-card p-5 space-y-4">
        <div className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <label className="text-sm font-medium">Name</label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">System Prompt</label>
            <Input value={systemPrompt} onChange={(e) => setSystemPrompt(e.target.value)} />
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
        <div className="flex justify-end">
          <Button type="submit" disabled={pendingAction === "update-playground"}>
            {pendingAction === "update-playground" ? "Saving..." : "Save Playground"}
          </Button>
        </div>
      </form>

      <section className="rounded-lg border border-border bg-card p-5 space-y-4">
        <div>
          <h2 className="text-base font-semibold">Test Cases</h2>
          <p className="text-sm text-muted-foreground">
            Inline variables and expectations for this playground.
          </p>
        </div>

        <form onSubmit={handleCreateTestCase} className="grid gap-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">Case Key</label>
            <Input value={newCaseKey} onChange={(e) => setNewCaseKey(e.target.value)} placeholder="summary-case-1" />
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <label className="text-sm font-medium">Variables JSON</label>
              <textarea
                value={newCaseVariables}
                onChange={(e) => setNewCaseVariables(e.target.value)}
                className="min-h-40 w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-xs"
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Expectations JSON</label>
              <textarea
                value={newCaseExpectations}
                onChange={(e) => setNewCaseExpectations(e.target.value)}
                className="min-h-40 w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-xs"
              />
            </div>
          </div>
          <div className="flex justify-end">
            <Button type="submit" disabled={pendingAction === "create-test-case"}>
              {pendingAction === "create-test-case" ? "Adding..." : "Add Test Case"}
            </Button>
          </div>
        </form>

        <div className="space-y-4">
          {testCases.map((testCase) => (
            <form
              key={testCase.id}
              onSubmit={(event) => handleUpdateTestCase(event, testCase.id)}
              className="rounded-md border border-border p-4 space-y-3"
            >
              <div className="flex items-center justify-between gap-4">
                <Input name="case_key" defaultValue={testCase.case_key} className="max-w-sm" />
                <Button type="button" variant="outline" onClick={() => handleDeleteTestCase(testCase.id)}>
                  Delete
                </Button>
              </div>
              <div className="grid gap-4 md:grid-cols-2">
                <textarea
                  name="variables"
                  defaultValue={prettyJSON(testCase.variables)}
                  className="min-h-36 w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-xs"
                />
                <textarea
                  name="expectations"
                  defaultValue={prettyJSON(testCase.expectations)}
                  className="min-h-36 w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-xs"
                />
              </div>
              <div className="flex justify-end">
                <Button type="submit" variant="secondary">
                  Save Test Case
                </Button>
              </div>
            </form>
          ))}
        </div>
      </section>

      <section className="rounded-lg border border-border bg-card p-5 space-y-4">
        <div>
          <h2 className="text-base font-semibold">Experiments</h2>
          <p className="text-sm text-muted-foreground">
            Run the current prompt against a chosen provider account and model alias.
          </p>
        </div>

        <form onSubmit={handleLaunchExperiment} className="grid gap-4 md:grid-cols-4">
          <div className="space-y-2 md:col-span-2">
            <label className="text-sm font-medium">Experiment Name</label>
            <Input value={experimentName} onChange={(e) => setExperimentName(e.target.value)} placeholder="Optional" />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">Provider Account</label>
            <select
              value={providerAccountId}
              onChange={(e) => setProviderAccountId(e.target.value)}
              className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
            >
              {providerAccounts.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.name}
                </option>
              ))}
            </select>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">Model Alias</label>
            <select
              value={modelAliasId}
              onChange={(e) => setModelAliasId(e.target.value)}
              className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
            >
              {modelAliases.map((alias) => (
                <option key={alias.id} value={alias.id}>
                  {alias.display_name}
                </option>
              ))}
            </select>
          </div>
          <div className="md:col-span-4 flex justify-end">
            <Button type="submit" disabled={pendingAction === "launch-experiment"}>
              {pendingAction === "launch-experiment" ? "Launching..." : "Launch Experiment"}
            </Button>
          </div>
        </form>

        <div className="space-y-3">
          {experiments.map((experiment) => (
            <div
              key={experiment.id}
              className="flex flex-col gap-3 rounded-md border border-border p-4 md:flex-row md:items-center md:justify-between"
            >
              <div className="space-y-1">
                <div className="flex items-center gap-3">
                  <span className="font-medium">{experiment.name}</span>
                  <Badge variant={statusVariant(experiment.status)}>{experiment.status}</Badge>
                </div>
                <p className="text-sm text-muted-foreground">
                  Queued {experiment.queued_at ? new Date(experiment.queued_at).toLocaleString() : "just now"}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button variant="outline" onClick={() => openResults(experiment.id)}>
                  {selectedExperimentId === experiment.id ? "Viewing Results" : "View Results"}
                </Button>
              </div>
            </div>
          ))}
        </div>

        {experiments.length >= 2 ? (
          <div className="rounded-md border border-border p-4 space-y-3">
            <h3 className="font-medium">Compare Experiments</h3>
            <div className="grid gap-4 md:grid-cols-2">
              <select
                value={baselineId}
                onChange={(e) => setBaselineId(e.target.value)}
                className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
              >
                {experiments.map((experiment) => (
                  <option key={experiment.id} value={experiment.id}>
                    Baseline: {experiment.name}
                  </option>
                ))}
              </select>
              <select
                value={candidateId}
                onChange={(e) => setCandidateId(e.target.value)}
                className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
              >
                {experiments.map((experiment) => (
                  <option key={experiment.id} value={experiment.id}>
                    Candidate: {experiment.name}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex justify-end">
              <Button onClick={openComparison}>Compare</Button>
            </div>
          </div>
        ) : null}
      </section>

      {selectedExperimentResults ? (
        <section className="rounded-lg border border-border bg-card p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-base font-semibold">Experiment Results</h2>
              <p className="text-sm text-muted-foreground">
                Per-case rendered prompts, outputs, and scoring artifacts.
              </p>
            </div>
            <Link
              href={`/workspaces/${workspaceId}/playgrounds/${playground.id}`}
              className="text-sm text-muted-foreground hover:text-foreground"
            >
              Clear
            </Link>
          </div>
          <div className="space-y-4">
            {selectedExperimentResults.map((result) => (
              <div key={result.id} className="rounded-md border border-border p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <span className="font-medium">{result.case_key}</span>
                  <Badge variant={statusVariant(result.status)}>{result.status}</Badge>
                </div>
                <div className="grid gap-4 md:grid-cols-2">
                  <div>
                    <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Rendered Prompt</p>
                    <pre className="overflow-x-auto rounded-md bg-muted/50 p-3 text-xs whitespace-pre-wrap">{result.rendered_prompt}</pre>
                  </div>
                  <div>
                    <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Actual Output</p>
                    <pre className="overflow-x-auto rounded-md bg-muted/50 p-3 text-xs whitespace-pre-wrap">{result.actual_output || result.error_message || "No output"}</pre>
                  </div>
                </div>
                <div className="grid gap-3 md:grid-cols-4 text-sm">
                  <div>
                    <p className="text-muted-foreground">Latency</p>
                    <p>{result.latency_ms} ms</p>
                  </div>
                  <div>
                    <p className="text-muted-foreground">Tokens</p>
                    <p>{result.total_tokens}</p>
                  </div>
                  <div>
                    <p className="text-muted-foreground">Cost</p>
                    <p>{result.cost_usd ?? 0}</p>
                  </div>
                  <div>
                    <p className="text-muted-foreground">Dimensions</p>
                    <pre className="overflow-x-auto text-xs">{prettyJSON(result.dimension_scores)}</pre>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </section>
      ) : null}

      {comparison ? (
        <section className="rounded-lg border border-border bg-card p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-base font-semibold">Experiment Comparison</h2>
              <p className="text-sm text-muted-foreground">
                Aggregated dimension deltas and per-case changes.
              </p>
            </div>
            <Link
              href={`/workspaces/${workspaceId}/playgrounds/${playground.id}`}
              className="text-sm text-muted-foreground hover:text-foreground"
            >
              Clear
            </Link>
          </div>
          <div className="grid gap-3 md:grid-cols-4">
            {Object.entries(comparison.aggregated_dimension_deltas).map(([dimension, delta]) => (
              <div key={dimension} className="rounded-md border border-border p-3">
                <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{dimension}</p>
                <p className="mt-2 text-sm">
                  Baseline {delta.baseline_value ?? "n/a"} / Candidate {delta.candidate_value ?? "n/a"}
                </p>
                <p className="text-sm text-muted-foreground">Delta {delta.delta ?? "n/a"} ({delta.state})</p>
              </div>
            ))}
          </div>
          <div className="space-y-3">
            {comparison.per_case.map((item) => (
              <div key={item.case_key} className="rounded-md border border-border p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <span className="font-medium">{item.case_key}</span>
                  <div className="flex gap-2">
                    <Badge variant={statusVariant(item.baseline_status)}>Baseline {item.baseline_status}</Badge>
                    <Badge variant={statusVariant(item.candidate_status)}>Candidate {item.candidate_status}</Badge>
                  </div>
                </div>
                <div className="grid gap-4 md:grid-cols-2">
                  <pre className="overflow-x-auto rounded-md bg-muted/50 p-3 text-xs whitespace-pre-wrap">{item.baseline_output || item.baseline_error_message || "No baseline output"}</pre>
                  <pre className="overflow-x-auto rounded-md bg-muted/50 p-3 text-xs whitespace-pre-wrap">{item.candidate_output || item.candidate_error_message || "No candidate output"}</pre>
                </div>
                <pre className="overflow-x-auto rounded-md bg-muted/50 p-3 text-xs">{prettyJSON(item.dimension_deltas)}</pre>
              </div>
            ))}
          </div>
        </section>
      ) : null}
    </div>
  );
}
