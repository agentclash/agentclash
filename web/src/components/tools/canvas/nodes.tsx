"use client";

import { Handle, Position, type NodeProps } from "@xyflow/react";
import { Boxes, MessageSquareDashed, PlayCircle, Wrench } from "lucide-react";
import { cn } from "@/lib/utils";
import { operationLabel } from "../lib/friendly";
import type { CanvasNodeData } from "../lib/graph";

const handleClass = "!size-2.5 !border-2 !border-background !bg-muted-foreground";

function Shell({
  selected,
  icon,
  tag,
  title,
  subtitle,
  tone = "default",
}: {
  selected?: boolean;
  icon: React.ReactNode;
  tag: string;
  title: string;
  subtitle?: string;
  tone?: "default" | "inputs" | "canned";
}) {
  return (
    <div
      className={cn(
        "w-56 rounded-lg border bg-card px-3 py-2.5 shadow-sm transition-colors",
        selected ? "border-primary ring-2 ring-primary/30" : "border-border",
        tone === "inputs" && "bg-muted/40",
      )}
    >
      <div className="mb-1 flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
        {icon}
        {tag}
      </div>
      <div className="truncate text-sm font-medium">{title}</div>
      {subtitle && <div className="mt-0.5 truncate text-xs text-muted-foreground">{subtitle}</div>}
    </div>
  );
}

export function InputsNode({ data, selected }: NodeProps) {
  const d = data as CanvasNodeData;
  const count = Object.keys(d.parameters?.properties ?? {}).length;
  return (
    <>
      <Shell
        selected={selected}
        tone="inputs"
        icon={<PlayCircle className="size-3.5" />}
        tag="Start"
        title="Agent inputs"
        subtitle={count === 0 ? "No inputs defined" : `${count} input${count > 1 ? "s" : ""}`}
      />
      <Handle type="source" position={Position.Right} className={handleClass} />
    </>
  );
}

export function OperationNode({ data, selected }: NodeProps) {
  const d = data as CanvasNodeData;
  return (
    <>
      <Handle type="target" position={Position.Left} className={handleClass} />
      <Shell
        selected={selected}
        icon={<Boxes className="size-3.5" />}
        tag="Operation"
        title={d.primitive ? operationLabel(d.primitive) : "Choose an operation"}
        subtitle={d.primitive || "Click to configure"}
      />
      <Handle type="source" position={Position.Right} className={handleClass} />
    </>
  );
}

export function ToolNode({ data, selected }: NodeProps) {
  const d = data as CanvasNodeData;
  return (
    <>
      <Handle type="target" position={Position.Left} className={handleClass} />
      <Shell
        selected={selected}
        icon={<Wrench className="size-3.5" />}
        tag="Tool"
        title={d.toolName || d.slug || "Choose a tool"}
        subtitle={d.slug ? "Runs another saved tool" : "Click to configure"}
      />
      <Handle type="source" position={Position.Right} className={handleClass} />
    </>
  );
}

export function CannedNode({ data, selected }: NodeProps) {
  const d = data as CanvasNodeData;
  return (
    <>
      <Handle type="target" position={Position.Left} className={handleClass} />
      <Shell
        selected={selected}
        tone="canned"
        icon={<MessageSquareDashed className="size-3.5" />}
        tag="Canned response"
        title="Returns a fixed response"
        subtitle={d.mock?.strategy ? `${d.mock.strategy} response` : "Click to configure"}
      />
    </>
  );
}

export const nodeTypes = {
  inputs: InputsNode,
  operation: OperationNode,
  tool: ToolNode,
  canned: CannedNode,
};
