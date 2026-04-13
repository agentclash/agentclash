"use client";

import { useCallback, useState } from "react";
import { useRouter } from "next/navigation";
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
import { Badge } from "@/components/ui/badge";
import { PageHeader } from "@/components/ui/page-header";
import { ConfirmProvider, useConfirm } from "@/components/ui/confirm-dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useExperimentPolling } from "@/hooks/use-experiment-polling";
import { PromptEditor } from "./components/prompt-editor";
import { TestCasePanel } from "./components/test-case-panel";
import { ExperimentLauncher } from "./components/experiment-launcher";
import { ExperimentList } from "./components/experiment-list";
import { ComparisonPanel } from "./components/comparison-panel";
import { EvalSpecBuilder } from "./components/eval-spec-builder";
import { Trash2 } from "lucide-react";

export function PlaygroundDetailClient(props: {
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
  return (
    <ConfirmProvider>
      <PlaygroundDetailInner
        workspaceId={props.workspaceId}
        playground={props.playground}
        testCases={props.testCases}
        initialExperiments={props.experiments}
        providerAccounts={props.providerAccounts}
        modelAliases={props.modelAliases}
        comparison={props.comparison}
        baselineExperimentId={props.baselineExperimentId}
        candidateExperimentId={props.candidateExperimentId}
      />
    </ConfirmProvider>
  );
}

function PlaygroundDetailInner({
  workspaceId,
  playground,
  testCases,
  initialExperiments,
  providerAccounts,
  modelAliases,
  comparison,
  baselineExperimentId,
  candidateExperimentId,
}: {
  workspaceId: string;
  playground: Playground;
  testCases: PlaygroundTestCase[];
  initialExperiments: PlaygroundExperiment[];
  providerAccounts: ProviderAccount[];
  modelAliases: ModelAlias[];
  comparison: PlaygroundExperimentComparison | null;
  baselineExperimentId: string | null;
  candidateExperimentId: string | null;
}) {
  const router = useRouter();
  const confirm = useConfirm();
  const { getAccessToken } = useAccessToken();
  const [activeTab, setActiveTab] = useState("editor");
  const [error, setError] = useState<string | null>(null);
  const [evalSpec, setEvalSpec] = useState<unknown>(playground.evaluation_spec);

  const { experiments, resultsByExperimentId, isPolling, fetchResultsForExperiment } =
    useExperimentPolling({
      playgroundId: playground.id,
      initialExperiments,
      enabled: activeTab === "experiments" || activeTab === "compare",
    });

  const completedCount = experiments.filter(
    (e) => e.status === "completed",
  ).length;

  const api = useCallback(
    async () => {
      const token = await getAccessToken();
      return createApiClient(token);
    },
    [getAccessToken],
  );

  async function handleSavePlayground(data: {
    name: string;
    promptTemplate: string;
    systemPrompt: string;
    evaluationSpec: unknown;
  }) {
    setError(null);
    try {
      const client = await api();
      await client.patch(`/v1/playgrounds/${playground.id}`, {
        name: data.name,
        prompt_template: data.promptTemplate,
        system_prompt: data.systemPrompt,
        evaluation_spec: evalSpec,
      });
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save playground");
      throw err;
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

  async function handleLaunchSingle(data: {
    name: string;
    providerAccountId: string;
    modelAliasId: string;
  }) {
    setError(null);
    try {
      const client = await api();
      await client.post(`/v1/playgrounds/${playground.id}/experiments`, {
        name: data.name,
        provider_account_id: data.providerAccountId,
        model_alias_id: data.modelAliasId,
        request_config: { trace_mode: "required", step_timeout_ms: 120000 },
      });
      router.refresh();
      setActiveTab("experiments");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to launch experiment",
      );
      throw err;
    }
  }

  async function handleLaunchBatch(data: {
    models: {
      providerAccountId: string;
      modelAliasId: string;
      name: string;
    }[];
  }) {
    setError(null);
    try {
      const client = await api();
      await client.post(
        `/v1/playgrounds/${playground.id}/experiments/batch`,
        {
          models: data.models.map((m) => ({
            provider_account_id: m.providerAccountId,
            model_alias_id: m.modelAliasId,
            name: m.name,
          })),
          request_config: { trace_mode: "required", step_timeout_ms: 120000 },
        },
      );
      router.refresh();
      setActiveTab("experiments");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to launch experiments",
      );
      throw err;
    }
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

      <Tabs value={activeTab} onValueChange={(v) => v && setActiveTab(v as string)}>
        <TabsList>
          <TabsTrigger value="editor">Editor</TabsTrigger>
          <TabsTrigger value="test-cases">
            Test Cases
            {testCases.length > 0 && (
              <Badge variant="secondary" className="ml-1.5 text-[10px] px-1.5 py-0">
                {testCases.length}
              </Badge>
            )}
          </TabsTrigger>
          <TabsTrigger value="experiments">
            Experiments
            {experiments.length > 0 && (
              <Badge variant="secondary" className="ml-1.5 text-[10px] px-1.5 py-0">
                {experiments.length}
              </Badge>
            )}
            {isPolling && (
              <span className="ml-1 size-1.5 rounded-full bg-emerald-500 animate-pulse" />
            )}
          </TabsTrigger>
          <TabsTrigger value="compare" disabled={completedCount < 2}>
            Compare
          </TabsTrigger>
        </TabsList>

        <TabsContent value="editor">
          <PromptEditor
            name={playground.name}
            promptTemplate={playground.prompt_template}
            systemPrompt={playground.system_prompt}
            evaluationSpec={evalSpec}
            onSave={handleSavePlayground}
            evalSpecBuilder={
              <EvalSpecBuilder value={evalSpec} onChange={setEvalSpec} />
            }
          />
        </TabsContent>

        <TabsContent value="test-cases">
          <TestCasePanel
            testCases={testCases}
            onCreateTestCase={handleCreateTestCase}
            onUpdateTestCase={handleUpdateTestCase}
            onDeleteTestCase={handleDeleteTestCase}
          />
        </TabsContent>

        <TabsContent value="experiments">
          <div className="space-y-6">
            <ExperimentLauncher
              providerAccounts={providerAccounts}
              modelAliases={modelAliases}
              onLaunchSingle={handleLaunchSingle}
              onLaunchBatch={handleLaunchBatch}
            />
            <ExperimentList
              experiments={experiments}
              resultsByExperimentId={resultsByExperimentId}
              isPolling={isPolling}
              onFetchResults={fetchResultsForExperiment}
            />
          </div>
        </TabsContent>

        <TabsContent value="compare">
          <ComparisonPanel
            workspaceId={workspaceId}
            playgroundId={playground.id}
            experiments={experiments}
            comparison={comparison}
            initialBaselineId={baselineExperimentId}
            initialCandidateId={candidateExperimentId}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
}
