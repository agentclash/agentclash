"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";

import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { useApiListQuery, useApiMutator } from "@/lib/api/swr";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/ui/page-header";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

import { ComposedBuilder } from "./composed-builder";
import { PrimitiveBuilder } from "./primitive-builder";
import { DefinitionPreview } from "./definition-preview";
import { SimulatePanel } from "./simulate-panel";
import { Field, controlClass } from "./field";
import { ToolTypeBadge } from "./tool-type-badge";
import { JsonValidityProvider } from "./json-validity";
import { useToolPrimitives } from "./use-tool-primitives";
import { declaredParamNames, emptyDefinition } from "./lib/definition";
import { validateDefinition } from "./lib/validate";
import type { ToolDefinition, ToolRecord, ToolType } from "./lib/types";

export function ToolBuilder({
  workspaceId,
  mode,
  toolType,
  toolId,
  initialName,
  initialSlug,
  initialDefinition,
}: {
  workspaceId: string;
  mode: "create" | "edit";
  toolType: ToolType;
  toolId?: string;
  initialName?: string;
  initialSlug?: string;
  initialDefinition?: ToolDefinition;
}) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const { mutateMany } = useApiMutator();

  const [name, setName] = useState(initialName ?? "");
  const [definition, setDefinition] = useState<ToolDefinition>(
    initialDefinition ?? emptyDefinition(toolType),
  );
  const [submitting, setSubmitting] = useState(false);
  const [jsonInvalid, setJsonInvalid] = useState(false);

  const { primitives } = useToolPrimitives();
  const { data: toolsData } = useApiListQuery<ToolRecord>(
    `/v1/workspaces/${workspaceId}/tools`,
  );

  const primitivesByName = useMemo(
    () => new Map(primitives.filter((p) => p.delegatable).map((p) => [p.name, p] as const)),
    [primitives],
  );
  const toolOptions = useMemo(
    () =>
      (toolsData?.items ?? [])
        .filter((t) => t.slug !== initialSlug)
        .map((t) => ({ slug: t.slug, name: t.name })),
    [toolsData, initialSlug],
  );
  const knownToolSlugs = useMemo(
    () => new Set((toolsData?.items ?? []).map((t) => t.slug)),
    [toolsData],
  );

  const primitivesReady = primitives.length > 0;
  const issues = useMemo(
    () =>
      primitivesReady
        ? validateDefinition(definition, {
            primitives: primitivesByName,
            knownToolSlugs,
            selfSlug: initialSlug,
          })
        : [],
    [primitivesReady, definition, primitivesByName, knownToolSlugs, initialSlug],
  );

  const previewIssues = jsonInvalid
    ? [...issues, { path: "editor", message: "Fix the invalid JSON in the highlighted field(s)." }]
    : issues;

  const paramNames = declaredParamNames(definition);
  const canSave =
    primitivesReady && name.trim().length > 0 && issues.length === 0 && !jsonInvalid && !submitting;

  async function save() {
    if (!name.trim()) {
      toast.error("Name is required");
      return;
    }
    setSubmitting(true);
    try {
      const api = createApiClient((await getAccessToken()) ?? undefined);
      if (mode === "create") {
        await api.post(`/v1/workspaces/${workspaceId}/tools`, {
          name: name.trim(),
          tool_kind: toolType,
          definition,
        });
      } else {
        await api.patch(`/v1/tools/${toolId}`, { name: name.trim(), definition });
      }
      await mutateMany([workspaceResourceKeys.tools(workspaceId)]);
      toast.success(mode === "create" ? "Tool created" : "Tool saved");
      router.push(`/workspaces/${workspaceId}/tools`);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to save tool");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <PageHeader
        breadcrumbs={[
          { label: "Tools", href: `/workspaces/${workspaceId}/tools` },
          { label: mode === "create" ? "New tool" : (initialName ?? "Edit") },
        ]}
        title={mode === "create" ? "New tool" : (initialName ?? "Edit tool")}
        description={
          <span className="inline-flex items-center gap-2">
            <ToolTypeBadge kind={toolType} />
            {toolType === "primitive"
              ? "A single operation agents can call."
              : "An ordered chain of primitives and other tools."}
          </span>
        }
        actions={
          <>
            <Button
              variant="outline"
              onClick={() => router.push(`/workspaces/${workspaceId}/tools`)}
              disabled={submitting}
            >
              Cancel
            </Button>
            <Button onClick={save} disabled={!canSave}>
              {submitting ? <Loader2 className="size-4 animate-spin" /> : "Save tool"}
            </Button>
          </>
        }
      />

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_24rem]">
        <JsonValidityProvider onChange={setJsonInvalid}>
          <div className="space-y-6">
            <Field label="Name" htmlFor="tool-name" hint="A human-friendly name. A stable slug is derived from it.">
              <input
                id="tool-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={toolType === "primitive" ? "e.g. lookup_order" : "e.g. refund_flow"}
                className={controlClass}
              />
            </Field>

            {definition.tool_type === "primitive" ? (
              <PrimitiveBuilder def={definition} onChange={setDefinition} primitives={primitives} />
            ) : (
              <ComposedBuilder
                def={definition}
                onChange={setDefinition}
                primitives={primitives}
                tools={toolOptions}
              />
            )}
          </div>
        </JsonValidityProvider>

        <div className="lg:sticky lg:top-4 lg:self-start">
          <Tabs defaultValue="preview">
            <TabsList className="w-full">
              <TabsTrigger value="preview">Preview</TabsTrigger>
              <TabsTrigger value="simulate">Simulate</TabsTrigger>
            </TabsList>
            <TabsContent value="preview" className="pt-3">
              <DefinitionPreview definition={definition} issues={previewIssues} />
            </TabsContent>
            <TabsContent value="simulate" className="pt-3">
              <SimulatePanel definition={definition} paramNames={paramNames} />
            </TabsContent>
          </Tabs>
        </div>
      </div>
    </div>
  );
}
