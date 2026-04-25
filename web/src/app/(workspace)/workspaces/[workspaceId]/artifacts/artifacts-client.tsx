"use client";

import type { ArtifactUploadResponse } from "@/lib/api/types";
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
import { UploadArtifactDialog } from "@/components/artifacts/upload-artifact-dialog";
import { DownloadArtifactButton } from "@/components/artifacts/download-artifact-button";
import { FileArchive } from "lucide-react";

function formatBytes(bytes: number | undefined | null): string {
  if (bytes == null || bytes === 0) return "\u2014";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function artifactDisplayName(artifact: ArtifactUploadResponse): string {
  const metadata = artifact.metadata as Record<string, unknown> | undefined;
  if (typeof metadata?.original_filename === "string") {
    return metadata.original_filename;
  }
  return artifact.id.slice(0, 8);
}

export function ArtifactsClient({ workspaceId }: { workspaceId: string }) {
  const { data, error, isLoading } = useApiListQuery<ArtifactUploadResponse>(
    `/v1/workspaces/${workspaceId}/artifacts`,
  );
  const items = data?.items ?? [];

  if (isLoading && !data) {
    return <WorkspaceListLoading rows={6} />;
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-lg font-semibold tracking-tight">Artifacts</h1>
        <UploadArtifactDialog workspaceId={workspaceId} />
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load artifacts.
        </div>
      ) : items.length === 0 ? (
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
              {items.map((artifact) => (
                <TableRow key={artifact.id}>
                  <TableCell className="font-medium">
                    {artifactDisplayName(artifact)}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {artifact.artifact_type}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {artifact.content_type ?? "\u2014"}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatBytes(artifact.size_bytes)}
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={artifact.visibility === "public" ? "default" : "secondary"}
                    >
                      {artifact.visibility}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(artifact.created_at).toLocaleDateString()}
                  </TableCell>
                  <TableCell>
                    <DownloadArtifactButton artifactId={artifact.id} />
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
