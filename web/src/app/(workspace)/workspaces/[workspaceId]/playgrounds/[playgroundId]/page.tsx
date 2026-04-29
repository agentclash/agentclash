import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  ModelAlias,
  KnowledgeSource,
  Playground,
  PlaygroundExperiment,
  PlaygroundExperimentComparison,
  PlaygroundTestCase,
  ProviderAccount,
  WorkspaceTool,
} from "@/lib/api/types";
import { PlaygroundDetailClient } from "./playground-detail-client";

export default async function PlaygroundDetailPage({
  params,
  searchParams,
}: {
  params: Promise<{ workspaceId: string; playgroundId: string }>;
  searchParams: Promise<{ baseline?: string; candidate?: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId, playgroundId } = await params;
  const { baseline, candidate } = await searchParams;
  const api = createApiClient(accessToken);

  let playground: Playground;
  let testCases: PlaygroundTestCase[];
  let experiments: PlaygroundExperiment[];
  let providerAccounts: ProviderAccount[];
  let modelAliases: ModelAlias[];
  let tools: WorkspaceTool[];
  let knowledgeSources: KnowledgeSource[];

  try {
    const [
      playgroundRes,
      testCasesRes,
      experimentsRes,
      providerAccountsRes,
      modelAliasesRes,
      toolsRes,
      knowledgeSourcesRes,
    ] =
      await Promise.all([
        api.get<Playground>(`/v1/playgrounds/${playgroundId}`),
        api.get<{ items: PlaygroundTestCase[] }>(`/v1/playgrounds/${playgroundId}/test-cases`),
        api.get<{ items: PlaygroundExperiment[] }>(`/v1/playgrounds/${playgroundId}/experiments`),
        api.get<{ items: ProviderAccount[] }>(`/v1/workspaces/${workspaceId}/provider-accounts`),
        api.get<{ items: ModelAlias[] }>(`/v1/workspaces/${workspaceId}/model-aliases`),
        api.get<{ items: WorkspaceTool[] }>(`/v1/workspaces/${workspaceId}/tools`),
        api.get<{ items: KnowledgeSource[] }>(`/v1/workspaces/${workspaceId}/knowledge-sources`),
      ]);
    playground = playgroundRes;
    testCases = testCasesRes.items;
    experiments = experimentsRes.items;
    providerAccounts = providerAccountsRes.items;
    modelAliases = modelAliasesRes.items;
    tools = toolsRes.items;
    knowledgeSources = knowledgeSourcesRes.items;
  } catch (err) {
    const message = err instanceof ApiError ? err.message : "Failed to load playground";
    return (
      <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-6 text-center text-sm text-destructive">
        {message}
      </div>
    );
  }

  const comparison =
    baseline && candidate
      ? await api
          .get<PlaygroundExperimentComparison>(
            "/v1/playground-experiments/compare",
            { params: { baseline, candidate } },
          )
          .catch(() => null)
      : null;

  return (
    <div>
      <PlaygroundDetailClient
        workspaceId={workspaceId}
        playground={playground}
        testCases={testCases}
        experiments={experiments}
        providerAccounts={providerAccounts}
        modelAliases={modelAliases}
        tools={tools}
        knowledgeSources={knowledgeSources}
        comparison={comparison}
        baselineExperimentId={baseline ?? null}
        candidateExperimentId={candidate ?? null}
      />
    </div>
  );
}
