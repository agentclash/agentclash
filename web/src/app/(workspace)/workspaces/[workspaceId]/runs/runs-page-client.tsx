"use client";

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { RunList } from "./run-list";
import { CreateRunDialog } from "./create-run-dialog";
import { CreateEvalSessionDialog } from "./create-eval-session-dialog";
import { EvalSessionList } from "./eval-session-list";

export function RunsPageClient({ workspaceId }: { workspaceId: string }) {
  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">Runs</h1>
          <p className="mt-0.5 text-sm text-muted-foreground">
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
          <RunList workspaceId={workspaceId} />
        </TabsContent>

        <TabsContent value="eval-sessions" className="pt-4">
          <EvalSessionList workspaceId={workspaceId} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
