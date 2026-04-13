"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import type {
  PlaygroundExperiment,
  PlaygroundExperimentResult,
} from "@/lib/api/types";

const POLL_INTERVAL_MS = 2000;
const ACTIVE_STATUSES = new Set<string>(["queued", "running"]);

function hasActiveExperiments(experiments: PlaygroundExperiment[]): boolean {
  return experiments.some((e) => ACTIVE_STATUSES.has(e.status));
}

interface UseExperimentPollingOptions {
  playgroundId: string;
  initialExperiments: PlaygroundExperiment[];
  enabled: boolean;
}

interface UseExperimentPollingResult {
  experiments: PlaygroundExperiment[];
  resultsByExperimentId: Record<string, PlaygroundExperimentResult[]>;
  isPolling: boolean;
  fetchResultsForExperiment: (experimentId: string) => Promise<void>;
}

export function useExperimentPolling({
  playgroundId,
  initialExperiments,
  enabled,
}: UseExperimentPollingOptions): UseExperimentPollingResult {
  const { getAccessToken } = useAccessToken();
  const [experiments, setExperiments] =
    useState<PlaygroundExperiment[]>(initialExperiments);
  const [resultsByExperimentId, setResultsByExperimentId] = useState<
    Record<string, PlaygroundExperimentResult[]>
  >({});
  const [isPolling, setIsPolling] = useState(false);
  const cancelledRef = useRef(false);
  const hasFetchedInitialRef = useRef(false);

  // Sync initial data when server-side props change
  useEffect(() => {
    setExperiments(initialExperiments);
  }, [initialExperiments]);

  const fetchResultsForExperiment = useCallback(
    async (experimentId: string) => {
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const { items } = await api.get<{
          items: PlaygroundExperimentResult[];
        }>(`/v1/playground-experiments/${experimentId}/results`);
        setResultsByExperimentId((prev) => ({ ...prev, [experimentId]: items }));
      } catch {
        // Silently fail — user can retry by collapsing/expanding
      }
    },
    [getAccessToken],
  );

  const fetchAll = useCallback(async () => {
    if (cancelledRef.current) return;
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);

      const { items: updatedExperiments } = await api.get<{
        items: PlaygroundExperiment[];
      }>(`/v1/playgrounds/${playgroundId}/experiments`);

      if (cancelledRef.current) return;
      setExperiments(updatedExperiments);

      // Fetch results for all non-queued experiments
      const experimentsToFetch = updatedExperiments.filter(
        (e) => e.status === "running" || e.status === "completed" || e.status === "failed",
      );

      const resultEntries = await Promise.all(
        experimentsToFetch.map(async (exp) => {
          try {
            const { items } = await api.get<{
              items: PlaygroundExperimentResult[];
            }>(`/v1/playground-experiments/${exp.id}/results`);
            return [exp.id, items] as const;
          } catch {
            return [exp.id, [] as PlaygroundExperimentResult[]] as const;
          }
        }),
      );

      if (cancelledRef.current) return;
      setResultsByExperimentId((prev) => {
        const next = { ...prev };
        for (const [id, results] of resultEntries) {
          next[id] = results;
        }
        return next;
      });
    } catch {
      // Swallow polling errors — stale data is better than crashing
    }
  }, [getAccessToken, playgroundId]);

  // Initial fetch when enabled — loads results for already-completed experiments
  useEffect(() => {
    if (!enabled || hasFetchedInitialRef.current) return;
    hasFetchedInitialRef.current = true;
    fetchAll();
  }, [enabled, fetchAll]);

  // Polling for active experiments
  useEffect(() => {
    const shouldPoll = enabled && hasActiveExperiments(experiments);
    setIsPolling(shouldPoll);

    if (!shouldPoll) return;

    cancelledRef.current = false;
    // Immediate tick + interval
    fetchAll();
    const interval = setInterval(fetchAll, POLL_INTERVAL_MS);

    return () => {
      cancelledRef.current = true;
      clearInterval(interval);
    };
  }, [enabled, experiments, fetchAll]);

  return { experiments, resultsByExperimentId, isPolling, fetchResultsForExperiment };
}
