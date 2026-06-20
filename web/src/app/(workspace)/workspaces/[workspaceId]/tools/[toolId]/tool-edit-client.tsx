"use client";

import { useApiQuery } from "@/lib/api/swr";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { EmptyState } from "@/components/ui/empty-state";
import { AlertTriangle } from "lucide-react";
import { ToolBuilder } from "@/components/tools/tool-builder";
import { ToolCanvasBuilder } from "@/components/tools/canvas-builder";
import { isBuilderToolType, normalizeDefinition } from "@/components/tools/lib/definition";
import type { ToolRecord } from "@/components/tools/lib/types";

export function ToolEditClient({
  workspaceId,
  toolId,
  editor,
}: {
  workspaceId: string;
  toolId: string;
  editor?: string;
}) {
  const { data: tool, error, isLoading } = useApiQuery<ToolRecord>(`/v1/tools/${toolId}`);

  if (isLoading && !tool) return <WorkspaceListLoading rows={4} />;

  if (error || !tool) {
    return (
      <EmptyState
        icon={<AlertTriangle className="size-9" />}
        title="Tool not found"
        description="This tool may have been deleted."
        action={{ label: "Back to tools", href: `/workspaces/${workspaceId}/tools` }}
      />
    );
  }

  const rawDef = tool.definition as Record<string, unknown> | undefined;
  const inferredType = (rawDef?.tool_type as string) || tool.tool_kind;

  if (!isBuilderToolType(inferredType)) {
    return (
      <div className="space-y-4">
        <EmptyState
          icon={<AlertTriangle className="size-9" />}
          title="Unsupported tool format"
          description={`This tool (kind "${tool.tool_kind}") was created outside the visual builder and can't be edited here yet.`}
          action={{ label: "Back to tools", href: `/workspaces/${workspaceId}/tools` }}
        />
        <pre className="max-h-96 overflow-auto rounded-lg border border-border bg-muted/30 p-3 font-[family-name:var(--font-mono)] text-[11px]">
          {JSON.stringify(tool.definition, null, 2)}
        </pre>
      </div>
    );
  }

  const definition = normalizeDefinition(inferredType, tool.definition);

  if (editor === "form") {
    return (
      <ToolBuilder
        workspaceId={workspaceId}
        mode="edit"
        toolType={inferredType}
        toolId={tool.id}
        initialName={tool.name}
        initialSlug={tool.slug}
        initialDefinition={definition}
      />
    );
  }

  return (
    <ToolCanvasBuilder
      workspaceId={workspaceId}
      mode="edit"
      lockedKind={inferredType}
      toolId={tool.id}
      initialName={tool.name}
      initialSlug={tool.slug}
      initialDefinition={definition}
      formEditorHref={`/workspaces/${workspaceId}/tools/${tool.id}?editor=form`}
    />
  );
}
