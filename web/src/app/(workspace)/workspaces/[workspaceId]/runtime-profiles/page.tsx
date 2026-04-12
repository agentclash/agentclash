import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { RuntimeProfile } from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CreateResourceDialog } from "@/components/infra/create-resource-dialog";
import { Settings2 } from "lucide-react";

export default async function RuntimeProfilesPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");
  const { workspaceId } = await params;

  const api = createApiClient(accessToken);
  const { items } = await api.get<{ items: RuntimeProfile[] }>(
    `/v1/workspaces/${workspaceId}/runtime-profiles`,
  );

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-lg font-semibold tracking-tight">Runtime Profiles</h1>
        <CreateResourceDialog
          title="New Runtime Profile"
          description="Define execution constraints for agent runs."
          endpoint={`/v1/workspaces/${workspaceId}/runtime-profiles`}
          buttonLabel="New Profile"
          fields={[
            { key: "name", label: "Name", placeholder: "e.g. default", required: true },
            {
              key: "execution_target",
              label: "Execution Target",
              type: "select",
              required: true,
              options: [
                { value: "native", label: "Native" },
                { value: "hosted_external", label: "Hosted External" },
              ],
            },
            { key: "max_iterations", label: "Max Iterations", placeholder: "1" },
            { key: "max_tool_calls", label: "Max Tool Calls", placeholder: "0" },
            { key: "step_timeout_seconds", label: "Step Timeout (s)", placeholder: "60" },
            { key: "run_timeout_seconds", label: "Run Timeout (s)", placeholder: "300" },
            { key: "profile_config", label: "Profile Config", type: "json", placeholder: "{}" },
          ]}
        />
      </div>

      {items.length === 0 ? (
        <EmptyState icon={<Settings2 className="size-10" />} title="No runtime profiles" description="Create a runtime profile to define execution constraints." />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Target</TableHead>
                <TableHead>Trace Mode</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((p) => (
                <TableRow key={p.id}>
                  <TableCell className="font-medium">{p.name}</TableCell>
                  <TableCell><Badge variant="outline">{p.execution_target}</Badge></TableCell>
                  <TableCell className="text-muted-foreground">{p.trace_mode}</TableCell>
                  <TableCell className="text-muted-foreground">{new Date(p.created_at).toLocaleDateString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
