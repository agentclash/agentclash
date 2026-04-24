import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { ListEvalSessionsResponse, Run } from "@/lib/api/types";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { RunList } from "./run-list";
import { CreateRunDialog } from "./create-run-dialog";
import { CreateEvalSessionDialog } from "./create-eval-session-dialog";
import { EvalSessionList } from "./eval-session-list";

export default async function RunsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId } = await params;

  const api = createApiClient(accessToken);
  const [runsResponse, evalSessionsResponse] = await Promise.all([
    api.get<{
      items: Run[];
      total: number;
      limit: number;
      offset: number;
    }>(`/v1/workspaces/${workspaceId}/runs`, {
      params: { limit: 20, offset: 0 },
    }),
    api.get<ListEvalSessionsResponse>("/v1/eval-sessions", {
      params: { workspace_id: workspaceId, limit: 20, offset: 0 },
    }),
  ]);

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">Runs</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            Benchmark single runs and repeated eval sessions against challenge packs.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <CreateEvalSessionDialog workspaceId={workspaceId} />
          <CreateRunDialog workspaceId={workspaceId} />
        </div>
      </div>

      <Tabs defaultValue="runs" className="w-full">
        <TabsList variant="line">
          <TabsTrigger value="runs">Runs</TabsTrigger>
          <TabsTrigger value="eval-sessions">Eval Sessions</TabsTrigger>
        </TabsList>

        <TabsContent value="runs" className="pt-4">
          <RunList
            workspaceId={workspaceId}
            initialRuns={runsResponse.items}
            initialTotal={runsResponse.total}
          />
        </TabsContent>

        <TabsContent value="eval-sessions" className="pt-4">
          <EvalSessionList
            workspaceId={workspaceId}
            initialSessions={evalSessionsResponse.items}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
}
