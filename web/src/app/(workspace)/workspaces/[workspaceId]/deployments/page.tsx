import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { AgentDeployment } from "@/lib/api/types";
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

export default async function DeploymentsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId } = await params;

  const api = createApiClient(accessToken);
  const { items: deployments } = await api.get<{ items: AgentDeployment[] }>(
    `/v1/workspaces/${workspaceId}/agent-deployments`,
  );

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-lg font-semibold tracking-tight">Deployments</h1>
        <CreateDeploymentDialog workspaceId={workspaceId} />
      </div>

      {deployments.length === 0 ? (
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
              {deployments.map((d) => (
                <TableRow key={d.id}>
                  <TableCell className="font-medium">{d.name}</TableCell>
                  <TableCell>
                    <Badge variant={statusVariant[d.status] ?? "outline"}>
                      {d.status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(d.created_at).toLocaleDateString()}
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
