"use client";

import type { ReplayStep } from "@/lib/api/types";
import { ReplayStepCard } from "./replay-step-card";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Loader2, ListTree } from "lucide-react";

interface ReplayTimelineProps {
  steps: ReplayStep[];
  hasMore: boolean;
  isLoadingMore: boolean;
  onLoadMore: () => void;
}

export function ReplayTimeline({
  steps,
  hasMore,
  isLoadingMore,
  onLoadMore,
}: ReplayTimelineProps) {
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
          <ReplayStepCard key={`${step.started_sequence}-${i}`} step={step} index={i} />
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
