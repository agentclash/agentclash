import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { ModelAlias } from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CreateResourceDialog } from "@/components/infra/create-resource-dialog";
import { Tag } from "lucide-react";

export default async function ModelAliasesPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");
  const { workspaceId } = await params;

  const api = createApiClient(accessToken);
  const { items } = await api.get<{ items: ModelAlias[] }>(
    `/v1/workspaces/${workspaceId}/model-aliases`,
  );

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
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
        />
      </div>

      {items.length === 0 ? (
        <EmptyState icon={<Tag className="size-10" />} title="No model aliases" description="Create aliases to reference specific models by name." />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Display Name</TableHead>
                <TableHead>Alias Key</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((a) => (
                <TableRow key={a.id}>
                  <TableCell className="font-medium">{a.display_name}</TableCell>
                  <TableCell><code className="text-xs font-[family-name:var(--font-mono)] text-muted-foreground">{a.alias_key}</code></TableCell>
                  <TableCell><Badge variant={a.status === "active" ? "default" : "secondary"}>{a.status}</Badge></TableCell>
                  <TableCell className="text-muted-foreground">{new Date(a.created_at).toLocaleDateString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
