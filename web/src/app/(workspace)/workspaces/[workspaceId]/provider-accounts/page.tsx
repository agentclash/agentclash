import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { ProviderAccount } from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CreateResourceDialog } from "@/components/infra/create-resource-dialog";
import { DeleteResourceButton } from "@/components/infra/delete-resource-button";
import { Key } from "lucide-react";

const statusVariant: Record<string, "default" | "secondary" | "outline"> = {
  active: "default",
  paused: "outline",
  error: "destructive" as "default",
  archived: "secondary",
};

export default async function ProviderAccountsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");
  const { workspaceId } = await params;

  const api = createApiClient(accessToken);
  const { items } = await api.get<{ items: ProviderAccount[] }>(
    `/v1/workspaces/${workspaceId}/provider-accounts`,
  );

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
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
                { value: "openrouter", label: "OpenRouter" },
                { value: "mistral", label: "Mistral" },
              ],
            },
            { key: "name", label: "Name", placeholder: "e.g. OpenAI Production", required: true },
            { key: "api_key", label: "API Key", placeholder: "sk-...", required: true },
            { key: "limits_config", label: "Limits Config", type: "json", placeholder: "{}" },
          ]}
        />
      </div>

      {items.length === 0 ? (
        <EmptyState icon={<Key className="size-10" />} title="No provider accounts" description="Connect an LLM provider to use with agent deployments." />
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
              {items.map((a) => (
                <TableRow key={a.id}>
                  <TableCell className="font-medium">{a.name}</TableCell>
                  <TableCell><code className="text-xs font-[family-name:var(--font-mono)] text-muted-foreground">{a.provider_key}</code></TableCell>
                  <TableCell><Badge variant={statusVariant[a.status] ?? "outline"}>{a.status}</Badge></TableCell>
                  <TableCell className="text-muted-foreground">{new Date(a.created_at).toLocaleDateString()}</TableCell>
                  <TableCell>
                    <DeleteResourceButton
                      endpoint={`/v1/provider-accounts/${a.id}`}
                      resourceName="provider account"
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
