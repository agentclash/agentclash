"use client";

import Link from "next/link";
import type { EvalPack } from "@/lib/api/types";
import { useApiListQuery } from "@/lib/api/swr";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { VoiceModeBadges } from "@/components/voice/voice-mode-badges";
import { latestEvalPackVersion } from "@/lib/voice-evals";
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
import { NewPackButton } from "./new-pack-button";

const lifecycleVariant: Record<string, "default" | "secondary" | "outline"> = {
  runnable: "default",
  draft: "outline",
  deprecated: "secondary",
  archived: "secondary",
};

export function EvalPacksClient({ workspaceId }: { workspaceId: string }) {
  const { data, error, isLoading } = useApiListQuery<EvalPack>(
    `/v1/workspaces/${workspaceId}/eval-packs`,
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
            Eval Packs
          </h1>
          <p className="mt-0.5 text-sm text-muted-foreground">
            Benchmark definitions that agents are tested against.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Link
            href={`/workspaces/${workspaceId}/eval-packs/library`}
            className={buttonVariants({ variant: "outline", size: "sm" })}
          >
            Browse library
          </Link>
          <NewPackButton workspaceId={workspaceId} />
          <PublishPackDialog workspaceId={workspaceId} />
        </div>
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load eval packs.
        </div>
      ) : packs.length === 0 ? (
        <EmptyState
          icon={<Package className="size-10" />}
          title="No eval packs"
          description="Start from a ready-made template in the library — add it to your workspace and run it in one click — or publish your own."
          action={{
            label: "Browse the library",
            href: `/workspaces/${workspaceId}/eval-packs/library`,
          }}
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Modality</TableHead>
                <TableHead>Description</TableHead>
                <TableHead>Versions</TableHead>
                <TableHead>Latest Status</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {packs.map((pack) => {
                const latestVersion = latestEvalPackVersion(pack);

                return (
                  <TableRow key={pack.id}>
                    <TableCell>
                      <Link
                        href={`/workspaces/${workspaceId}/eval-packs/${pack.id}`}
                        className="font-medium text-foreground hover:underline underline-offset-4"
                      >
                        {pack.name}
                      </Link>
                    </TableCell>
                    <TableCell>
                      {latestVersion?.modality ? (
                        <VoiceModeBadges
                          modality={latestVersion.modality}
                          transports={latestVersion.interface_transports}
                        />
                      ) : (
                        <span className="text-sm text-muted-foreground">
                          {"\u2014"}
                        </span>
                      )}
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
