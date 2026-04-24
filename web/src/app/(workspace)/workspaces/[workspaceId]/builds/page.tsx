import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import Link from "next/link";
import { createApiClient } from "@/lib/api/client";
import type { AgentBuild } from "@/lib/api/types";
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
import { Bot } from "lucide-react";
import { CreateBuildDialog } from "./create-build-dialog";

const statusVariant: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
  active: "default",
  archived: "secondary",
};

export default async function BuildsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId } = await params;

  const api = createApiClient(accessToken);
  const { items: builds } = await api.get<{ items: AgentBuild[] }>(
    `/v1/workspaces/${workspaceId}/agent-builds`,
  );

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-lg font-semibold tracking-tight">Agent Builds</h1>
        <CreateBuildDialog workspaceId={workspaceId} />
      </div>

      {builds.length === 0 ? (
        <EmptyState
          icon={<Bot className="size-10" />}
          title="No agent builds yet"
          description="Create your first agent build to define how an AI agent behaves."
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Slug</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {builds.map((build) => (
                <TableRow key={build.id}>
                  <TableCell>
                    <Link
                      href={`/workspaces/${workspaceId}/builds/${build.id}`}
                      className="font-medium text-foreground hover:underline underline-offset-4"
                    >
                      {build.name}
                    </Link>
                    {build.description && (
                      <p className="text-xs text-muted-foreground mt-0.5 truncate max-w-xs">
                        {build.description}
                      </p>
                    )}
                  </TableCell>
                  <TableCell>
                    <code className="text-xs font-[family-name:var(--font-mono)] text-muted-foreground">
                      {build.slug}
                    </code>
                  </TableCell>
                  <TableCell>
                    <Badge variant={statusVariant[build.lifecycle_status] ?? "outline"}>
                      {build.lifecycle_status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(build.created_at).toLocaleDateString()}
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
