import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { WorkspaceSecret } from "@/lib/api/types";
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

export default async function SecretsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");
  const { workspaceId } = await params;

  const api = createApiClient(accessToken);
  const { items: secrets } = await api.get<{ items: WorkspaceSecret[] }>(
    `/v1/workspaces/${workspaceId}/secrets`,
  );

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">Secrets</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            Encrypted key-value pairs referenced by agents as{" "}
            <code className="font-[family-name:var(--font-mono)] text-xs">
              {"${secrets.KEY}"}
            </code>
          </p>
        </div>
        <UpsertSecretDialog workspaceId={workspaceId} />
      </div>

      {secrets.length === 0 ? (
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
              {secrets.map((s) => (
                <TableRow key={s.key}>
                  <TableCell>
                    <code className="text-sm font-[family-name:var(--font-mono)] font-medium">
                      {s.key}
                    </code>
                  </TableCell>
                  <TableCell className="text-muted-foreground text-sm">
                    ••••••••
                  </TableCell>
                  <TableCell className="text-muted-foreground text-sm">
                    {new Date(s.updated_at).toLocaleDateString()}
                  </TableCell>
                  <TableCell>
                    <DeleteSecretButton
                      workspaceId={workspaceId}
                      secretKey={s.key}
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
