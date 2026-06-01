import type { ApiClient } from "@/lib/api/client";
import type { Run, RunStatus } from "@/lib/api/types";

const TERMINAL_STATUSES: RunStatus[] = ["completed", "failed", "cancelled"];

export async function waitForRunCompletion(
  api: ApiClient,
  runId: string,
  opts?: {
    timeoutMs?: number;
    pollIntervalMs?: number;
    onStatus?: (run: Run) => void;
  },
): Promise<Run> {
  const timeoutMs = opts?.timeoutMs ?? 30 * 60 * 1000;
  const pollIntervalMs = opts?.pollIntervalMs ?? 5000;
  const started = Date.now();

  while (Date.now() - started < timeoutMs) {
    const run = await api.get<Run>(`/v1/runs/${runId}`);
    opts?.onStatus?.(run);
    if (TERMINAL_STATUSES.includes(run.status)) {
      return run;
    }
    await new Promise((resolve) => setTimeout(resolve, pollIntervalMs));
  }

  throw new Error("Timed out waiting for the eval run to finish");
}
