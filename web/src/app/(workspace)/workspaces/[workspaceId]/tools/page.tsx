import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
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

export default async function ToolsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");
  const { workspaceId } = await params;

  const api = createApiClient(accessToken);
  const { items } = await api.get<{ items: Tool[] }>(
    `/v1/workspaces/${workspaceId}/tools`,
  );

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
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
        />
      </div>

      {items.length === 0 ? (
        <EmptyState icon={<Wrench className="size-10" />} title="No tools" description="Register tools that agents can use during challenge runs." />
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
              {items.map((t) => (
                <TableRow key={t.id}>
                  <TableCell className="font-medium">{t.name}</TableCell>
                  <TableCell className="text-muted-foreground">{t.tool_kind}</TableCell>
                  <TableCell><code className="text-xs font-[family-name:var(--font-mono)] text-muted-foreground">{t.capability_key}</code></TableCell>
                  <TableCell><Badge variant={t.lifecycle_status === "active" ? "default" : "secondary"}>{t.lifecycle_status}</Badge></TableCell>
                  <TableCell className="text-muted-foreground">{new Date(t.created_at).toLocaleDateString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
