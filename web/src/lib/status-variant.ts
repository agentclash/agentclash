import type {
  EvalSessionStatus,
  RunAgentStatus,
  RunStatus,
} from "@/lib/api/types";

export type StatusBadgeVariant =
  | "default"
  | "secondary"
  | "outline"
  | "destructive";

// Each map names every status of its union, so the compiler finds a status
// that a map leaves out.

export const runStatusVariant: Record<RunStatus, StatusBadgeVariant> = {
  draft: "outline",
  queued: "secondary",
  provisioning: "secondary",
  running: "outline",
  scoring: "outline",
  completed: "default",
  failed: "destructive",
  cancelled: "secondary",
};

export const runAgentStatusVariant: Record<RunAgentStatus, StatusBadgeVariant> =
  {
    queued: "secondary",
    ready: "secondary",
    executing: "outline",
    evaluating: "outline",
    completed: "default",
    failed: "destructive",
  };

export const evalSessionStatusVariant: Record<
  EvalSessionStatus,
  StatusBadgeVariant
> = {
  queued: "secondary",
  running: "outline",
  aggregating: "outline",
  completed: "default",
  failed: "destructive",
  cancelled: "secondary",
};
