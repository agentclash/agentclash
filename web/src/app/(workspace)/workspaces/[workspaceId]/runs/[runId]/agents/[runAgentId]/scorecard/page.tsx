import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect, notFound } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { Run, RunAgent, ScorecardResponse } from "@/lib/api/types";
import { Breadcrumbs } from "@/components/ui/breadcrumbs";
import { ScorecardClient } from "./scorecard-client";

export default async function ScorecardPage({
  params,
}: {
  params: Promise<{
    workspaceId: string;
    runId: string;
    runAgentId: string;
  }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId, runId, runAgentId } = await params;
  const api = createApiClient(accessToken);

  let run: Run;
  try {
    run = await api.get<Run>(`/v1/runs/${runId}`);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) notFound();
    throw err;
  }

  const [{ items: agents }, scorecard] = await Promise.all([
    api.get<{ items: RunAgent[] }>(`/v1/runs/${runId}/agents`),
    api.get<ScorecardResponse>(`/v1/scorecards/${runAgentId}`, {
      allowedStatuses: [202, 409],
    }),
  ]);

  const agent = agents.find((a) => a.id === runAgentId);
  if (!agent) notFound();

  return (
    <div>
      <Breadcrumbs
        className="mb-4"
        entries={[
          { label: "Runs", href: `/workspaces/${workspaceId}/runs` },
          { label: run.name, href: `/workspaces/${workspaceId}/runs/${runId}` },
          { label: `${agent.label} — Scorecard` },
        ]}
      />

      <ScorecardClient
        initialScorecard={scorecard}
        run={run}
        agent={agent}
        workspaceId={workspaceId}
      />
    </div>
  );
}
