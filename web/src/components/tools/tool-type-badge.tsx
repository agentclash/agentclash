import { Badge } from "@/components/ui/badge";
import { Boxes, Workflow } from "lucide-react";
import { toolTypeLabel } from "./lib/definition";

/** A consistent badge distinguishing primitive vs composed tools. */
export function ToolTypeBadge({ kind }: { kind: string }) {
  const isComposed = kind === "composed";
  const Icon = isComposed ? Workflow : Boxes;
  return (
    <Badge variant="secondary" className="gap-1">
      <Icon className="size-3" />
      {toolTypeLabel(kind)}
    </Badge>
  );
}
