import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { ArtifactUploadResponse } from "@/lib/api/types";
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
import { UploadArtifactDialog } from "@/components/artifacts/upload-artifact-dialog";
import { DownloadArtifactButton } from "@/components/artifacts/download-artifact-button";
import { FileArchive } from "lucide-react";

function formatBytes(bytes: number | undefined | null): string {
  if (bytes == null || bytes === 0) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function artifactDisplayName(artifact: ArtifactUploadResponse): string {
  const meta = artifact.metadata as Record<string, unknown> | undefined;
  if (meta?.original_filename && typeof meta.original_filename === "string") {
    return meta.original_filename;
  }
  return artifact.id.slice(0, 8);
}

export default async function ArtifactsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");
  const { workspaceId } = await params;

  const api = createApiClient(accessToken);
  const { items } = await api.get<{ items: ArtifactUploadResponse[] }>(
    `/v1/workspaces/${workspaceId}/artifacts`,
  );

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-lg font-semibold tracking-tight">Artifacts</h1>
        <UploadArtifactDialog workspaceId={workspaceId} />
      </div>

      {items.length === 0 ? (
        <EmptyState
          icon={<FileArchive className="size-10" />}
          title="No artifacts"
          description="Upload files to use as context in challenge packs or attach to runs."
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Content Type</TableHead>
                <TableHead>Size</TableHead>
                <TableHead>Visibility</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="w-[1%]" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((a) => (
                <TableRow key={a.id}>
                  <TableCell className="font-medium">
                    {artifactDisplayName(a)}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {a.artifact_type}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {a.content_type ?? "—"}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatBytes(a.size_bytes)}
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        a.visibility === "public" ? "default" : "secondary"
                      }
                    >
                      {a.visibility}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(a.created_at).toLocaleDateString()}
                  </TableCell>
                  <TableCell>
                    <DownloadArtifactButton artifactId={a.id} />
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
