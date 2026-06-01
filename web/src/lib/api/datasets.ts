import { ApiError, NetworkError } from "./errors";
import type { ApiClient, PaginatedResponse } from "./client";
import type {
  CreateDatasetBaselineInput,
  CreateDatasetInput,
  CreateDatasetVersionInput,
  Dataset,
  DatasetBaseline,
  DatasetExample,
  DatasetGenerationJob,
  DatasetImportMode,
  DatasetImportResponse,
  DatasetInteropFormat,
  DatasetRegressionSuiteLink,
  DatasetVersion,
  EvaluateDatasetGateInput,
  EvaluateDatasetGateResponse,
  ImportDatasetTracesInput,
  ImportDatasetTracesResponse,
  ListDatasetResultsResponse,
  ListDatasetTraceCandidatesResponse,
  PatchDatasetExampleInput,
  PatchDatasetInput,
  PromoteDatasetTraceCandidateInput,
  PromoteDatasetTraceCandidateResponse,
  StartDatasetEvalInput,
  StartDatasetEvalResponse,
  StartDatasetGenerationInput,
  SyncDatasetRegressionSuiteInput,
  SyncDatasetRegressionSuiteResult,
  UpsertDatasetExampleInput,
  ApiErrorResponse,
} from "./types";

function resolveBaseUrl(): string {
  const url =
    typeof window === "undefined"
      ? process.env.API_URL ?? process.env.NEXT_PUBLIC_API_URL
      : process.env.NEXT_PUBLIC_API_URL;
  if (!url) {
    throw new Error(
      "Missing API_URL or NEXT_PUBLIC_API_URL environment variable",
    );
  }
  return url.replace(/\/+$/, "");
}

function datasetBase(workspaceId: string, datasetId: string): string {
  return `/v1/workspaces/${workspaceId}/datasets/${datasetId}`;
}

export function listDatasets(
  api: ApiClient,
  workspaceId: string,
): Promise<PaginatedResponse<Dataset>> {
  return api.paginated<Dataset>(`/v1/workspaces/${workspaceId}/datasets`);
}

export function createDataset(
  api: ApiClient,
  workspaceId: string,
  body: CreateDatasetInput,
): Promise<Dataset> {
  return api.post<Dataset>(`/v1/workspaces/${workspaceId}/datasets`, body);
}

export function getDataset(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
): Promise<Dataset> {
  return api.get<Dataset>(datasetBase(workspaceId, datasetId));
}

export function patchDataset(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
  body: PatchDatasetInput,
): Promise<Dataset> {
  return api.patch<Dataset>(datasetBase(workspaceId, datasetId), body);
}

export function deleteDataset(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
): Promise<void> {
  return api.del<void>(datasetBase(workspaceId, datasetId));
}

export function listDatasetExamples(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
): Promise<PaginatedResponse<DatasetExample>> {
  return api.paginated<DatasetExample>(
    `${datasetBase(workspaceId, datasetId)}/examples`,
  );
}

export function addDatasetExample(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
  body: UpsertDatasetExampleInput,
): Promise<DatasetExample> {
  return api.post<DatasetExample>(
    `${datasetBase(workspaceId, datasetId)}/examples`,
    body,
  );
}

export function patchDatasetExample(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
  exampleId: string,
  body: PatchDatasetExampleInput,
): Promise<DatasetExample> {
  return api.patch<DatasetExample>(
    `${datasetBase(workspaceId, datasetId)}/examples/${exampleId}`,
    body,
  );
}

export function deleteDatasetExample(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
  exampleId: string,
): Promise<DatasetExample> {
  return api.del<DatasetExample>(
    `${datasetBase(workspaceId, datasetId)}/examples/${exampleId}`,
  );
}

export function listDatasetVersions(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
): Promise<{ items: DatasetVersion[] }> {
  return api.get<{ items: DatasetVersion[] }>(
    `${datasetBase(workspaceId, datasetId)}/versions`,
  );
}

export function createDatasetVersion(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
  body: CreateDatasetVersionInput = {},
): Promise<DatasetVersion> {
  return api.post<DatasetVersion>(
    `${datasetBase(workspaceId, datasetId)}/versions`,
    body,
  );
}

export interface ImportDatasetParams {
  token: string;
  workspaceId: string;
  datasetId: string;
  file: File;
  format: DatasetInteropFormat;
  mode?: DatasetImportMode;
  dryRun?: boolean;
  mapping?: Record<string, unknown>;
}

export async function importDataset(
  params: ImportDatasetParams,
): Promise<DatasetImportResponse> {
  const { token, workspaceId, datasetId, file, format, mode, dryRun, mapping } =
    params;

  const form = new FormData();
  form.append("file", file);
  if (mapping) {
    form.append("mapping", JSON.stringify(mapping));
  }

  const url = new URL(
    `${resolveBaseUrl()}${datasetBase(workspaceId, datasetId)}/import`,
  );
  url.searchParams.set("format", format);
  if (mode) url.searchParams.set("mode", mode);
  if (dryRun) url.searchParams.set("dry_run", "true");

  let res: Response;
  try {
    res = await fetch(url.toString(), {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: form,
    });
  } catch (err) {
    throw new NetworkError(
      err instanceof Error ? err.message : "Import request failed",
    );
  }

  if (!res.ok) {
    try {
      const body = (await res.json()) as ApiErrorResponse;
      throw new ApiError(
        res.status,
        body.error.code,
        body.error.message,
      );
    } catch (err) {
      if (err instanceof ApiError) throw err;
      throw new ApiError(res.status, "unknown", res.statusText || "Import failed");
    }
  }

  return (await res.json()) as DatasetImportResponse;
}

export interface ExportDatasetParams {
  token: string;
  workspaceId: string;
  datasetId: string;
  format: DatasetInteropFormat;
  versionId?: string;
}

export async function exportDatasetBlob(
  params: ExportDatasetParams,
): Promise<{ blob: Blob; filename: string }> {
  const { token, workspaceId, datasetId, format, versionId } = params;
  const url = new URL(
    `${resolveBaseUrl()}${datasetBase(workspaceId, datasetId)}/export`,
  );
  url.searchParams.set("format", format);
  if (versionId) url.searchParams.set("version_id", versionId);

  let res: Response;
  try {
    res = await fetch(url.toString(), {
      headers: { Authorization: `Bearer ${token}` },
    });
  } catch (err) {
    throw new NetworkError(
      err instanceof Error ? err.message : "Export request failed",
    );
  }

  if (!res.ok) {
    try {
      const body = (await res.json()) as ApiErrorResponse;
      throw new ApiError(
        res.status,
        body.error.code,
        body.error.message,
      );
    } catch (err) {
      if (err instanceof ApiError) throw err;
      throw new ApiError(res.status, "unknown", res.statusText || "Export failed");
    }
  }

  const blob = await res.blob();
  const ext =
    format === "csv" ? "csv" : format === "openai" ? "jsonl" : format;
  const filename = `dataset-${datasetId.slice(0, 8)}.${ext}`;
  return { blob, filename };
}

export function startDatasetEval(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
  body: StartDatasetEvalInput,
): Promise<StartDatasetEvalResponse> {
  return api.post<StartDatasetEvalResponse>(
    `${datasetBase(workspaceId, datasetId)}/evals`,
    body,
  );
}

export function listDatasetResults(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
  opts?: { versionId?: string; limit?: number; offset?: number },
): Promise<ListDatasetResultsResponse> {
  return api.get<ListDatasetResultsResponse>(
    `${datasetBase(workspaceId, datasetId)}/results`,
    {
      params: {
        version_id: opts?.versionId,
        limit: opts?.limit ?? 50,
        offset: opts?.offset ?? 0,
      },
    },
  );
}

export function importDatasetTraces(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
  body: ImportDatasetTracesInput,
): Promise<ImportDatasetTracesResponse> {
  return api.post<ImportDatasetTracesResponse>(
    `${datasetBase(workspaceId, datasetId)}/traces/import`,
    body,
  );
}

export function listDatasetTraceCandidates(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
  opts?: { status?: string; limit?: number; offset?: number },
): Promise<ListDatasetTraceCandidatesResponse> {
  return api.get<ListDatasetTraceCandidatesResponse>(
    `${datasetBase(workspaceId, datasetId)}/trace-candidates`,
    {
      params: {
        status: opts?.status,
        limit: opts?.limit ?? 50,
        offset: opts?.offset ?? 0,
      },
    },
  );
}

export function promoteDatasetTraceCandidate(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
  candidateId: string,
  body: PromoteDatasetTraceCandidateInput = {},
): Promise<PromoteDatasetTraceCandidateResponse> {
  return api
    .postWithMeta<PromoteDatasetTraceCandidateResponse>(
      `${datasetBase(workspaceId, datasetId)}/trace-candidates/${candidateId}/promote`,
      body,
    )
    .then((response) => response.data);
}

export function listDatasetBaselines(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
): Promise<PaginatedResponse<DatasetBaseline>> {
  return api.paginated<DatasetBaseline>(
    `${datasetBase(workspaceId, datasetId)}/baselines`,
  );
}

export function createDatasetBaseline(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
  body: CreateDatasetBaselineInput,
): Promise<DatasetBaseline> {
  return api.post<DatasetBaseline>(
    `${datasetBase(workspaceId, datasetId)}/baselines`,
    body,
  );
}

export function getDatasetRegressionSuiteLink(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
): Promise<DatasetRegressionSuiteLink> {
  return api.get<DatasetRegressionSuiteLink>(
    `${datasetBase(workspaceId, datasetId)}/regression-suite`,
  );
}

export function syncDatasetRegressionSuite(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
  body: SyncDatasetRegressionSuiteInput,
): Promise<SyncDatasetRegressionSuiteResult> {
  return api.post<SyncDatasetRegressionSuiteResult>(
    `${datasetBase(workspaceId, datasetId)}/regression-suite/sync`,
    body,
  );
}

export function startDatasetGeneration(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
  body: StartDatasetGenerationInput,
): Promise<DatasetGenerationJob> {
  return api.post<DatasetGenerationJob>(
    `${datasetBase(workspaceId, datasetId)}/generate`,
    body,
  );
}

export function getDatasetGenerationJob(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
  jobId: string,
): Promise<DatasetGenerationJob> {
  return api.get<DatasetGenerationJob>(
    `${datasetBase(workspaceId, datasetId)}/generations/${jobId}`,
  );
}

export function evaluateDatasetGate(
  api: ApiClient,
  workspaceId: string,
  datasetId: string,
  body: EvaluateDatasetGateInput,
): Promise<EvaluateDatasetGateResponse> {
  return api.post<EvaluateDatasetGateResponse>(
    `${datasetBase(workspaceId, datasetId)}/gate`,
    body,
  );
}
