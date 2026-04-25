"use client";

import type { AgentDeployment } from "@/lib/api/types";
import { useApiListQuery } from "@/lib/api/swr";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Rocket } from "lucide-react";
import { CreateDeploymentDialog } from "./create-deployment-dialog";

const statusVariant: Record<string, "default" | "secondary" | "outline"> = {
  active: "default",
  paused: "outline",
  archived: "secondary",
};

export function DeploymentsClient({ workspaceId }: { workspaceId: string }) {
  const { data, error, isLoading } = useApiListQuery<AgentDeployment>(
    `/v1/workspaces/${workspaceId}/agent-deployments`,
  );
  const deployments = data?.items ?? [];

  if (isLoading && !data) {
    return <WorkspaceListLoading rows={6} />;
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-lg font-semibold tracking-tight">Deployments</h1>
        <CreateDeploymentDialog workspaceId={workspaceId} />
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load deployments.
        </div>
      ) : deployments.length === 0 ? (
        <EmptyState
          icon={<Rocket className="size-10" />}
          title="No deployments yet"
          description="Deploy an agent build version to make it runnable against challenge packs."
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {deployments.map((deployment) => (
                <TableRow key={deployment.id}>
                  <TableCell className="font-medium">{deployment.name}</TableCell>
                  <TableCell>
                    <Badge variant={statusVariant[deployment.status] ?? "outline"}>
                      {deployment.status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(deployment.created_at).toLocaleDateString()}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
