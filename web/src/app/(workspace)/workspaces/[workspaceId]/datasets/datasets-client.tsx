"use client";

import Link from "next/link";
import { Database } from "lucide-react";

import type { Dataset } from "@/lib/api/types";
import { usePaginatedApiQuery } from "@/lib/api/swr";
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

export function DatasetsClient({ workspaceId }: { workspaceId: string }) {
  const { data, error, isLoading } = usePaginatedApiQuery<Dataset>(
    `/v1/workspaces/${workspaceId}/datasets`,
  );
  const items = data?.items ?? [];

  if (isLoading && !data) {
    return <WorkspaceListLoading rows={6} />;
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-lg font-semibold tracking-tight">Datasets</h1>
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load datasets.
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          icon={<Database className="size-10" />}
          title="No datasets"
          description="Create datasets from the CLI or API to reuse examples across evals."
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Slug</TableHead>
                <TableHead>Examples</TableHead>
                <TableHead>Versions</TableHead>
                <TableHead>Schema</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((dataset) => (
                <TableRow key={dataset.id}>
                  <TableCell className="font-medium">
                    <Link
                      href={`/workspaces/${workspaceId}/datasets/${dataset.id}`}
                      className="hover:underline"
                    >
                      {dataset.name}
                    </Link>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {dataset.slug}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {dataset.active_example_count}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {dataset.version_count}
                  </TableCell>
                  <TableCell>
                    <Badge variant={dataset.input_schema_enforced ? "default" : "secondary"}>
                      {dataset.input_schema_enforced ? "enforced" : "optional"}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(dataset.created_at).toLocaleDateString()}
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
