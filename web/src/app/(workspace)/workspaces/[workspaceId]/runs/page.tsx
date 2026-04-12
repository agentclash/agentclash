import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { Run } from "@/lib/api/types";
import { RunList } from "./run-list";
import { CreateRunDialog } from "./create-run-dialog";

export default async function RunsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId } = await params;

  const api = createApiClient(accessToken);
  const res = await api.get<{
    items: Run[];
    total: number;
    limit: number;
    offset: number;
  }>(`/v1/workspaces/${workspaceId}/runs`, {
    params: { limit: 20, offset: 0 },
  });

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">Runs</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            Benchmark runs pitting agents against challenge packs.
          </p>
        </div>
        <CreateRunDialog workspaceId={workspaceId} />
      </div>

      <RunList
        workspaceId={workspaceId}
        initialRuns={res.items}
        initialTotal={res.total}
      />
    </div>
  );
}
