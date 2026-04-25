"use client";

import Link from "next/link";
import type { ChallengePack } from "@/lib/api/types";
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
import { Package } from "lucide-react";
import { PublishPackDialog } from "./publish-pack-dialog";

const lifecycleVariant: Record<string, "default" | "secondary" | "outline"> = {
  runnable: "default",
  draft: "outline",
  deprecated: "secondary",
  archived: "secondary",
};

export function ChallengePacksClient({ workspaceId }: { workspaceId: string }) {
  const { data, error, isLoading } = useApiListQuery<ChallengePack>(
    `/v1/workspaces/${workspaceId}/challenge-packs`,
  );
  const packs = data?.items ?? [];

  if (isLoading && !data) {
    return <WorkspaceListLoading rows={6} />;
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">
            Challenge Packs
          </h1>
          <p className="mt-0.5 text-sm text-muted-foreground">
            Benchmark definitions that agents are tested against.
          </p>
        </div>
        <PublishPackDialog workspaceId={workspaceId} />
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load challenge packs.
        </div>
      ) : packs.length === 0 ? (
        <EmptyState
          icon={<Package className="size-10" />}
          title="No challenge packs"
          description="Publish your first challenge pack to define benchmarks for agent evaluation."
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Description</TableHead>
                <TableHead>Versions</TableHead>
                <TableHead>Latest Status</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {packs.map((pack) => {
                const latestVersion =
                  pack.versions.length > 0
                    ? pack.versions.reduce((left, right) =>
                        left.version_number > right.version_number ? left : right,
                      )
                    : null;

                return (
                  <TableRow key={pack.id}>
                    <TableCell>
                      <Link
                        href={`/workspaces/${workspaceId}/challenge-packs/${pack.id}`}
                        className="font-medium text-foreground hover:underline underline-offset-4"
                      >
                        {pack.name}
                      </Link>
                    </TableCell>
                    <TableCell className="max-w-xs truncate text-sm text-muted-foreground">
                      {pack.description ?? "\u2014"}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {pack.versions.length}
                    </TableCell>
                    <TableCell>
                      {latestVersion ? (
                        <Badge
                          variant={
                            lifecycleVariant[latestVersion.lifecycle_status] ??
                            "outline"
                          }
                        >
                          {latestVersion.lifecycle_status}
                        </Badge>
                      ) : (
                        <span className="text-sm text-muted-foreground">
                          {"\u2014"}
                        </span>
                      )}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {new Date(pack.created_at).toLocaleDateString()}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
