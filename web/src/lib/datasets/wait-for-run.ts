import type { ApiClient } from "@/lib/api/client";
import type { Run } from "@/lib/api/types";
import { isRunTerminal } from "@/lib/run-status";

export async function waitForRunCompletion(
  api: ApiClient,
  runId: string,
  opts?: {
    timeoutMs?: number;
    pollIntervalMs?: number;
    onStatus?: (run: Run) => void;
    /** Stops polling. The returned promise rejects with the abort reason. */
    signal?: AbortSignal;
  },
): Promise<Run> {
  const timeoutMs = opts?.timeoutMs ?? 30 * 60 * 1000;
  const pollIntervalMs = opts?.pollIntervalMs ?? 5000;
  const signal = opts?.signal;
  const started = Date.now();

  while (Date.now() - started < timeoutMs) {
    signal?.throwIfAborted();
    const run = await api.get<Run>(`/v1/runs/${runId}`, { signal });
    signal?.throwIfAborted();
    opts?.onStatus?.(run);
    if (isRunTerminal(run.status)) {
      return run;
    }
    await sleep(pollIntervalMs, signal);
  }

  throw new Error("Timed out waiting for the eval run to finish");
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(signal.reason);
      return;
    }
    const onAbort = () => {
      clearTimeout(timer);
      reject(signal?.reason);
    };
    const timer = setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    signal?.addEventListener("abort", onAbort, { once: true });
  });
}
