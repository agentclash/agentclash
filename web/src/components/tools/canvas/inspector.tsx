"use client";

import { useMemo } from "react";
import Link from "next/link";
import { Trash2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ArgsEditor } from "../args-editor";
import { controlClass } from "../field";
import { MockEditor } from "../primitive-builder";
import { OperationPicker } from "../operation-picker";
import { ParametersEditor } from "../parameters-editor";
import { StepInputsEditor } from "../step-inputs-editor";
import { paramsToSchema, schemaToParams } from "../lib/definition";
import type { CanvasNode } from "../lib/graph";
import type { MockConfig, ToolPrimitive } from "../lib/types";
import type { ValueReference } from "../lib/friendly";

/**
 * Configures the selected canvas node. Each node kind reuses the same
 * humanized editors as the form builder — only the wiring (what references are
 * available) differs, and that comes from the node's incoming connections.
 */
export function NodeInspector({
  node,
  workspaceId,
  references,
  primitives,
  tools,
  onPatch,
  onDelete,
  onClose,
}: {
  node: CanvasNode;
  workspaceId: string;
  references: ValueReference[];
  primitives: ToolPrimitive[];
  tools: { slug: string; name: string }[];
  onPatch: (data: Partial<CanvasNode["data"]>) => void;
  onDelete: () => void;
  onClose: () => void;
}) {
  const delegatable = useMemo(() => primitives.filter((p) => p.delegatable), [primitives]);
  const selectedPrimitive =
    node.kind === "operation"
      ? (delegatable.find((p) => p.name === node.data.primitive) ?? null)
      : null;

  const title =
    node.kind === "inputs"
      ? "Agent inputs"
      : node.kind === "operation"
        ? "Built-in action"
        : node.kind === "tool"
          ? "Your tools"
          : "Canned response";

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b border-border px-3 py-2">
        <span className="text-sm font-medium">{title}</span>
        <div className="flex items-center gap-1">
          {node.kind !== "inputs" && (
            <Button type="button" variant="ghost" size="icon-sm" onClick={onDelete} aria-label="Delete node">
              <Trash2 className="size-3.5 text-muted-foreground" />
            </Button>
          )}
          <Button type="button" variant="ghost" size="icon-sm" onClick={onClose} aria-label="Close">
            <X className="size-3.5 text-muted-foreground" />
          </Button>
        </div>
      </div>

      <div className="flex-1 space-y-4 overflow-auto p-3">
        {node.kind === "inputs" && (
          <ParametersEditor
            params={schemaToParams(node.data.parameters)}
            onChange={(next) => onPatch({ parameters: paramsToSchema(next) })}
          />
        )}

        {node.kind === "operation" && (
          <>
            <div className="space-y-1.5">
              <h4 className="text-sm font-medium">Choose an action</h4>
              <OperationPicker
                primitives={delegatable}
                selected={node.data.primitive ?? ""}
                onSelect={(name) => onPatch({ primitive: name })}
              />
            </div>
            {selectedPrimitive && (
              <div className="space-y-1.5">
                <h4 className="text-sm font-medium">Details</h4>
                <p className="text-xs text-muted-foreground">
                  Type a value, or use Insert to pull in a wired-in input or result.
                </p>
                <ArgsEditor
                  primitive={selectedPrimitive}
                  args={node.data.inputs ?? {}}
                  onChange={(inputs) => onPatch({ inputs })}
                  references={references}
                  allowSecrets={node.data.primitive === "http_request"}
                />
              </div>
            )}
          </>
        )}

        {node.kind === "tool" &&
          (tools.length === 0 ? (
            <div className="space-y-2 rounded-md border border-dashed border-border p-3 text-xs text-muted-foreground">
              <p>This step runs another tool you&apos;ve built — but you don&apos;t have any others yet.</p>
              <p>
                <Link
                  href={`/workspaces/${workspaceId}/tools/new`}
                  className="text-foreground underline underline-offset-4"
                >
                  Add a tool from the library
                </Link>{" "}
                or build one first, then come back and connect it. For a single action, use a
                built-in action instead.
              </p>
            </div>
          ) : (
            <>
              <div>
                <label className="mb-1.5 block text-sm font-medium">Which of your tools?</label>
                <select
                  value={node.data.slug ?? ""}
                  onChange={(e) => {
                    const slug = e.target.value;
                    const name = tools.find((t) => t.slug === slug)?.name;
                    onPatch({ slug, toolName: name });
                  }}
                  className={controlClass}
                >
                  <option value="">Choose one of your tools…</option>
                  {tools.map((t) => (
                    <option key={t.slug} value={t.slug}>
                      {t.name}
                    </option>
                  ))}
                </select>
              </div>
              <StepInputsEditor
                inputs={node.data.inputs ?? {}}
                onChange={(inputs) => onPatch({ inputs })}
                references={references}
                allowSecrets={false}
                label="Inputs to pass in"
                emptyHint="Add the inputs this tool needs."
              />
            </>
          ))}

        {node.kind === "canned" && (
          <MockEditor
            mock={node.data.mock ?? { strategy: "static" }}
            onChange={(mock: MockConfig) => onPatch({ mock })}
          />
        )}
      </div>
    </div>
  );
}
