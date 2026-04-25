import { apiQueryKey, type ApiQueryKey } from "@/lib/api/swr";

const RUN_PAGE_SIZE = 20;
const SUITE_PAGE_SIZE = 50;
const MEMBERS_PAGE_SIZE = 50;

export const workspaceResourceKeys = {
  builds: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/agent-builds`),
  deployments: (workspaceId: string): ApiQueryKey =>
    apiQueryKey(`/v1/workspaces/${workspaceId}/agent-deployments`),
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
};

export const workspacePageSizes = {
  runs: RUN_PAGE_SIZE,
  suites: SUITE_PAGE_SIZE,
  members: MEMBERS_PAGE_SIZE,
};
