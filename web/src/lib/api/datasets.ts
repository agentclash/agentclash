import type { ApiClient, PaginatedResponse } from "./client";
import type { Dataset, DatasetExample, DatasetVersion } from "./types";

export function listDatasets(
  api: ApiClient,
  workspaceId: string,
): Promise<PaginatedResponse<Dataset>> {
  return api.paginated<Dataset>(`/v1/workspaces/${workspaceId}/datasets`);
}

export function getDataset(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
): Promise<Dataset> {
  return api.get<Dataset>(`/v1/workspaces/${workspaceId}/datasets/${datasetId}`);
}

export function listDatasetExamples(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
): Promise<PaginatedResponse<DatasetExample>> {
  return api.paginated<DatasetExample>(
    `/v1/workspaces/${workspaceId}/datasets/${datasetId}/examples`,
  );
}

export function listDatasetVersions(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
): Promise<{ items: DatasetVersion[] }> {
  return api.get<{ items: DatasetVersion[] }>(
    `/v1/workspaces/${workspaceId}/datasets/${datasetId}/versions`,
  );
}
