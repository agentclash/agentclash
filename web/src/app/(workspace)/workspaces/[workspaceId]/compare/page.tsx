import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { ComparisonResponse, Run } from "@/lib/api/types";
import Link from "next/link";
import { CompareClient } from "./compare-client";

export default async function ComparePage({
  params,
  searchParams,
}: {
  params: Promise<{ workspaceId: string }>;
  searchParams: Promise<{ baseline?: string; candidate?: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId } = await params;
  const { baseline, candidate } = await searchParams;

  if (!baseline || !candidate) {
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
          <span className="text-sm text-foreground">Compare</span>
        </div>
        <div className="rounded-lg border border-border bg-card p-8 text-center">
          <h2 className="text-lg font-semibold mb-2">No runs selected</h2>
          <p className="text-sm text-muted-foreground mb-4">
            Select two runs to compare from the{" "}
            <Link
              href={`/workspaces/${workspaceId}/runs`}
              className="text-foreground underline underline-offset-4"
            >
              runs list
            </Link>
            .
          </p>
        </div>
      </div>
    );
  }

  const api = createApiClient(accessToken);

  let comparison: ComparisonResponse;
  let baselineRun: Run;
  let candidateRun: Run;

  try {
    [comparison, baselineRun, candidateRun] = await Promise.all([
      api.get<ComparisonResponse>("/v1/compare", {
        params: {
          baseline_run_id: baseline,
          candidate_run_id: candidate,
        },
      }),
      api.get<Run>(`/v1/runs/${baseline}`),
      api.get<Run>(`/v1/runs/${candidate}`),
    ]);
  } catch (err) {
    const message =
      err instanceof ApiError ? err.message : "Failed to load comparison";
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
          <span className="text-sm text-foreground">Compare</span>
        </div>
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-6 text-center text-sm text-destructive">
          {message}
        </div>
      </div>
    );
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
        <span className="text-sm text-foreground">Compare</span>
      </div>

      <CompareClient
        comparison={comparison}
        baselineRun={baselineRun}
        candidateRun={candidateRun}
        workspaceId={workspaceId}
      />
    </div>
  );
}
