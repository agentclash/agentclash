import { apiQueryKey, type ApiQueryKey } from "@/lib/api/swr";

const RUN_PAGE_SIZE = 20;
const SUITE_PAGE_SIZE = 50;
const REGRESSION_CASE_PAGE_SIZE = 20;
const MEMBERS_PAGE_SIZE = 50;

export const workspaceResourceKeys = {
  builds: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/agent-builds`),
  deployments: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/agent-deployments`),
  agentHarnesses: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/agent-harnesses`),
  agentHarnessExecutions: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/agent-harness-executions`),
  challengePacks: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/challenge-packs`),
  runs: (workspaceId: string, offset = 0): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/runs`, {
      limit: RUN_PAGE_SIZE,
      offset,
    }),
  evalSessions: (workspaceId: string, offset = 0): ApiQueryKey =>
    apiQueryKey("/v1/eval-sessions", {
      workspace_id: workspaceId,
      limit: RUN_PAGE_SIZE,
      offset,
    }),
  playgrounds: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/playgrounds`),
  regressionSuites: (workspaceId: string, offset = 0): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/regression-suites`, {
      limit: SUITE_PAGE_SIZE,
      offset,
    }),
  regressionCases: (
    workspaceId: string,
    status?: string,
    offset = 0,
  ): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/regression-cases`, {
      status,
      limit: REGRESSION_CASE_PAGE_SIZE,
      offset,
    }),
  runtimeProfiles: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/runtime-profiles`),
  providerAccounts: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/provider-accounts`),
  modelAliases: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/model-aliases`),
  tools: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/tools`),
  knowledgeSources: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/knowledge-sources`),
  artifacts: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/artifacts`),
  datasets: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/datasets`),
  agentTryouts: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/agent-tryouts`),
  datasetExamples: (workspaceId: string, datasetId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/datasets/${datasetId}/examples`),
  datasetVersions: (workspaceId: string, datasetId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/datasets/${datasetId}/versions`),
  datasetBaselines: (workspaceId: string, datasetId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/datasets/${datasetId}/baselines`),
  datasetTraceCandidates: (workspaceId: string, datasetId: string): ApiQueryKey =>
    apiQueryKey(
      `/v1/workspaces/${workspaceId}/datasets/${datasetId}/trace-candidates`,
    ),
  datasetResults: (workspaceId: string, datasetId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/datasets/${datasetId}/results`),
  secrets: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/secrets`),
  workspaceMembers: (workspaceId: string, offset = 0): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/memberships`, {
      limit: MEMBERS_PAGE_SIZE,
      offset,
    }),
};

export const workspaceMutationKeys = {
  createDeploymentDialog(workspaceId: string): ApiQueryKey[] {
    return [
      workspaceResourceKeys.builds(workspaceId),
      workspaceResourceKeys.runtimeProfiles(workspaceId),
      workspaceResourceKeys.providerAccounts(workspaceId),
      workspaceResourceKeys.modelAliases(workspaceId),
    ];
  },
  createRunDialog(workspaceId: string): ApiQueryKey[] {
    return [
      workspaceResourceKeys.challengePacks(workspaceId),
      workspaceResourceKeys.deployments(workspaceId),
      workspaceResourceKeys.regressionSuites(workspaceId, 0),
    ];
  },
  createEvalSessionDialog(workspaceId: string): ApiQueryKey[] {
    return [
      workspaceResourceKeys.challengePacks(workspaceId),
      workspaceResourceKeys.deployments(workspaceId),
    ];
  },
  datasetDetail(workspaceId: string, datasetId: string): ApiQueryKey[] {
    return [
      workspaceResourceKeys.datasets(workspaceId),
      workspaceResourceKeys.datasetExamples(workspaceId, datasetId),
      workspaceResourceKeys.datasetVersions(workspaceId, datasetId),
      workspaceResourceKeys.datasetBaselines(workspaceId, datasetId),
      workspaceResourceKeys.datasetTraceCandidates(workspaceId, datasetId),
      workspaceResourceKeys.datasetResults(workspaceId, datasetId),
    ];
  },
};

export const workspacePageSizes = {
  runs: RUN_PAGE_SIZE,
  suites: SUITE_PAGE_SIZE,
  regressionCases: REGRESSION_CASE_PAGE_SIZE,
  members: MEMBERS_PAGE_SIZE,
};
