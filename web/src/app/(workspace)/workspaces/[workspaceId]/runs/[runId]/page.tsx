import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect, notFound } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { Run, RunAgent } from "@/lib/api/types";
import { Breadcrumbs } from "@/components/ui/breadcrumbs";
import { RunDetailClient } from "./run-detail-client";

export default async function RunDetailPage({
  params,
}: {
  params: Promise<{ workspaceId: string; runId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId, runId } = await params;

  const api = createApiClient(accessToken);

  let run: Run;
  try {
    run = await api.get<Run>(`/v1/runs/${runId}`);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      notFound();
    }
    throw err;
  }

  const { items: agents } = await api.get<{ items: RunAgent[] }>(
    `/v1/runs/${runId}/agents`,
  );

  return (
    <div>
      <Breadcrumbs
        className="mb-6"
        entries={[
          { label: "Runs", href: `/workspaces/${workspaceId}/runs` },
          { label: run.name },
        ]}
      />

      <RunDetailClient
        initialRun={run}
        initialAgents={agents}
        workspaceId={workspaceId}
      />
    </div>
  );
}
