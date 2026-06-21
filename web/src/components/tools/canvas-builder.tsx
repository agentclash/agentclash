"use client";

import "@xyflow/react/dist/style.css";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import {
  Background,
  Controls,
  ReactFlow,
  ReactFlowProvider,
  addEdge,
  useEdgesState,
  useNodesState,
  useReactFlow,
  type Connection,
  type Edge,
  type Node,
} from "@xyflow/react";
import { PanelsTopLeft } from "lucide-react";

import { cn } from "@/lib/utils";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { useApiListQuery, useApiMutator } from "@/lib/api/swr";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
import { useGuidedTour } from "@/components/onboarding/use-guided-tour";

import { DefinitionPreview } from "./definition-preview";
import { Field, controlClass } from "./field";
import { JsonValidityProvider } from "./json-validity";
import { NodeInspector } from "./canvas/inspector";
import { EditorToolbar } from "./canvas/editor-toolbar";
import { CANVAS_TOUR_ID, canvasTourSteps } from "./canvas/tour-steps";
import { nodeTypes } from "./canvas/nodes";
import { useToolPrimitives } from "./use-tool-primitives";
import { operationLabel } from "./lib/friendly";
import {
  INPUTS_NODE_ID,
  compileGraph,
  inferKind,
  nextStepId,
  nodeReferences,
  parseDefinition,
  validateGraph,
  type CanvasNode,
  type CanvasNodeData,
  type CanvasNodeKind,
  type ToolGraph,
} from "./lib/graph";
import { emptyPrimitiveDefinition } from "./lib/definition";
import { validateDefinition } from "./lib/validate";
import type { ToolDefinition, ToolRecord, ToolType } from "./lib/types";

function labelFor(node: CanvasNode): string {
  if (node.kind === "operation") return node.data.primitive ? operationLabel(node.data.primitive) : "operation";
  if (node.kind === "tool") return node.data.toolName || node.data.slug || "tool";
  return "inputs";
}

function toRf(nodes: CanvasNode[]): Node[] {
  return nodes.map((n) => ({
    id: n.id,
    type: n.kind,
    position: n.position,
    data: n.data,
    deletable: n.kind !== "inputs",
  }));
}

function toCanvas(nodes: Node[]): CanvasNode[] {
  return nodes.map((n) => ({
    id: n.id,
    kind: (n.type ?? "operation") as CanvasNodeKind,
    position: n.position,
    data: n.data as CanvasNodeData,
  }));
}

export function ToolCanvasBuilder(props: ToolCanvasBuilderProps) {
  return (
    <ReactFlowProvider>
      <CanvasBuilderInner {...props} />
    </ReactFlowProvider>
  );
}

interface ToolCanvasBuilderProps {
  workspaceId: string;
  mode: "create" | "edit";
  /** On edit, tool_kind is immutable — the graph must compile to this kind. */
  lockedKind?: ToolType;
  toolId?: string;
  initialName?: string;
  initialSlug?: string;
  initialDefinition?: ToolDefinition;
  /** Link to the classic form editor for the same tool. */
  formEditorHref: string;
}

function CanvasBuilderInner({
  workspaceId,
  mode,
  lockedKind,
  toolId,
  initialName,
  initialSlug,
  initialDefinition,
  formEditorHref,
}: ToolCanvasBuilderProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const { mutateMany } = useApiMutator();

  const initialGraph = useMemo<ToolGraph>(
    () => parseDefinition(initialDefinition ?? emptyPrimitiveDefinition()),
    [initialDefinition],
  );

  const [name, setName] = useState(initialName ?? "");
  const [description, setDescription] = useState(initialDefinition?.description ?? "");
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>(toRf(initialGraph.nodes));
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>(initialGraph.edges as Edge[]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [jsonInvalid, setJsonInvalid] = useState(false);
  // Immersive (full-viewport) by default; minimize drops back into the page.
  const [immersive, setImmersive] = useState(true);

  const { fitView } = useReactFlow();
  const tour = useGuidedTour(CANVAS_TOUR_ID, canvasTourSteps, { autoStartOnce: true });

  // Esc leaves fullscreen — but only when the guided tour isn't using Esc itself.
  useEffect(() => {
    if (!immersive) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !tour.isActive()) setImmersive(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [immersive, tour]);

  // The canvas resizes when entering/leaving fullscreen; recenter the graph.
  useEffect(() => {
    const t = window.setTimeout(() => fitView({ duration: 200 }), 80);
    return () => window.clearTimeout(t);
  }, [immersive, fitView]);

  const { primitives } = useToolPrimitives();
  const { data: toolsData } = useApiListQuery<ToolRecord>(`/v1/workspaces/${workspaceId}/tools`);

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

  const graph: ToolGraph = useMemo(
    () => ({ nodes: toCanvas(nodes), edges: edges.map((e) => ({ id: e.id, source: e.source, target: e.target })) }),
    [nodes, edges],
  );
  const kind: ToolType = lockedKind ?? inferKind(graph.nodes);
  const definition = useMemo(
    () => compileGraph(graph, kind, { description }),
    [graph, kind, description],
  );

  const primitivesReady = primitives.length > 0;
  const issues = useMemo(() => {
    const graphIssues = validateGraph(graph, kind).map((i) => ({ path: "canvas", message: i.message }));
    const defIssues = primitivesReady
      ? validateDefinition(definition, { primitives: primitivesByName, knownToolSlugs, selfSlug: initialSlug })
      : [];
    return [...graphIssues, ...defIssues];
  }, [graph, kind, definition, primitivesReady, primitivesByName, knownToolSlugs, initialSlug]);

  const previewIssues = jsonInvalid
    ? [...issues, { path: "editor", message: "Fix the invalid JSON in the highlighted field(s)." }]
    : issues;

  const selectedNode = graph.nodes.find((n) => n.id === selectedId) ?? null;
  const references = selectedNode ? nodeReferences(selectedNode.id, graph, labelFor) : [];

  const canSave =
    primitivesReady && name.trim().length > 0 && issues.length === 0 && !jsonInvalid && !submitting;

  const patchNode = useCallback(
    (id: string, patch: Partial<CanvasNodeData>) => {
      setNodes((ns) => ns.map((n) => (n.id === id ? { ...n, data: { ...n.data, ...patch } } : n)));
    },
    [setNodes],
  );

  const addNode = useCallback(
    (nodeKind: Exclude<CanvasNodeKind, "inputs">) => {
      if (nodeKind === "canned" && nodes.some((n) => n.type === "canned")) {
        toast.error("Only one canned response is allowed.");
        return;
      }
      const canvasNodes = toCanvas(nodes);
      const stepId = nextStepId(canvasNodes);
      const id = nodeKind === "canned" ? "canned" : stepId;
      const offset = nodes.filter((n) => n.type !== "inputs").length;
      const data: CanvasNodeData =
        nodeKind === "canned"
          ? { mock: { strategy: "static" } }
          : { stepId, inputs: {} };
      const newNode: Node = {
        id,
        type: nodeKind,
        position: { x: 320, y: 24 + offset * 130 },
        data,
        deletable: true,
      };
      setNodes((ns) => [...ns, newNode]);
      if (nodeKind !== "canned") {
        setEdges((es) => addEdge({ id: `e-${INPUTS_NODE_ID}-${id}`, source: INPUTS_NODE_ID, target: id }, es));
      }
      setSelectedId(id);
    },
    [nodes, setNodes, setEdges],
  );

  const deleteNode = useCallback(
    (id: string) => {
      setNodes((ns) => ns.filter((n) => n.id !== id));
      setEdges((es) => es.filter((e) => e.source !== id && e.target !== id));
      setSelectedId(null);
    },
    [setNodes, setEdges],
  );

  const onConnect = useCallback(
    (c: Connection) => {
      if (c.source === c.target) return; // a step can't feed itself
      setEdges((es) => addEdge(c, es));
    },
    [setEdges],
  );

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
          tool_kind: kind,
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
    <div
      className={cn(
        "flex flex-col bg-background",
        immersive
          ? "fixed inset-0 z-50"
          : "h-[calc(100dvh-8rem)] min-h-[30rem] overflow-hidden rounded-lg border border-border",
      )}
    >
      <EditorToolbar
        mode={mode}
        name={name}
        onNameChange={setName}
        onAdd={addNode}
        onStartTour={tour.start}
        immersive={immersive}
        onToggleImmersive={() => setImmersive((v) => !v)}
        formEditorHref={formEditorHref}
        onExit={() => router.push(`/workspaces/${workspaceId}/tools`)}
        onSave={save}
        canSave={canSave}
        saving={submitting}
      />

      <JsonValidityProvider onChange={setJsonInvalid}>
        <div className="flex min-h-0 flex-1">
          <div className="min-w-0 flex-1">
            <ReactFlow
              nodes={nodes}
              edges={edges}
              nodeTypes={nodeTypes}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              onConnect={onConnect}
              onNodeClick={(_, node) => setSelectedId(node.id)}
              onPaneClick={() => setSelectedId(null)}
              onNodesDelete={(deleted) => {
                if (deleted.some((n) => n.id === selectedId)) setSelectedId(null);
              }}
              fitView
              colorMode="dark"
              proOptions={{ hideAttribution: true }}
            >
              <Background />
              <Controls showInteractive={false} />
            </ReactFlow>
          </div>

          <aside
            data-tour="sidebar"
            className="flex w-80 flex-shrink-0 flex-col overflow-y-auto border-l border-border"
          >
            {selectedNode ? (
              <NodeInspector
                node={selectedNode}
                references={references}
                primitives={primitives}
                tools={toolOptions}
                onPatch={(patch) => patchNode(selectedNode.id, patch)}
                onDelete={() => deleteNode(selectedNode.id)}
                onClose={() => setSelectedId(null)}
              />
            ) : (
              <div className="space-y-4 p-3">
                <Field
                  label="Description"
                  htmlFor="tool-description"
                  hint="The agent reads this to decide when to use the tool."
                >
                  <textarea
                    id="tool-description"
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    rows={2}
                    placeholder="e.g. Look up an order by its id and return its status."
                    className={controlClass}
                  />
                </Field>
                <div className="space-y-3 border-t border-border pt-3">
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <PanelsTopLeft className="size-3.5" />
                    Select a node to configure it.
                  </div>
                  <DefinitionPreview definition={definition} issues={previewIssues} />
                </div>
              </div>
            )}
          </aside>
        </div>
      </JsonValidityProvider>
    </div>
  );
}
