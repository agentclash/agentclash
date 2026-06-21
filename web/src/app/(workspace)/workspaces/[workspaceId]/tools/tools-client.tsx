"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { MoreHorizontal, Pencil, Plus, Trash2, Wrench } from "lucide-react";

import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { useApiListQuery, useApiMutator } from "@/lib/api/swr";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { PageHeader } from "@/components/ui/page-header";
import { EmptyState } from "@/components/ui/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmProvider, useConfirm } from "@/components/ui/confirm-dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ToolTypeBadge } from "@/components/tools/tool-type-badge";
import type { ToolRecord } from "@/components/tools/lib/types";

export function ToolsClient({ workspaceId }: { workspaceId: string }) {
  return (
    <ConfirmProvider>
      <ToolsList workspaceId={workspaceId} />
    </ConfirmProvider>
  );
}

function ToolsList({ workspaceId }: { workspaceId: string }) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const { mutateMany } = useApiMutator();
  const confirm = useConfirm();
  const { data, error, isLoading } = useApiListQuery<ToolRecord>(
    `/v1/workspaces/${workspaceId}/tools`,
  );
  const items = data?.items ?? [];

  async function handleDelete(tool: ToolRecord) {
    const ok = await confirm({
      title: `Delete "${tool.name}"?`,
      description: "This archives the tool. Composed tools that reference it will no longer resolve.",
      confirmLabel: "Delete",
      variant: "danger",
    });
    if (!ok) return;
    try {
      const api = createApiClient((await getAccessToken()) ?? undefined);
      await api.del(`/v1/tools/${tool.id}`);
      await mutateMany([workspaceResourceKeys.tools(workspaceId)]);
      toast.success("Tool deleted");
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to delete tool");
    }
  }

  const newToolButton = (
    <Button size="sm" render={<Link href={`/workspaces/${workspaceId}/tools/new`} />}>
      <Plus data-icon="inline-start" className="size-4" />
      New tool
    </Button>
  );

  if (isLoading && !data) return <WorkspaceListLoading rows={6} />;

  return (
    <div>
      <PageHeader
        title="Tools"
        description="Give agents tools they can use during a run — like calling an API or running a command. No code or YAML."
        actions={items.length > 0 ? newToolButton : undefined}
      />

      {error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load tools.
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          icon={<Wrench className="size-10" />}
          title="No tools yet"
          description="Add a ready-made tool from the library — web search, Slack, read a file, query a database, and more — or build your own. No code required."
          action={{ label: "Browse the library", href: `/workspaces/${workspaceId}/tools/new` }}
        />
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {items.map((tool) => (
            <div
              key={tool.id}
              className="group relative flex flex-col rounded-lg border border-border bg-card p-4 transition-colors hover:border-foreground/20"
            >
              <div className="mb-2 flex items-start justify-between gap-2">
                <ToolTypeBadge kind={tool.tool_kind} />
                <ToolActions onEdit={() => router.push(`/workspaces/${workspaceId}/tools/${tool.id}`)} onDelete={() => handleDelete(tool)} />
              </div>
              <Link
                href={`/workspaces/${workspaceId}/tools/${tool.id}`}
                className="font-medium tracking-tight hover:underline"
              >
                {tool.name}
              </Link>
              <code className="mt-0.5 font-[family-name:var(--font-mono)] text-xs text-muted-foreground">
                {tool.slug}
              </code>
              <div className="mt-3 flex items-center justify-between">
                <Badge variant={tool.lifecycle_status === "active" ? "default" : "secondary"}>
                  {tool.lifecycle_status}
                </Badge>
                <span className="text-xs text-muted-foreground">
                  {new Date(tool.created_at).toLocaleDateString()}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function ToolActions({ onEdit, onDelete }: { onEdit: () => void; onDelete: () => void }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label="Tool actions" />}>
        <MoreHorizontal className="size-4" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={onEdit}>
          <Pencil className="size-4" />
          Edit
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onClick={onDelete}>
          <Trash2 className="size-4" />
          Delete
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
