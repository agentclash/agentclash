"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type { ReplayStep } from "@/lib/api/types";
import { ReplayStepCard } from "./replay-step-card";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Panel } from "@/app/(workspace)/workspaces/[workspaceId]/runs/[runId]/agents/[runAgentId]/scorecard/components/panel";
import { Badge } from "@/components/ui/badge";
import { Loader2, ListTree } from "lucide-react";
import { findHighlightIndex } from "./replay-highlight";

interface ReplayTimelineProps {
  steps: ReplayStep[];
  hasMore: boolean;
  isLoadingMore: boolean;
  onLoadMore: () => void;
  highlightSequence?: number;
}

type TurnGroup = {
  turnIndex: number | null;
  steps: ReplayStep[];
  hasMismatch: boolean;
};

function groupStepsByTurn(steps: ReplayStep[]): TurnGroup[] {
  const groups: TurnGroup[] = [];
  for (const step of steps) {
    const turnIndex = step.turn_index ?? null;
    const last = groups[groups.length - 1];
    if (last && last.turnIndex === turnIndex) {
      last.steps.push(step);
      if (step.mismatch) last.hasMismatch = true;
      continue;
    }
    groups.push({
      turnIndex,
      steps: [step],
      hasMismatch: Boolean(step.mismatch),
    });
  }
  return groups;
}

export function ReplayTimeline({
  steps,
  hasMore,
  isLoadingMore,
  onLoadMore,
  highlightSequence,
}: ReplayTimelineProps) {
  const highlightedIndex = useMemo(() => {
    if (highlightSequence == null) return -1;
    return findHighlightIndex(steps, highlightSequence);
  }, [steps, highlightSequence]);

  const flatIndexByStep = useMemo(() => {
    const map = new Map<ReplayStep, number>();
    steps.forEach((step, index) => map.set(step, index));
    return map;
  }, [steps]);

  const groups = useMemo(() => groupStepsByTurn(steps), [steps]);
  const hasTurnGroups = groups.some((g) => g.turnIndex != null);

  const highlightRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (highlightedIndex < 0) return;
    highlightRef.current?.scrollIntoView({ behavior: "smooth", block: "center" });
  }, [highlightedIndex]);

  if (steps.length === 0) {
    return (
      <EmptyState
        icon={<ListTree className="size-10 text-muted-foreground" />}
        title="No replay steps"
        description="No execution steps have been recorded yet."
      />
    );
  }

  return (
    <div>
      <Panel className="overflow-hidden">
        {groups.map((group, groupIndex) => (
          <div key={`turn-${group.turnIndex ?? "none"}-${groupIndex}`}>
            {hasTurnGroups && group.turnIndex != null && (
              <div className="flex items-center gap-2 border-b bg-muted/30 px-4 py-2 text-sm font-medium">
                <span>Turn {group.turnIndex}</span>
                {group.hasMismatch && (
                  <Badge variant="destructive" className="text-xs">
                    mismatch
                  </Badge>
                )}
              </div>
            )}
            {group.steps.map((step) => {
              const flatIndex = flatIndexByStep.get(step) ?? -1;
              return (
                <div
                  key={`${step.started_sequence}-${flatIndex}`}
                  ref={flatIndex === highlightedIndex ? highlightRef : undefined}
                >
                  <ReplayStepCard
                    step={step}
                    index={flatIndex}
                    highlighted={flatIndex === highlightedIndex}
                  />
                </div>
              );
            })}
          </div>
        ))}
      </Panel>

      {hasMore && (
        <div className="mt-4 flex justify-center">
          <Button
            variant="outline"
            size="sm"
            onClick={onLoadMore}
            disabled={isLoadingMore}
          >
            {isLoadingMore && <Loader2 className="size-4 animate-spin mr-1.5" />}
            Load more steps
          </Button>
        </div>
      )}
    </div>
  );
}
