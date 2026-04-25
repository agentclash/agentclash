"use client";

import type { RuntimeProfile } from "@/lib/api/types";
import { useApiListQuery } from "@/lib/api/swr";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CreateResourceDialog } from "@/components/infra/create-resource-dialog";
import { Settings2 } from "lucide-react";

export function RuntimeProfilesClient({ workspaceId }: { workspaceId: string }) {
  const { data, error, isLoading } = useApiListQuery<RuntimeProfile>(
    `/v1/workspaces/${workspaceId}/runtime-profiles`,
  );
  const items = data?.items ?? [];

  if (isLoading && !data) {
    return <WorkspaceListLoading rows={6} />;
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
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
            { key: "max_iterations", label: "Max Iterations", type: "number", placeholder: "1" },
            { key: "max_tool_calls", label: "Max Tool Calls", type: "number", placeholder: "0" },
            { key: "step_timeout_seconds", label: "Step Timeout (s)", type: "number", placeholder: "60" },
            { key: "run_timeout_seconds", label: "Run Timeout (s)", type: "number", placeholder: "300" },
            { key: "profile_config", label: "Profile Config", type: "json", placeholder: "{}" },
          ]}
          invalidateKeys={[workspaceResourceKeys.runtimeProfiles(workspaceId)]}
        />
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load runtime profiles.
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          icon={<Settings2 className="size-10" />}
          title="No runtime profiles"
          description="Create a runtime profile to define execution constraints."
        />
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
              {items.map((profile) => (
                <TableRow key={profile.id}>
                  <TableCell className="font-medium">{profile.name}</TableCell>
                  <TableCell>
                    <Badge variant="outline">{profile.execution_target}</Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {profile.trace_mode}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(profile.created_at).toLocaleDateString()}
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
