import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
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

export default async function KnowledgeSourcesPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");
  const { workspaceId } = await params;

  const api = createApiClient(accessToken);
  const { items } = await api.get<{ items: KnowledgeSource[] }>(
    `/v1/workspaces/${workspaceId}/knowledge-sources`,
  );

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-lg font-semibold tracking-tight">Knowledge Sources</h1>
        <CreateResourceDialog
          title="New Knowledge Source"
          description="Connect a knowledge source for agent context retrieval."
          endpoint={`/v1/workspaces/${workspaceId}/knowledge-sources`}
          buttonLabel="New Source"
          fields={[
            { key: "name", label: "Name", placeholder: "e.g. docs-index", required: true },
            { key: "source_kind", label: "Source Kind", placeholder: "e.g. vector_store", required: true },
            { key: "connection_config", label: "Connection Config", type: "json", placeholder: '{"url": "..."}' },
          ]}
        />
      </div>

      {items.length === 0 ? (
        <EmptyState icon={<Database className="size-10" />} title="No knowledge sources" description="Connect knowledge sources for agent context retrieval." />
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
              {items.map((k) => (
                <TableRow key={k.id}>
                  <TableCell className="font-medium">{k.name}</TableCell>
                  <TableCell className="text-muted-foreground">{k.source_kind}</TableCell>
                  <TableCell><Badge variant={k.lifecycle_status === "active" ? "default" : "secondary"}>{k.lifecycle_status}</Badge></TableCell>
                  <TableCell className="text-muted-foreground">{new Date(k.created_at).toLocaleDateString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
