import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect, notFound } from "next/navigation";
import Link from "next/link";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { listRunFailures } from "@/lib/api/failure-reviews";
import type {
  ChallengePack,
  ListRunFailuresResponse,
  Run,
  RunAgent,
} from "@/lib/api/types";
import { FailuresClient } from "./failures-client";

const DEFAULT_LIMIT = 50;

export default async function RunFailuresPage({
  params,
}: {
  params: Promise<{ workspaceId: string; runId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId, runId } = await params;
  const api = createApiClient(accessToken);

  let run: Run;
  let agents: RunAgent[];
  let initialPage: ListRunFailuresResponse;
  let sourceChallengePack: ChallengePack | null = null;
  try {
    const [runRes, agentsRes, firstPage, packsRes] = await Promise.all([
      api.get<Run>(`/v1/runs/${runId}`),
      api.get<{ items: RunAgent[] }>(`/v1/runs/${runId}/agents`),
      listRunFailures(api, workspaceId, runId, { limit: DEFAULT_LIMIT }),
      api.get<{ items: ChallengePack[] }>(
        `/v1/workspaces/${workspaceId}/challenge-packs`,
      ),
    ]);
    run = runRes;
    agents = agentsRes.items;
    initialPage = firstPage;
    sourceChallengePack =
      packsRes.items.find((pack) =>
        pack.versions.some((version) => version.id === runRes.challenge_pack_version_id),
      ) ?? null;
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      notFound();
    }
    throw err;
  }

  return (
    <div>
      <div className="flex items-center gap-3 mb-4">
        <Link
          href={`/workspaces/${workspaceId}/runs`}
          className="text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          Runs
        </Link>
        <span className="text-muted-foreground/40">/</span>
        <Link
          href={`/workspaces/${workspaceId}/runs/${runId}`}
          className="text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          {run.name}
        </Link>
        <span className="text-muted-foreground/40">/</span>
        <span className="text-sm text-foreground">Failures</span>
      </div>

      <FailuresClient
        workspaceId={workspaceId}
        runId={runId}
        runName={run.name}
        agents={agents}
        initialPage={initialPage}
        initialLimit={DEFAULT_LIMIT}
        sourceChallengePackId={sourceChallengePack?.id}
        sourceChallengePackName={sourceChallengePack?.name ?? null}
      />
    </div>
  );
}
