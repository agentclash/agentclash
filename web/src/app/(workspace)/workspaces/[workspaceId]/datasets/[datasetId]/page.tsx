import { withAuth } from "@workos-inc/authkit-nextjs";
import { notFound, redirect } from "next/navigation";

import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  Dataset,
  DatasetBaseline,
  DatasetExample,
  DatasetRegressionSuiteLink,
  DatasetVersion,
} from "@/lib/api/types";

import { DatasetDetailClient } from "./dataset-detail-client";

export default async function DatasetDetailPage({
  params,
}: {
  params: Promise<{ workspaceId: string; datasetId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId, datasetId } = await params;
  const api = createApiClient(accessToken);

  let dataset: Dataset;
  try {
    dataset = await api.get<Dataset>(
      `/v1/workspaces/${workspaceId}/datasets/${datasetId}`,
    );
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) notFound();
    throw err;
  }

  const [examples, versions, baselinesResult, regressionLink] = await Promise.all([
    api.get<{ items: DatasetExample[] }>(
      `/v1/workspaces/${workspaceId}/datasets/${datasetId}/examples`,
    ),
    api.get<{ items: DatasetVersion[] }>(
      `/v1/workspaces/${workspaceId}/datasets/${datasetId}/versions`,
    ),
    api.get<{ items: DatasetBaseline[] }>(
      `/v1/workspaces/${workspaceId}/datasets/${datasetId}/baselines`,
    ),
    api
      .get<DatasetRegressionSuiteLink>(
        `/v1/workspaces/${workspaceId}/datasets/${datasetId}/regression-suite`,
      )
      .catch((err) => {
        if (err instanceof ApiError && err.status === 404) return undefined;
        throw err;
      }),
  ]);

  return (
    <DatasetDetailClient
      dataset={dataset}
      examples={examples.items}
      versions={versions.items}
      baselines={baselinesResult.items}
      regressionLink={regressionLink}
      workspaceId={workspaceId}
    />
  );
}
