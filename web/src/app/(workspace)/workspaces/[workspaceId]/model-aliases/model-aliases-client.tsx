"use client";

import type { ModelAlias } from "@/lib/api/types";
import { useApiListQuery } from "@/lib/api/swr";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CreateResourceDialog } from "@/components/infra/create-resource-dialog";
import { DeleteResourceButton } from "@/components/infra/delete-resource-button";
import { Tag } from "lucide-react";

export function ModelAliasesClient({ workspaceId }: { workspaceId: string }) {
  const { data, error, isLoading } = useApiListQuery<ModelAlias>(
    `/v1/workspaces/${workspaceId}/model-aliases`,
  );
  const items = data?.items ?? [];

  if (isLoading && !data) {
    return <WorkspaceListLoading rows={6} />;
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-lg font-semibold tracking-tight">Model Aliases</h1>
        <CreateResourceDialog
          title="New Model Alias"
          description="Create a named reference to a model catalog entry."
          endpoint={`/v1/workspaces/${workspaceId}/model-aliases`}
          buttonLabel="New Alias"
          fields={[
            { key: "alias_key", label: "Alias Key", placeholder: "e.g. gpt4-latest", required: true },
            { key: "display_name", label: "Display Name", placeholder: "e.g. GPT-4 Latest", required: true },
            { key: "model_catalog_entry_id", label: "Model Catalog Entry ID", placeholder: "UUID", required: true },
            { key: "provider_account_id", label: "Provider Account ID", placeholder: "UUID" },
          ]}
          invalidateKeys={[workspaceResourceKeys.modelAliases(workspaceId)]}
        />
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load model aliases.
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          icon={<Tag className="size-10" />}
          title="No model aliases"
          description="Create aliases to reference specific models by name."
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Display Name</TableHead>
                <TableHead>Alias Key</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="w-10" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((alias) => (
                <TableRow key={alias.id}>
                  <TableCell className="font-medium">{alias.display_name}</TableCell>
                  <TableCell>
                    <code className="text-xs font-[family-name:var(--font-mono)] text-muted-foreground">
                      {alias.alias_key}
                    </code>
                  </TableCell>
                  <TableCell>
                    <Badge variant={alias.status === "active" ? "default" : "secondary"}>
                      {alias.status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(alias.created_at).toLocaleDateString()}
                  </TableCell>
                  <TableCell>
                    <DeleteResourceButton
                      endpoint={`/v1/model-aliases/${alias.id}`}
                      resourceName="model alias"
                      invalidateKeys={[workspaceResourceKeys.modelAliases(workspaceId)]}
                    />
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
