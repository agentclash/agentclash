import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect, notFound } from "next/navigation";
import Link from "next/link";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { Run, RunAgent } from "@/lib/api/types";
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
      {/* Breadcrumb */}
      <div className="flex items-center gap-3 mb-4">
        <Link
          href={`/workspaces/${workspaceId}/runs`}
          className="text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          Runs
        </Link>
        <span className="text-muted-foreground/40">/</span>
        <span className="text-sm text-foreground">{run.name}</span>
      </div>

      <RunDetailClient
        initialRun={run}
        initialAgents={agents}
        workspaceId={workspaceId}
      />
    </div>
  );
}
