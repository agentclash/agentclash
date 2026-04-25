"use client";

import type { WorkspaceSecret } from "@/lib/api/types";
import { useApiListQuery } from "@/lib/api/swr";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Lock } from "lucide-react";
import { UpsertSecretDialog } from "./upsert-secret-dialog";
import { DeleteSecretButton } from "./delete-secret-button";

export function SecretsClient({ workspaceId }: { workspaceId: string }) {
  const { data, error, isLoading } = useApiListQuery<WorkspaceSecret>(
    `/v1/workspaces/${workspaceId}/secrets`,
  );
  const secrets = data?.items ?? [];

  if (isLoading && !data) {
    return <WorkspaceListLoading rows={6} />;
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">Secrets</h1>
          <p className="mt-0.5 text-sm text-muted-foreground">
            Encrypted key-value pairs referenced by agents as{" "}
            <code className="text-xs font-[family-name:var(--font-mono)]">
              {"${secrets.KEY}"}
            </code>
          </p>
        </div>
        <UpsertSecretDialog workspaceId={workspaceId} />
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load secrets.
        </div>
      ) : secrets.length === 0 ? (
        <EmptyState
          icon={<Lock className="size-10" />}
          title="No secrets stored"
          description="Add secrets like API keys or database URLs that your agents can reference at runtime."
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Key</TableHead>
                <TableHead>Value</TableHead>
                <TableHead>Updated</TableHead>
                <TableHead className="w-12" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {secrets.map((secret) => (
                <TableRow key={secret.key}>
                  <TableCell>
                    <code className="text-sm font-[family-name:var(--font-mono)] font-medium">
                      {secret.key}
                    </code>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    ••••••••
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {new Date(secret.updated_at).toLocaleDateString()}
                  </TableCell>
                  <TableCell>
                    <DeleteSecretButton
                      workspaceId={workspaceId}
                      secretKey={secret.key}
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
