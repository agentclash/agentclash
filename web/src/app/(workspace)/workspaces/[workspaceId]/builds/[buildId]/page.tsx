import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import Link from "next/link";
import { createApiClient } from "@/lib/api/client";
import type { AgentBuildDetail } from "@/lib/api/types";
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
import { Layers } from "lucide-react";
import { CreateVersionButton } from "./create-version-button";

const versionStatusVariant: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
  draft: "outline",
  ready: "default",
  archived: "secondary",
};

export default async function BuildDetailPage({
  params,
}: {
  params: Promise<{ workspaceId: string; buildId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId, buildId } = await params;

  const api = createApiClient(accessToken);
  const build = await api.get<AgentBuildDetail>(
    `/v1/agent-builds/${buildId}`,
  );

  return (
    <div>
      {/* Build header */}
      <div className="mb-6">
        <div className="flex items-center gap-3 mb-1">
          <Link
            href={`/workspaces/${workspaceId}/builds`}
            className="text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            Builds
          </Link>
          <span className="text-muted-foreground/40">/</span>
          <h1 className="text-lg font-semibold tracking-tight">
            {build.name}
          </h1>
          <Badge variant={versionStatusVariant[build.lifecycle_status] ?? "outline"}>
            {build.lifecycle_status}
          </Badge>
        </div>
        {build.description && (
          <p className="text-sm text-muted-foreground">{build.description}</p>
        )}
        <div className="mt-2 flex gap-4 text-xs text-muted-foreground/60">
          <span>
            Slug:{" "}
            <code className="font-[family-name:var(--font-mono)]">
              {build.slug}
            </code>
          </span>
          <span>
            Created: {new Date(build.created_at).toLocaleDateString()}
          </span>
        </div>
      </div>

      {/* Versions */}
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-sm font-semibold">Versions</h2>
        <CreateVersionButton buildId={buildId} workspaceId={workspaceId} />
      </div>

      {build.versions.length === 0 ? (
        <EmptyState
          icon={<Layers className="size-10" />}
          title="No versions yet"
          description="Create a version to define this agent's behavior, model, and tools."
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Version</TableHead>
                <TableHead>Agent Kind</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {build.versions.map((v) => (
                <TableRow key={v.id}>
                  <TableCell>
                    <Link
                      href={`/workspaces/${workspaceId}/builds/${buildId}/versions/${v.id}`}
                      className="font-medium text-foreground hover:underline underline-offset-4"
                    >
                      v{v.version_number}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <code className="text-xs font-[family-name:var(--font-mono)] text-muted-foreground">
                      {v.agent_kind}
                    </code>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={versionStatusVariant[v.version_status] ?? "outline"}
                    >
                      {v.version_status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(v.created_at).toLocaleDateString()}
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
