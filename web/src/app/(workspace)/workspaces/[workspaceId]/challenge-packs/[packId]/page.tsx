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
import { Layers } from "lucide-react";

const lifecycleVariant: Record<string, "default" | "secondary" | "outline"> = {
  runnable: "default",
  draft: "outline",
  deprecated: "secondary",
  archived: "secondary",
};

export default async function PackDetailPage({
  params,
}: {
  params: Promise<{ workspaceId: string; packId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId, packId } = await params;

  const api = createApiClient(accessToken);

  // The list endpoint returns all packs with their versions.
  // Filter to the target pack client-side.
  const { items: packs } = await api.get<{ items: ChallengePack[] }>(
    `/v1/workspaces/${workspaceId}/challenge-packs`,
  );

  const pack = packs.find((p) => p.id === packId);
  if (!pack) {
    return (
      <div>
        <div className="mb-6">
          <div className="flex items-center gap-3 mb-1">
            <Link
              href={`/workspaces/${workspaceId}/challenge-packs`}
              className="text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              Challenge Packs
            </Link>
            <span className="text-muted-foreground/40">/</span>
            <h1 className="text-lg font-semibold tracking-tight">Not Found</h1>
          </div>
        </div>
        <EmptyState
          icon={<Layers className="size-10" />}
          title="Pack not found"
          description="This challenge pack does not exist or you do not have access."
        />
      </div>
    );
  }

  const sortedVersions = [...pack.versions].sort(
    (a, b) => b.version_number - a.version_number,
  );

  return (
    <div>
      {/* Pack header */}
      <div className="mb-6">
        <div className="flex items-center gap-3 mb-1">
          <Link
            href={`/workspaces/${workspaceId}/challenge-packs`}
            className="text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            Challenge Packs
          </Link>
          <span className="text-muted-foreground/40">/</span>
          <h1 className="text-lg font-semibold tracking-tight">{pack.name}</h1>
        </div>
        {pack.description && (
          <p className="text-sm text-muted-foreground">{pack.description}</p>
        )}
        <div className="mt-2 flex gap-4 text-xs text-muted-foreground/60">
          <span>
            ID:{" "}
            <code className="font-[family-name:var(--font-mono)]">
              {pack.id}
            </code>
          </span>
          <span>
            Created: {new Date(pack.created_at).toLocaleDateString()}
          </span>
        </div>
      </div>

      {/* Versions */}
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-sm font-semibold">Versions</h2>
      </div>

      {sortedVersions.length === 0 ? (
        <EmptyState
          icon={<Layers className="size-10" />}
          title="No versions"
          description="This challenge pack has no published versions yet."
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Version</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sortedVersions.map((v) => (
                <TableRow key={v.id}>
                  <TableCell className="font-medium">
                    v{v.version_number}
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        lifecycleVariant[v.lifecycle_status] ?? "outline"
                      }
                    >
                      {v.lifecycle_status}
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
