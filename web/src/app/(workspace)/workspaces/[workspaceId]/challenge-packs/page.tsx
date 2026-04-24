import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import Link from "next/link";
import { createApiClient } from "@/lib/api/client";
import type { ChallengePack } from "@/lib/api/types";
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

export default async function ChallengePacksPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId } = await params;

  const api = createApiClient(accessToken);
  const { items: packs } = await api.get<{ items: ChallengePack[] }>(
    `/v1/workspaces/${workspaceId}/challenge-packs`,
  );

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">
            Challenge Packs
          </h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            Benchmark definitions that agents are tested against.
          </p>
        </div>
        <PublishPackDialog workspaceId={workspaceId} />
      </div>

      {packs.length === 0 ? (
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
                    ? pack.versions.reduce((a, b) =>
                        a.version_number > b.version_number ? a : b,
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
                    <TableCell className="text-muted-foreground text-sm max-w-xs truncate">
                      {pack.description ?? "\u2014"}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">
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
                        <span className="text-muted-foreground text-sm">
                          {"\u2014"}
                        </span>
                      )}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">
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
