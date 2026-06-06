"use client";

import { useState } from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PageHeader } from "@/components/ui/page-header";
import { RunList } from "./run-list";
import { CreateRunDialog } from "./create-run-dialog";
import { CreateEvalSessionDialog } from "./create-eval-session-dialog";
import { EvalSessionList } from "./eval-session-list";

export function RunsPageClient({ workspaceId }: { workspaceId: string }) {
  const [createRunOpen, setCreateRunOpen] = useState(false);

  return (
    <div>
      <PageHeader
        title="Runs"
        description="Benchmark single runs and repeated eval sessions against challenge packs."
        actions={
          <>
            <CreateEvalSessionDialog workspaceId={workspaceId} />
            <CreateRunDialog
              workspaceId={workspaceId}
              open={createRunOpen}
              onOpenChange={setCreateRunOpen}
            />
          </>
        }
      />

      <Tabs defaultValue="runs" className="w-full">
        <TabsList variant="line">
          <TabsTrigger value="runs">Runs</TabsTrigger>
          <TabsTrigger value="eval-sessions">Eval Sessions</TabsTrigger>
        </TabsList>

        <TabsContent value="runs" className="pt-4">
          <RunList
            workspaceId={workspaceId}
            onCreateRun={() => setCreateRunOpen(true)}
          />
        </TabsContent>

        <TabsContent value="eval-sessions" className="pt-4">
          <EvalSessionList workspaceId={workspaceId} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
