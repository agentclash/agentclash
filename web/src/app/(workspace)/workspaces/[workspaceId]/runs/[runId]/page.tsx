import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import Link from "next/link";
import { createApiClient } from "@/lib/api/client";
import type { Run } from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import type { RunStatus } from "@/lib/api/types";

const statusVariant: Record<
  RunStatus,
  "default" | "secondary" | "outline" | "destructive"
> = {
  draft: "outline",
  queued: "secondary",
  provisioning: "secondary",
  running: "outline",
  scoring: "outline",
  completed: "default",
  failed: "destructive",
  cancelled: "secondary",
};

export default async function RunDetailPage({
  params,
}: {
  params: Promise<{ workspaceId: string; runId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId, runId } = await params;

  const api = createApiClient(accessToken);
  const run = await api.get<Run>(`/v1/runs/${runId}`);

  return (
    <div>
      {/* Header */}
      <div className="mb-6">
        <div className="flex items-center gap-3 mb-1">
          <Link
            href={`/workspaces/${workspaceId}/runs`}
            className="text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            Runs
          </Link>
          <span className="text-muted-foreground/40">/</span>
          <h1 className="text-lg font-semibold tracking-tight">{run.name}</h1>
          <Badge variant={statusVariant[run.status] ?? "outline"}>
            {run.status}
          </Badge>
        </div>
        <div className="mt-2 flex flex-wrap gap-4 text-xs text-muted-foreground/60">
          <span>
            ID:{" "}
            <code className="font-[family-name:var(--font-mono)]">
              {run.id}
            </code>
          </span>
          <span>
            Mode:{" "}
            {run.execution_mode === "comparison"
              ? "Comparison"
              : "Single Agent"}
          </span>
          <span>
            Created: {new Date(run.created_at).toLocaleDateString()}
          </span>
          {run.started_at && (
            <span>
              Started: {new Date(run.started_at).toLocaleString()}
            </span>
          )}
          {run.finished_at && (
            <span>
              Finished: {new Date(run.finished_at).toLocaleString()}
            </span>
          )}
          {run.failed_at && (
            <span>
              Failed: {new Date(run.failed_at).toLocaleString()}
            </span>
          )}
        </div>
      </div>

      {/* Placeholder for future agent details, scoring, replay, etc. */}
      <div className="rounded-lg border border-border bg-card p-6 text-sm text-muted-foreground">
        Run agent details, scoring, and replay will be displayed here in a
        future update.
      </div>
    </div>
  );
}
