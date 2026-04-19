import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";

import { createApiClient } from "@/lib/api/client";
import type { ChallengePack, RegressionSuite } from "@/lib/api/types";

import { RegressionSuitesClient } from "./regression-suites-client";

const PAGE_SIZE = 50;

export default async function RegressionSuitesPage({
  params,
  searchParams,
}: {
  params: Promise<{ workspaceId: string }>;
  searchParams?: Promise<{
    offset?: string;
    create?: string;
    sourcePackId?: string;
  }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId } = await params;
  const sp = (await searchParams) ?? {};
  const offset = Math.max(0, Number.parseInt(sp.offset ?? "0", 10) || 0);
  const initialCreateOpen = sp.create === "1";
  const initialCreatePackId = sp.sourcePackId;

  const api = createApiClient(accessToken);

  const [suitesPage, packsResponse] = await Promise.all([
    api.paginated<RegressionSuite>(
      `/v1/workspaces/${workspaceId}/regression-suites`,
      { limit: PAGE_SIZE, offset },
    ),
    api.get<{ items: ChallengePack[] }>(
      `/v1/workspaces/${workspaceId}/challenge-packs`,
    ),
  ]);

  return (
    <RegressionSuitesClient
      workspaceId={workspaceId}
      suites={suitesPage.items}
      total={suitesPage.total}
      offset={suitesPage.offset}
      packs={packsResponse.items}
      initialCreateOpen={initialCreateOpen}
      initialCreatePackId={initialCreatePackId}
    />
  );
}
