"use client";

import { useApiListQuery } from "@/lib/api/swr";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CreateResourceDialog } from "@/components/infra/create-resource-dialog";
import { Database } from "lucide-react";

interface KnowledgeSource {
  id: string;
  name: string;
  slug: string;
  source_kind: string;
  lifecycle_status: string;
  created_at: string;
}

export function KnowledgeSourcesClient({ workspaceId }: { workspaceId: string }) {
  const { data, error, isLoading } = useApiListQuery<KnowledgeSource>(
    `/v1/workspaces/${workspaceId}/knowledge-sources`,
  );
  const items = data?.items ?? [];

  if (isLoading && !data) {
    return <WorkspaceListLoading rows={6} />;
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-lg font-semibold tracking-tight">Knowledge Sources</h1>
        <CreateResourceDialog
          title="New Knowledge Source"
          description="Connect a knowledge source for agent context retrieval."
          endpoint={`/v1/workspaces/${workspaceId}/knowledge-sources`}
          buttonLabel="New Source"
          fields={[
            { key: "name", label: "Name", placeholder: "e.g. docs-index", required: true },
            { key: "source_kind", label: "Source Kind", placeholder: "e.g. vector_store", required: true },
            { key: "connection_config", label: "Connection Config", type: "json", placeholder: '{\"url\": \"...\"}' },
          ]}
          invalidateKeys={[workspaceResourceKeys.knowledgeSources(workspaceId)]}
        />
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load knowledge sources.
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          icon={<Database className="size-10" />}
          title="No knowledge sources"
          description="Connect knowledge sources for agent context retrieval."
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Kind</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((source) => (
                <TableRow key={source.id}>
                  <TableCell className="font-medium">{source.name}</TableCell>
                  <TableCell className="text-muted-foreground">{source.source_kind}</TableCell>
                  <TableCell>
                    <Badge variant={source.lifecycle_status === "active" ? "default" : "secondary"}>
                      {source.lifecycle_status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(source.created_at).toLocaleDateString()}
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
