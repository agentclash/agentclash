"use client";

import { useCallback, useEffect, useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import type {
  Run,
  RunAgent,
  RunRankingResponse,
  ScorecardResponse,
} from "@/lib/api/types";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Loader2, AlertTriangle } from "lucide-react";

import { Hero } from "./components/hero";
import { RankStrip } from "./components/rank-strip";
import { DimensionsDeck } from "./components/dimensions-deck";
import {
  JudgesPanel,
  MetricsPanel,
  ValidatorsPanel,
  type InspectorTarget,
} from "./components/evidence-panels";
import { InspectorSheet } from "./components/inspector-sheet";
import { CopyMarkdownButton } from "./components/copy-markdown";

const POLL_MS = 5000;

interface ScorecardClientProps {
  initialScorecard: ScorecardResponse;
  run: Run;
  agent: RunAgent;
  workspaceId: string;
}

export function ScorecardClient({
  initialScorecard,
  run,
  agent,
  workspaceId,
}: ScorecardClientProps) {
  const { getAccessToken } = useAccessToken();
  const [scorecard, setScorecard] = useState<ScorecardResponse>(initialScorecard);
  const [ranking, setRanking] = useState<RunRankingResponse | null>(null);
  const [inspected, setInspected] = useState<InspectorTarget | null>(null);

  const isPending = scorecard.state === "pending";
  const isErrored = scorecard.state === "errored";
  const isReady = scorecard.state === "ready";

  // Keep polling the scorecard while it's still evaluating.
  const fetchScorecard = useCallback(async () => {
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const res = await api.get<ScorecardResponse>(
        `/v1/scorecards/${agent.id}`,
        { allowedStatuses: [202, 409] },
      );
      setScorecard(res);
    } catch {
      // Silently fail on poll
    }
  }, [getAccessToken, agent.id]);

  useEffect(() => {
    if (!isPending) return;
    const interval = setInterval(fetchScorecard, POLL_MS);
    return () => clearInterval(interval);
  }, [isPending, fetchScorecard]);

  // Fetch run ranking for the cross-agent comparison strip. Fires once the
  // scorecard is ready (so we know scoring has settled) — in-progress runs
  // legitimately have no ranking yet.
  useEffect(() => {
    if (!isReady) return;
    let cancelled = false;
    (async () => {
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const res = await api.get<RunRankingResponse>(
          `/v1/runs/${run.id}/ranking`,
        );
        if (!cancelled) setRanking(res);
      } catch {
        if (!cancelled) setRanking(null);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [isReady, getAccessToken, run.id]);

  const doc = scorecard.scorecard;
  const validators = doc?.validator_details ?? [];
  const metrics = doc?.metric_details ?? [];
  const judges = scorecard.llm_judge_results ?? [];

  return (
    <TooltipProvider delay={250}>
      <div className="space-y-5">
        {/* Top bar: breadcrumb-adjacent actions */}
        {isReady && (
          <div className="flex items-center justify-end gap-2 -mt-2">
            <CopyMarkdownButton run={run} agent={agent} scorecard={scorecard} />
          </div>
        )}

        {isPending && <PendingState />}
        {isErrored && <ErroredState message={scorecard.message} />}

        {isReady && (
          <>
            <Hero run={run} agent={agent} scorecard={scorecard} />

            <RankStrip
              ranking={ranking}
              workspaceId={workspaceId}
              runId={run.id}
              currentRunAgentId={agent.id}
            />

            <DimensionsDeck scorecard={scorecard} />

            <ValidatorsPanel
              validators={validators}
              onInspect={setInspected}
            />
            <MetricsPanel metrics={metrics} onInspect={setInspected} />
            <JudgesPanel judges={judges} onInspect={setInspected} />
          </>
        )}

        <InspectorSheet
          target={inspected}
          onClose={() => setInspected(null)}
          replayBasePath={`/workspaces/${workspaceId}/runs/${run.id}/agents/${agent.id}/replay`}
        />
      </div>
    </TooltipProvider>
  );
}

function PendingState() {
  return (
    <div className="border border-white/[0.08] rounded-md bg-white/[0.015] px-8 py-14 text-center">
      <Loader2 className="size-5 animate-spin mx-auto mb-3 text-white/55" />
      <p className="text-[15px] text-white/85 font-medium tracking-[-0.005em]">
        Scoring in progress
      </p>
      <p className="mt-1.5 text-[12px] text-white/45 max-w-md mx-auto leading-relaxed">
        Running validators and collectors against the agent&apos;s trace. LLM judges,
        if any, run last — they can take a minute or two per sample.
      </p>
    </div>
  );
}

function ErroredState({ message }: { message?: string }) {
  return (
    <div className="border border-red-500/25 bg-red-500/[0.04] rounded-md px-8 py-14 text-center">
      <AlertTriangle className="size-5 mx-auto mb-3 text-red-400" />
      <p className="text-[15px] text-red-200 font-medium tracking-[-0.005em]">
        Scorecard unavailable
      </p>
      <p className="mt-1.5 text-[12px] text-red-200/60 max-w-md mx-auto leading-relaxed">
        {message || "An error occurred during evaluation."}
      </p>
    </div>
  );
}
