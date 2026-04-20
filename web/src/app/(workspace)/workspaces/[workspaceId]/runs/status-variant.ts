import type { EvalSessionStatus, RunStatus } from "@/lib/api/types";

export const runStatusVariant: Record<
  RunStatus,
  "default" | "secondary" | "outline" | "destructive"
> = {
  draft: "outline",
  queued: "secondary",
  provisioning: "secondary",
  running: "outline",
  scoring: "outline",
  completed: "default",
  failed: "destructive",
  cancelled: "secondary",
};

export const evalSessionStatusVariant: Record<
  EvalSessionStatus,
  "default" | "secondary" | "outline" | "destructive"
> = {
  queued: "secondary",
  running: "outline",
  aggregating: "outline",
  completed: "default",
  failed: "destructive",
  cancelled: "secondary",
};
