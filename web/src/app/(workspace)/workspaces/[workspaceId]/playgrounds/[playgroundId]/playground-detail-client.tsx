"use client";

import { useCallback, useMemo, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import type {
  KnowledgeSource,
  ModelAlias,
  Playground,
  PlaygroundExperiment,
  PlaygroundExperimentComparison,
  PlaygroundExperimentResult,
  PlaygroundTestCase,
  ProviderAccount,
  WorkspaceTool,
} from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { PageHeader } from "@/components/ui/page-header";
import { ConfirmProvider, useConfirm } from "@/components/ui/confirm-dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useExperimentPolling } from "@/hooks/use-experiment-polling";
import { TestCasePanel } from "./components/test-case-panel";
import { ComparisonPanel } from "./components/comparison-panel";
import { EvalSpecBuilder } from "./components/eval-spec-builder";
import { KpiStrip } from "./components/kpi-strip";
import {
  ArrowRightLeft,
  Bot,
  Brain,
  Database,
  Loader2,
  MessageSquareText,
  Rocket,
  Settings2,
  SlidersHorizontal,
  Trash2,
  Wrench,
} from "lucide-react";

type TraceMode = "required" | "best_effort" | "disabled";

interface LaneConfig {
  label: string;
  providerAccountId: string;
  modelAliasId: string;
  temperature: string;
  timeoutMs: string;
  traceMode: TraceMode;
  toolIds: string[];
  knowledgeSourceIds: string[];
}

function makeLaneConfig(
  label: string,
  providerAccounts: ProviderAccount[],
  modelAliases: ModelAlias[],
): LaneConfig {
  return {
    label,
    providerAccountId: providerAccounts[0]?.id ?? "",
    modelAliasId: modelAliases[0]?.id ?? "",
    temperature: "0.2",
    timeoutMs: "120000",
    traceMode: "required",
    toolIds: [],
    knowledgeSourceIds: [],
  };
}

function statusVariant(
  status: string,
): "default" | "secondary" | "destructive" | "outline" {
  switch (status) {
    case "completed":
      return "default";
    case "running":
    case "queued":
      return "secondary";
    case "failed":
      return "destructive";
    default:
      return "outline";
  }
}

function formatPercent(value: number | null | undefined): string {
  if (value == null) return "N/A";
  return `${Math.round(value * 100)}%`;
}

function experimentTitle(
  experiment: PlaygroundExperiment | undefined,
  fallback: string,
): string {
  return experiment?.name || fallback;
}

function parseTimeout(value: string): number {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 120000;
}

function parseTemperature(value: string): number {
  const parsed = Number.parseFloat(value);
  if (!Number.isFinite(parsed)) return 0.2;
  return Math.min(2, Math.max(0, parsed));
}

export function PlaygroundDetailClient(props: {
  workspaceId: string;
  playground: Playground;
  testCases: PlaygroundTestCase[];
  experiments: PlaygroundExperiment[];
  providerAccounts: ProviderAccount[];
  modelAliases: ModelAlias[];
  tools: WorkspaceTool[];
  knowledgeSources: KnowledgeSource[];
  comparison: PlaygroundExperimentComparison | null;
  baselineExperimentId: string | null;
  candidateExperimentId: string | null;
}) {
  return (
    <ConfirmProvider>
      <PlaygroundDetailInner {...props} />
    </ConfirmProvider>
  );
}

function PlaygroundDetailInner({
  workspaceId,
  playground,
  testCases,
  experiments: initialExperiments,
  providerAccounts,
  modelAliases,
  tools,
  knowledgeSources,
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
  tools: WorkspaceTool[];
  knowledgeSources: KnowledgeSource[];
  comparison: PlaygroundExperimentComparison | null;
  baselineExperimentId: string | null;
  candidateExperimentId: string | null;
}) {
  const router = useRouter();
  const confirm = useConfirm();
  const { getAccessToken } = useAccessToken();
  const [error, setError] = useState<string | null>(null);
  const [evalSpec, setEvalSpec] = useState<unknown>(playground.evaluation_spec);
  const [name, setName] = useState(playground.name);
  const [promptTemplate, setPromptTemplate] = useState(playground.prompt_template);
  const [systemPrompt, setSystemPrompt] = useState(playground.system_prompt);
  const [saving, setSaving] = useState(false);
  const [launching, setLaunching] = useState(false);
  const [baselineLane, setBaselineLane] = useState(() =>
    makeLaneConfig("Baseline", providerAccounts, modelAliases),
  );
  const [candidateLane, setCandidateLane] = useState(() =>
    makeLaneConfig("Candidate", providerAccounts, modelAliases),
  );

  const { experiments, resultsByExperimentId, isPolling, fetchResultsForExperiment } =
    useExperimentPolling({
      playgroundId: playground.id,
      initialExperiments,
      enabled: true,
    });

  const completedExperiments = useMemo(
    () => experiments.filter((experiment) => experiment.status === "completed"),
    [experiments],
  );
  const newestCompleted = completedExperiments[0];
  const secondNewestCompleted = completedExperiments[1];
  const [baselineSelection, setBaselineSelection] = useState(
    baselineExperimentId ?? newestCompleted?.id ?? "",
  );
  const [candidateSelection, setCandidateSelection] = useState(
    candidateExperimentId ?? secondNewestCompleted?.id ?? "",
  );

  const baselineExperiment = experiments.find((e) => e.id === baselineSelection);
  const candidateExperiment = experiments.find((e) => e.id === candidateSelection);
  const baselineResults = baselineSelection
    ? (resultsByExperimentId[baselineSelection] ?? [])
    : [];
  const candidateResults = candidateSelection
    ? (resultsByExperimentId[candidateSelection] ?? [])
    : [];
  const completedCount = completedExperiments.length;

  const api = useCallback(async () => {
    const token = await getAccessToken();
    return createApiClient(token);
  }, [getAccessToken]);

  async function handleSavePlayground() {
    setError(null);
    setSaving(true);
    try {
      const client = await api();
      await client.patch(`/v1/playgrounds/${playground.id}`, {
        name,
        prompt_template: promptTemplate,
        system_prompt: systemPrompt,
        evaluation_spec: evalSpec,
      });
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save playground");
      throw err;
    } finally {
      setSaving(false);
    }
  }

  async function handleDeletePlayground() {
    const ok = await confirm({
      title: "Delete playground?",
      description:
        "This will permanently delete this playground and all experiments.",
      confirmLabel: "Delete",
      variant: "danger",
    });
    if (!ok) return;
    try {
      const client = await api();
      await client.del(`/v1/playgrounds/${playground.id}`);
      router.push(`/workspaces/${workspaceId}/playgrounds`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete");
    }
  }

  async function handleCreateTestCase(data: {
    caseKey: string;
    variables: string;
    expectations: string;
  }) {
    setError(null);
    try {
      const client = await api();
      await client.post(`/v1/playgrounds/${playground.id}/test-cases`, {
        case_key: data.caseKey,
        variables: JSON.parse(data.variables),
        expectations: JSON.parse(data.expectations),
      });
      router.refresh();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to create test case",
      );
      throw err;
    }
  }

  async function handleUpdateTestCase(
    id: string,
    data: { caseKey: string; variables: string; expectations: string },
  ) {
    setError(null);
    try {
      const client = await api();
      await client.patch(`/v1/playground-test-cases/${id}`, {
        case_key: data.caseKey,
        variables: JSON.parse(data.variables),
        expectations: JSON.parse(data.expectations),
      });
      router.refresh();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to update test case",
      );
      throw err;
    }
  }

  async function handleDeleteTestCase(id: string) {
    setError(null);
    try {
      const client = await api();
      await client.del(`/v1/playground-test-cases/${id}`);
      router.refresh();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to delete test case",
      );
    }
  }

  function laneRequestConfig(lane: LaneConfig) {
    return {
      trace_mode: lane.traceMode,
      step_timeout_ms: parseTimeout(lane.timeoutMs),
      temperature: parseTemperature(lane.temperature),
      tools: lane.toolIds.map((toolId) => ({ tool_id: toolId })),
      knowledge_sources: lane.knowledgeSourceIds.map((knowledgeSourceId) => ({
        knowledge_source_id: knowledgeSourceId,
      })),
    };
  }

  async function handleLaunchComparison() {
    setError(null);
    setLaunching(true);
    try {
      const client = await api();
      await client.post(`/v1/playgrounds/${playground.id}/experiments/batch`, {
        models: [
          {
            provider_account_id: baselineLane.providerAccountId,
            model_alias_id: baselineLane.modelAliasId,
            name: baselineLane.label,
            request_config: laneRequestConfig(baselineLane),
          },
          {
            provider_account_id: candidateLane.providerAccountId,
            model_alias_id: candidateLane.modelAliasId,
            name: candidateLane.label,
            request_config: laneRequestConfig(candidateLane),
          },
        ],
        request_config: {
          baseline: laneRequestConfig(baselineLane),
          candidate: laneRequestConfig(candidateLane),
        },
      });
      router.refresh();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to launch comparison",
      );
      throw err;
    } finally {
      setLaunching(false);
    }
  }

  function handleCompareSelection() {
    if (!baselineSelection || !candidateSelection) return;
    const params = new URLSearchParams();
    params.set("baseline", baselineSelection);
    params.set("candidate", candidateSelection);
    router.push(
      `/workspaces/${workspaceId}/playgrounds/${playground.id}?${params.toString()}`,
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={playground.name}
        breadcrumbs={[
          {
            label: "Playgrounds",
            href: `/workspaces/${workspaceId}/playgrounds`,
          },
          { label: playground.name },
        ]}
        actions={
          <Button
            variant="destructive"
            size="sm"
            onClick={handleDeletePlayground}
          >
            <Trash2 className="mr-2 size-3.5" />
            Delete
          </Button>
        }
      />

      {error && (
        <div className="rounded-md border border-destructive/20 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      <section className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_360px]">
        <div className="space-y-5">
          <div className="rounded-lg border border-border bg-card">
            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-4 py-3">
              <div className="flex items-center gap-2">
                <ArrowRightLeft className="size-4 text-primary" />
                <div>
                  <h2 className="text-sm font-semibold">Side-by-side chat comparison</h2>
                  <p className="text-xs text-muted-foreground">
                    Run two configured lanes against the same prompt, cases, and scorecard.
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-2">
                {isPolling && (
                  <Badge variant="secondary" className="gap-1">
                    <span className="size-1.5 rounded-full bg-emerald-500" />
                    polling
                  </Badge>
                )}
                <Button
                  onClick={handleLaunchComparison}
                  disabled={
                    launching ||
                    testCases.length === 0 ||
                    !baselineLane.providerAccountId ||
                    !baselineLane.modelAliasId ||
                    !candidateLane.providerAccountId ||
                    !candidateLane.modelAliasId
                  }
                >
                  {launching ? (
                    <Loader2 className="mr-2 size-4 animate-spin" />
                  ) : (
                    <Rocket className="mr-2 size-4" />
                  )}
                  {launching ? "Running..." : "Run comparison"}
                </Button>
              </div>
            </div>

            <div className="grid gap-0 lg:grid-cols-2">
              <ComparisonLane
                title="Baseline"
                accent="border-l-blue-500"
                lane={baselineLane}
                onChange={setBaselineLane}
                providerAccounts={providerAccounts}
                modelAliases={modelAliases}
                tools={tools}
                knowledgeSources={knowledgeSources}
                selectedExperimentId={baselineSelection}
                onSelectExperiment={(id) => {
                  setBaselineSelection(id);
                  void fetchResultsForExperiment(id);
                }}
                experiments={completedExperiments}
                experiment={baselineExperiment}
                results={baselineResults}
              />
              <ComparisonLane
                title="Candidate"
                accent="border-l-emerald-500"
                lane={candidateLane}
                onChange={setCandidateLane}
                providerAccounts={providerAccounts}
                modelAliases={modelAliases}
                tools={tools}
                knowledgeSources={knowledgeSources}
                selectedExperimentId={candidateSelection}
                onSelectExperiment={(id) => {
                  setCandidateSelection(id);
                  void fetchResultsForExperiment(id);
                }}
                experiments={completedExperiments}
                experiment={candidateExperiment}
                results={candidateResults}
              />
            </div>
          </div>

          {completedCount >= 2 && (
            <div className="rounded-lg border border-border bg-card p-4">
              <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h2 className="text-sm font-semibold">Scorecard comparison</h2>
                  <p className="text-xs text-muted-foreground">
                    Compare saved runs by case, output, and score deltas.
                  </p>
                </div>
                <Button
                  variant="secondary"
                  onClick={handleCompareSelection}
                  disabled={
                    !baselineSelection ||
                    !candidateSelection ||
                    baselineSelection === candidateSelection
                  }
                >
                  <ArrowRightLeft className="mr-2 size-4" />
                  Compare selected
                </Button>
              </div>
              <ComparisonPanel
                workspaceId={workspaceId}
                playgroundId={playground.id}
                experiments={experiments}
                comparison={comparison}
                initialBaselineId={baselineSelection}
                initialCandidateId={candidateSelection}
              />
            </div>
          )}
        </div>

        <aside className="space-y-4">
          <SharedPromptPanel
            name={name}
            onNameChange={setName}
            systemPrompt={systemPrompt}
            onSystemPromptChange={setSystemPrompt}
            promptTemplate={promptTemplate}
            onPromptTemplateChange={setPromptTemplate}
            saving={saving}
            onSave={handleSavePlayground}
          />

          <div className="rounded-lg border border-border bg-card p-4">
            <div className="mb-3 flex items-center gap-2">
              <Brain className="size-4 text-primary" />
              <h2 className="text-sm font-semibold">Evaluation config</h2>
            </div>
            <EvalSpecBuilder value={evalSpec} onChange={setEvalSpec} />
          </div>

          <div className="rounded-lg border border-border bg-card p-4">
            <div className="mb-3 flex items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <MessageSquareText className="size-4 text-primary" />
                <h2 className="text-sm font-semibold">Test cases</h2>
              </div>
              <Badge variant="secondary">{testCases.length}</Badge>
            </div>
            <TestCasePanel
              testCases={testCases}
              onCreateTestCase={handleCreateTestCase}
              onUpdateTestCase={handleUpdateTestCase}
              onDeleteTestCase={handleDeleteTestCase}
            />
          </div>
        </aside>
      </section>
    </div>
  );
}

function SharedPromptPanel({
  name,
  onNameChange,
  systemPrompt,
  onSystemPromptChange,
  promptTemplate,
  onPromptTemplateChange,
  saving,
  onSave,
}: {
  name: string;
  onNameChange: (value: string) => void;
  systemPrompt: string;
  onSystemPromptChange: (value: string) => void;
  promptTemplate: string;
  onPromptTemplateChange: (value: string) => void;
  saving: boolean;
  onSave: () => Promise<void>;
}) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="mb-3 flex items-center gap-2">
        <Settings2 className="size-4 text-primary" />
        <h2 className="text-sm font-semibold">Shared prompt</h2>
      </div>
      <div className="space-y-3">
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-muted-foreground">Name</label>
          <Input value={name} onChange={(e) => onNameChange(e.target.value)} />
        </div>
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-muted-foreground">
            System prompt
          </label>
          <textarea
            value={systemPrompt}
            onChange={(e) => onSystemPromptChange(e.target.value)}
            spellCheck={false}
            className="min-h-20 w-full resize-y rounded-lg border border-input bg-transparent px-3 py-2 text-sm leading-relaxed focus:outline-none focus:ring-2 focus:ring-ring/50"
          />
        </div>
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-muted-foreground">
            Prompt template
          </label>
          <textarea
            value={promptTemplate}
            onChange={(e) => onPromptTemplateChange(e.target.value)}
            spellCheck={false}
            className="min-h-36 w-full resize-y rounded-lg border border-input bg-transparent px-3 py-2 text-sm leading-relaxed focus:outline-none focus:ring-2 focus:ring-ring/50"
          />
        </div>
        <Button onClick={() => void onSave()} disabled={saving} className="w-full">
          {saving && <Loader2 className="mr-2 size-4 animate-spin" />}
          {saving ? "Saving..." : "Save prompt and evaluation"}
        </Button>
      </div>
    </div>
  );
}

function ComparisonLane({
  title,
  accent,
  lane,
  onChange,
  providerAccounts,
  modelAliases,
  tools,
  knowledgeSources,
  selectedExperimentId,
  onSelectExperiment,
  experiments,
  experiment,
  results,
}: {
  title: string;
  accent: string;
  lane: LaneConfig;
  onChange: (lane: LaneConfig) => void;
  providerAccounts: ProviderAccount[];
  modelAliases: ModelAlias[];
  tools: WorkspaceTool[];
  knowledgeSources: KnowledgeSource[];
  selectedExperimentId: string;
  onSelectExperiment: (experimentId: string) => void;
  experiments: PlaygroundExperiment[];
  experiment: PlaygroundExperiment | undefined;
  results: PlaygroundExperimentResult[];
}) {
  function patch(update: Partial<LaneConfig>) {
    onChange({ ...lane, ...update });
  }

  return (
    <div className={`border-l-4 ${accent} border-t border-border p-4 first:border-t-0 lg:border-t-0 lg:first:border-r`}>
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Bot className="size-4 text-primary" />
          <h3 className="text-sm font-semibold">{title}</h3>
        </div>
        {experiment && (
          <Badge variant={statusVariant(experiment.status)}>{experiment.status}</Badge>
        )}
      </div>

      <div className="grid gap-3 md:grid-cols-2">
        <div className="space-y-1.5 md:col-span-2">
          <label className="text-xs font-medium text-muted-foreground">Lane label</label>
          <Input
            value={lane.label}
            onChange={(e) => patch({ label: e.target.value })}
            placeholder={title}
          />
        </div>
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-muted-foreground">Provider</label>
          <Select
            value={lane.providerAccountId}
            onValueChange={(value) => value && patch({ providerAccountId: value })}
          >
            <SelectTrigger className="w-full">
              <SelectValue placeholder="Select provider" />
            </SelectTrigger>
            <SelectContent>
              {providerAccounts.map((account) => (
                <SelectItem key={account.id} value={account.id}>
                  {account.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-muted-foreground">Model</label>
          <Select
            value={lane.modelAliasId}
            onValueChange={(value) => value && patch({ modelAliasId: value })}
          >
            <SelectTrigger className="w-full">
              <SelectValue placeholder="Select model" />
            </SelectTrigger>
            <SelectContent>
              {modelAliases.map((alias) => (
                <SelectItem key={alias.id} value={alias.id}>
                  {alias.display_name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="mt-4 rounded-md border border-border bg-muted/20 p-3">
        <div className="mb-3 flex items-center gap-2">
          <SlidersHorizontal className="size-3.5 text-muted-foreground" />
          <span className="text-xs font-semibold uppercase text-muted-foreground">
            Config
          </span>
        </div>
        <div className="grid gap-3 md:grid-cols-3">
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-muted-foreground">
              Temperature
            </label>
            <Input
              type="number"
              min="0"
              max="2"
              step="0.1"
              value={lane.temperature}
              onChange={(e) => patch({ temperature: e.target.value })}
            />
          </div>
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-muted-foreground">
              Timeout ms
            </label>
            <Input
              type="number"
              min="1000"
              step="1000"
              value={lane.timeoutMs}
              onChange={(e) => patch({ timeoutMs: e.target.value })}
            />
          </div>
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-muted-foreground">
              Trace mode
            </label>
            <Select
              value={lane.traceMode}
              onValueChange={(value) => patch({ traceMode: value as TraceMode })}
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="required">required</SelectItem>
                <SelectItem value="best_effort">best_effort</SelectItem>
                <SelectItem value="disabled">disabled</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </div>

      <ResourceChecklist
        icon={<Wrench className="size-3.5" />}
        title="Tools"
        emptyLabel="No tools registered"
        items={tools.map((tool) => ({
          id: tool.id,
          label: tool.name,
          meta: tool.capability_key,
        }))}
        selectedIds={lane.toolIds}
        onChange={(toolIds) => patch({ toolIds })}
      />

      <ResourceChecklist
        icon={<Database className="size-3.5" />}
        title="Knowledge"
        emptyLabel="No knowledge sources connected"
        items={knowledgeSources.map((source) => ({
          id: source.id,
          label: source.name,
          meta: source.source_kind,
        }))}
        selectedIds={lane.knowledgeSourceIds}
        onChange={(knowledgeSourceIds) => patch({ knowledgeSourceIds })}
      />

      <div className="mt-4 space-y-3 border-t border-border pt-4">
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-muted-foreground">
            Compare saved run
          </label>
          <Select
            value={selectedExperimentId}
            onValueChange={(value) => value && onSelectExperiment(value)}
          >
            <SelectTrigger className="w-full">
              <SelectValue placeholder="Select completed run" />
            </SelectTrigger>
            <SelectContent>
              {experiments.map((item) => (
                <SelectItem key={item.id} value={item.id}>
                  {item.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <LaneResults
          title={experimentTitle(experiment, title)}
          experiment={experiment}
          results={results}
        />
      </div>
    </div>
  );
}

function ResourceChecklist({
  icon,
  title,
  emptyLabel,
  items,
  selectedIds,
  onChange,
}: {
  icon: ReactNode;
  title: string;
  emptyLabel: string;
  items: { id: string; label: string; meta: string }[];
  selectedIds: string[];
  onChange: (ids: string[]) => void;
}) {
  function toggle(id: string) {
    onChange(
      selectedIds.includes(id)
        ? selectedIds.filter((selectedId) => selectedId !== id)
        : [...selectedIds, id],
    );
  }

  return (
    <div className="mt-4 rounded-md border border-border bg-muted/20 p-3">
      <div className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase text-muted-foreground">
        {icon}
        {title}
      </div>
      {items.length === 0 ? (
        <p className="text-xs text-muted-foreground">{emptyLabel}</p>
      ) : (
        <div className="grid gap-2">
          {items.map((item) => (
            <label
              key={item.id}
              className="flex min-h-9 items-center gap-2 rounded-md border border-border bg-background px-2 py-1.5 text-xs"
            >
              <input
                type="checkbox"
                checked={selectedIds.includes(item.id)}
                onChange={() => toggle(item.id)}
                className="size-3.5 accent-primary"
              />
              <span className="min-w-0 flex-1 truncate font-medium">{item.label}</span>
              <code className="truncate text-[11px] text-muted-foreground">
                {item.meta}
              </code>
            </label>
          ))}
        </div>
      )}
    </div>
  );
}

function LaneResults({
  title,
  experiment,
  results,
}: {
  title: string;
  experiment: PlaygroundExperiment | undefined;
  results: PlaygroundExperimentResult[];
}) {
  if (!experiment) {
    return (
      <div className="rounded-md border border-dashed border-border p-4 text-center text-sm text-muted-foreground">
        Select or run an experiment to see chat output here.
      </div>
    );
  }

  if (results.length === 0) {
    return (
      <div className="rounded-md border border-border p-4 text-center text-sm text-muted-foreground">
        Results are not available yet.
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <div>
        <h4 className="text-sm font-medium">{title}</h4>
        <p className="text-xs text-muted-foreground">
          {results.length} case{results.length === 1 ? "" : "s"}
        </p>
      </div>
      {results.map((result) => (
        <div key={result.id} className="rounded-md border border-border bg-background p-3">
          <div className="mb-3 flex items-center justify-between gap-3">
            <span className="truncate text-sm font-medium">{result.case_key}</span>
            <Badge variant={statusVariant(result.status)}>{result.status}</Badge>
          </div>
          <KpiStrip
            latencyMs={result.latency_ms}
            totalTokens={result.total_tokens}
            costUsd={result.cost_usd}
            dimensions={result.dimension_scores}
          />
          <div className="mt-3 space-y-3">
            <div>
              <p className="mb-1 text-xs font-medium uppercase text-muted-foreground">
                User
              </p>
              <pre className="max-h-32 overflow-auto whitespace-pre-wrap rounded-md bg-muted/50 p-3 text-xs leading-relaxed">
                {result.rendered_prompt}
              </pre>
            </div>
            <div>
              <p className="mb-1 text-xs font-medium uppercase text-muted-foreground">
                Assistant
              </p>
              <pre className="max-h-40 overflow-auto whitespace-pre-wrap rounded-md bg-muted/50 p-3 text-xs leading-relaxed">
                {result.actual_output || result.error_message || "No output"}
              </pre>
            </div>
            {Object.keys(result.dimension_scores ?? {}).length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {Object.entries(result.dimension_scores).map(([dimension, score]) => (
                  <Badge key={dimension} variant="outline">
                    {dimension}: {formatPercent(score)}
                  </Badge>
                ))}
              </div>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
