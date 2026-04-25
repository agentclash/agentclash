"use client";

import Link from "next/link";
import type { AgentBuild } from "@/lib/api/types";
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
import { Bot } from "lucide-react";
import { CreateBuildDialog } from "./create-build-dialog";

const statusVariant: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
  active: "default",
  archived: "secondary",
};

export function BuildsClient({ workspaceId }: { workspaceId: string }) {
  const { data, error, isLoading } = useApiListQuery<AgentBuild>(
    `/v1/workspaces/${workspaceId}/agent-builds`,
  );
  const builds = data?.items ?? [];

  if (isLoading && !data) {
    return <WorkspaceListLoading rows={6} />;
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-lg font-semibold tracking-tight">Agent Builds</h1>
        <CreateBuildDialog workspaceId={workspaceId} />
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load agent builds.
        </div>
      ) : builds.length === 0 ? (
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
                    {build.description ? (
                      <p className="mt-0.5 max-w-xs truncate text-xs text-muted-foreground">
                        {build.description}
                      </p>
                    ) : null}
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
