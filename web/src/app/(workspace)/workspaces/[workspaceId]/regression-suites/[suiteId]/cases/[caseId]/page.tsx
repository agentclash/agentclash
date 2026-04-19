import { withAuth } from "@workos-inc/authkit-nextjs";
import { notFound, redirect } from "next/navigation";

import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  ListRegressionCasesResponse,
  RegressionSuite,
} from "@/lib/api/types";

import { CaseDetailClient } from "./case-detail-client";

export default async function CaseDetailPage({
  params,
}: {
  params: Promise<{
    workspaceId: string;
    suiteId: string;
    caseId: string;
  }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId, suiteId, caseId } = await params;
  const api = createApiClient(accessToken);

  let suite: RegressionSuite;
  try {
    suite = await api.get<RegressionSuite>(
      `/v1/workspaces/${workspaceId}/regression-suites/${suiteId}`,
    );
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) notFound();
    throw err;
  }

  const casesResponse = await api.get<ListRegressionCasesResponse>(
    `/v1/workspaces/${workspaceId}/regression-suites/${suiteId}/cases`,
  );
  const regressionCase = casesResponse.items.find((c) => c.id === caseId);
  if (!regressionCase) notFound();

  return (
    <CaseDetailClient
      workspaceId={workspaceId}
      suite={suite}
      regressionCase={regressionCase}
    />
  );
}
