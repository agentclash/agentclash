"use client";

import type { ProviderAccount } from "@/lib/api/types";
import { useApiListQuery } from "@/lib/api/swr";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CreateResourceDialog } from "@/components/infra/create-resource-dialog";
import { DeleteResourceButton } from "@/components/infra/delete-resource-button";
import { Key } from "lucide-react";

const statusVariant: Record<string, "default" | "secondary" | "outline" | "destructive"> = {
  active: "default",
  paused: "outline",
  error: "destructive",
  archived: "secondary",
};

export function ProviderAccountsClient({ workspaceId }: { workspaceId: string }) {
  const { data, error, isLoading } = useApiListQuery<ProviderAccount>(
    `/v1/workspaces/${workspaceId}/provider-accounts`,
  );
  const items = data?.items ?? [];

  if (isLoading && !data) {
    return <WorkspaceListLoading rows={6} />;
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-lg font-semibold tracking-tight">Provider Accounts</h1>
        <CreateResourceDialog
          title="New Provider Account"
          description="Connect an LLM provider (OpenAI, Anthropic, etc.)."
          endpoint={`/v1/workspaces/${workspaceId}/provider-accounts`}
          buttonLabel="New Account"
          fields={[
            {
              key: "provider_key",
              label: "Provider",
              type: "select",
              required: true,
              options: [
                { value: "openai", label: "OpenAI" },
                { value: "anthropic", label: "Anthropic" },
                { value: "gemini", label: "Gemini" },
                { value: "xai", label: "xAI" },
                { value: "openrouter", label: "OpenRouter" },
                { value: "mistral", label: "Mistral" },
              ],
            },
            { key: "name", label: "Name", placeholder: "e.g. OpenAI Production", required: true },
            { key: "api_key", label: "API Key", placeholder: "sk-...", required: true },
            { key: "limits_config", label: "Limits Config", type: "json", placeholder: "{}" },
          ]}
          invalidateKeys={[workspaceResourceKeys.providerAccounts(workspaceId)]}
        />
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load provider accounts.
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          icon={<Key className="size-10" />}
          title="No provider accounts"
          description="Connect an LLM provider to use with agent deployments."
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Provider</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="w-10" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((account) => (
                <TableRow key={account.id}>
                  <TableCell className="font-medium">{account.name}</TableCell>
                  <TableCell>
                    <code className="text-xs font-[family-name:var(--font-mono)] text-muted-foreground">
                      {account.provider_key}
                    </code>
                  </TableCell>
                  <TableCell>
                    <Badge variant={statusVariant[account.status] ?? "outline"}>
                      {account.status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(account.created_at).toLocaleDateString()}
                  </TableCell>
                  <TableCell>
                    <DeleteResourceButton
                      endpoint={`/v1/provider-accounts/${account.id}`}
                      resourceName="provider account"
                      invalidateKeys={[workspaceResourceKeys.providerAccounts(workspaceId)]}
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
