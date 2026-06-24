import { withAuth } from "@workos-inc/authkit-nextjs";
import { notFound, redirect } from "next/navigation";

import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  ChallengePack,
  ListRegressionCasesResponse,
  RegressionSuite,
} from "@/lib/api/types";

import { SuiteDetailClient } from "./suite-detail-client";

export default async function SuiteDetailPage({
  params,
}: {
  params: Promise<{ workspaceId: string; suiteId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId, suiteId } = await params;
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

  const [casesResponse, packsResponse] = await Promise.all([
    api.get<ListRegressionCasesResponse>(
      `/v1/workspaces/${workspaceId}/regression-suites/${suiteId}/cases`,
    ),
    api.get<{ items: ChallengePack[] }>(
      `/v1/workspaces/${workspaceId}/challenge-packs`,
    ),
  ]);

  const sourcePack =
    packsResponse.items.find(
      (p) => p.id === suite.source_challenge_pack_id,
    ) ?? null;

  return (
    <SuiteDetailClient
      workspaceId={workspaceId}
      suite={suite}
      cases={casesResponse.items}
      sourcePack={sourcePack}
    />
  );
}
