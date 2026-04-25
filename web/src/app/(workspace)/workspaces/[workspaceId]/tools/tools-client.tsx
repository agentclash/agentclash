"use client";

import { useApiListQuery } from "@/lib/api/swr";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CreateResourceDialog } from "@/components/infra/create-resource-dialog";
import { Wrench } from "lucide-react";

interface Tool {
  id: string;
  name: string;
  slug: string;
  tool_kind: string;
  capability_key: string;
  lifecycle_status: string;
  created_at: string;
}

export function ToolsClient({ workspaceId }: { workspaceId: string }) {
  const { data, error, isLoading } = useApiListQuery<Tool>(
    `/v1/workspaces/${workspaceId}/tools`,
  );
  const items = data?.items ?? [];

  if (isLoading && !data) {
    return <WorkspaceListLoading rows={6} />;
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-lg font-semibold tracking-tight">Tools</h1>
        <CreateResourceDialog
          title="New Tool"
          description="Register a tool that agents can use during runs."
          endpoint={`/v1/workspaces/${workspaceId}/tools`}
          buttonLabel="New Tool"
          fields={[
            { key: "name", label: "Name", placeholder: "e.g. code-search", required: true },
            { key: "tool_kind", label: "Tool Kind", placeholder: "e.g. function", required: true },
            { key: "capability_key", label: "Capability Key", placeholder: "e.g. search", required: true },
            { key: "definition", label: "Definition", type: "json", placeholder: "{}" },
          ]}
          invalidateKeys={[workspaceResourceKeys.tools(workspaceId)]}
        />
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load tools.
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          icon={<Wrench className="size-10" />}
          title="No tools"
          description="Register tools that agents can use during challenge runs."
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Kind</TableHead>
                <TableHead>Capability</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((tool) => (
                <TableRow key={tool.id}>
                  <TableCell className="font-medium">{tool.name}</TableCell>
                  <TableCell className="text-muted-foreground">{tool.tool_kind}</TableCell>
                  <TableCell>
                    <code className="text-xs font-[family-name:var(--font-mono)] text-muted-foreground">
                      {tool.capability_key}
                    </code>
                  </TableCell>
                  <TableCell>
                    <Badge variant={tool.lifecycle_status === "active" ? "default" : "secondary"}>
                      {tool.lifecycle_status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(tool.created_at).toLocaleDateString()}
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
