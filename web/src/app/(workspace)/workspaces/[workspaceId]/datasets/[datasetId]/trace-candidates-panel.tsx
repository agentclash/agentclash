"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";

import {
  listDatasetTraceCandidates,
} from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  DatasetTraceCandidate,
  DatasetTraceCandidateStatus,
} from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import {
  CollapsibleSection,
  ExamplePayloadPreview,
  TagBadges,
} from "../dataset-ui-shared";
import { PromoteTraceDialog } from "./promote-trace-dialog";

const STATUS_FILTERS: Array<{
  value: "" | DatasetTraceCandidateStatus;
  label: string;
}> = [
  { value: "", label: "All statuses" },
  { value: "pending", label: "Pending" },
  { value: "promoted", label: "Promoted" },
  { value: "rejected", label: "Rejected" },
];

const inputClass =
  "rounded-lg border border-input bg-transparent [&>option]:bg-popover [&>option]:text-popover-foreground px-3 py-1.5 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

interface TraceCandidatesPanelProps {
  workspaceId: string;
  datasetId: string;
}

export function TraceCandidatesPanel({
  workspaceId,
  datasetId,
}: TraceCandidatesPanelProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [candidates, setCandidates] = useState<DatasetTraceCandidate[]>([]);
  const [loading, setLoading] = useState(true);
  const [statusFilter, setStatusFilter] = useState<
    "" | DatasetTraceCandidateStatus
  >("pending");
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [promoteCandidate, setPromoteCandidate] =
    useState<DatasetTraceCandidate | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const res = await listDatasetTraceCandidates(
        api,
        workspaceId,
        datasetId,
        {
          status: statusFilter || undefined,
          limit: 100,
        },
      );
      setCandidates(res.candidates);
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to load trace candidates",
      );
    } finally {
      setLoading(false);
    }
  }, [datasetId, getAccessToken, statusFilter, workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  if (loading) {
    return (
      <div className="flex justify-center py-12">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <label className="text-sm text-muted-foreground">Status</label>
        <select
          value={statusFilter}
          onChange={(e) =>
            setStatusFilter(e.target.value as "" | DatasetTraceCandidateStatus)
          }
          className={inputClass}
        >
          {STATUS_FILTERS.map((option) => (
            <option key={option.label} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      </div>

      {candidates.length === 0 ? (
        <EmptyState
          title="No trace candidates"
          description="Import traces to review and promote them into dataset examples."
        />
      ) : (
        <div className="space-y-3">
          {candidates.map((candidate) => (
            <div
              key={candidate.id}
              className="rounded-lg border border-border bg-card/20"
            >
              <div className="flex flex-wrap items-center gap-3 px-4 py-3">
                <button
                  type="button"
                  className="min-w-0 flex-1 text-left"
                  onClick={() =>
                    setExpandedId((prev) =>
                      prev === candidate.id ? null : candidate.id,
                    )
                  }
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-sm font-medium">
                      {candidate.external_id ?? candidate.id.slice(0, 8)}
                    </span>
                    <Badge variant="outline">{candidate.source_platform}</Badge>
                    <Badge
                      variant={
                        candidate.status === "promoted"
                          ? "default"
                          : "secondary"
                      }
                    >
                      {candidate.status}
                    </Badge>
                  </div>
                  <p className="mt-1 text-xs text-muted-foreground">
                    Created {new Date(candidate.created_at).toLocaleString()}
                  </p>
                </button>
                <TagBadges tags={candidate.tags} />
                {candidate.status === "pending" ? (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setPromoteCandidate(candidate)}
                  >
                    Promote
                  </Button>
                ) : null}
              </div>
              {expandedId === candidate.id ? (
                <div className="border-t border-border px-4 py-3">
                  <CollapsibleSection title="Trace payload" defaultOpen>
                    <ExamplePayloadPreview
                      input={candidate.input}
                      expected={candidate.output ?? candidate.expected}
                      metadata={candidate.metadata}
                    />
                  </CollapsibleSection>
                </div>
              ) : null}
            </div>
          ))}
        </div>
      )}

      {promoteCandidate ? (
        <PromoteTraceDialog
          workspaceId={workspaceId}
          datasetId={datasetId}
          candidate={promoteCandidate}
          open={Boolean(promoteCandidate)}
          onOpenChange={(open) => {
            if (!open) setPromoteCandidate(null);
          }}
          onPromoted={() => {
            setPromoteCandidate(null);
            void load();
            router.refresh();
          }}
        />
      ) : null}
    </div>
  );
}
