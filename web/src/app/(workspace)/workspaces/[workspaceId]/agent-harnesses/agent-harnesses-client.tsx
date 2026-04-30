"use client";

import type { AgentHarness } from "@/lib/api/types";
import { useApiListQuery } from "@/lib/api/swr";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { PackageCheck } from "lucide-react";
import { CreateAgentHarnessDialog } from "./create-agent-harness-dialog";

const authLabel: Record<string, string> = {
  chatgpt_device: "ChatGPT device",
  api_key_secret: "API key secret",
  bring_your_own_env: "Environment",
};

const statusVariant: Record<string, "default" | "secondary" | "outline"> = {
  active: "default",
  draft: "outline",
  archived: "secondary",
};

export function AgentHarnessesClient({
  workspaceId,
}: {
  workspaceId: string;
}) {
  const { data, error, isLoading } = useApiListQuery<AgentHarness>(
    `/v1/workspaces/${workspaceId}/agent-harnesses`,
  );
  const harnesses = data?.items ?? [];

  return (
    <div>
      <PageHeader
        title="Agent Harnesses"
        breadcrumbs={[{ label: "Agent Harnesses" }]}
        actions={<CreateAgentHarnessDialog workspaceId={workspaceId} />}
      />

      {isLoading && !data ? (
        <WorkspaceListLoading rows={6} />
      ) : error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load agent harnesses.
        </div>
      ) : harnesses.length === 0 ? (
        <EmptyState
          icon={<PackageCheck className="size-10" />}
          title="No agent harnesses yet"
          description="Create a Codex harness to evaluate long-running autonomous coding tasks without writing a challenge pack."
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Auth</TableHead>
                <TableHead>Codex</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Updated</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {harnesses.map((harness) => (
                <TableRow key={harness.id}>
                  <TableCell>
                    <div className="font-medium">{harness.name}</div>
                    <div className="max-w-xl truncate text-xs text-muted-foreground">
                      {harness.task_prompt}
                    </div>
                  </TableCell>
                  <TableCell>{authLabel[harness.auth_mode] ?? harness.auth_mode}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {harness.codex_model
                      ? `${harness.codex_template} / ${harness.codex_model}`
                      : harness.codex_template}
                  </TableCell>
                  <TableCell>
                    <Badge variant={statusVariant[harness.status] ?? "outline"}>
                      {harness.status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(harness.updated_at).toLocaleString()}
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
