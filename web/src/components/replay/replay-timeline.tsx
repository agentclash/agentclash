"use client";

import { useEffect, useMemo, useRef } from "react";
import type { ReplayStep } from "@/lib/api/types";
import { ReplayStepCard } from "./replay-step-card";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Loader2, ListTree } from "lucide-react";
import { findHighlightIndex } from "./replay-highlight";

interface ReplayTimelineProps {
  steps: ReplayStep[];
  hasMore: boolean;
  isLoadingMore: boolean;
  onLoadMore: () => void;
  /**
   * When set, the step whose sequence range contains this value is visually
   * highlighted and scrolled into view on mount. Used by the scorecard
   * Inspector Sheet's "View in replay" deep link.
   */
  highlightSequence?: number;
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
      {/* Timeline */}
      <div className="rounded-lg border border-border divide-y divide-border">
        {steps.map((step, i) => (
          <div
            key={`${step.started_sequence}-${i}`}
            ref={i === highlightedIndex ? highlightRef : undefined}
          >
            <ReplayStepCard
              step={step}
              index={i}
              highlighted={i === highlightedIndex}
            />
          </div>
        ))}
      </div>

      {/* Load more */}
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
