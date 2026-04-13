"use client";

import { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import type { Run, RunStatus } from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Play, ChevronLeft, ChevronRight, GitCompare } from "lucide-react";
import { runStatusVariant } from "./status-variant";

const ACTIVE_STATUSES: RunStatus[] = [
  "queued",
  "provisioning",
  "running",
  "scoring",
];
const PAGE_SIZE = 20;
const POLL_INTERVAL_MS = 5000;

interface RunListProps {
  workspaceId: string;
  initialRuns: Run[];
  initialTotal: number;
}

export function RunList({
  workspaceId,
  initialRuns,
  initialTotal,
}: RunListProps) {
  const { getAccessToken } = useAccessToken();
  const router = useRouter();
  const [runs, setRuns] = useState<Run[]>(initialRuns);
  const [total, setTotal] = useState(initialTotal);
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const hasActiveRuns = runs.some((r) =>
    ACTIVE_STATUSES.includes(r.status),
  );

  const fetchRuns = useCallback(
    async (currentOffset: number) => {
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const res = await api.get<{
          items: Run[];
          total: number;
          limit: number;
          offset: number;
        }>(`/v1/workspaces/${workspaceId}/runs`, {
          params: { limit: PAGE_SIZE, offset: currentOffset },
        });
        setRuns(res.items);
        setTotal(res.total);
      } catch {
        // Silently fail on poll — data stays stale until next poll
      }
    },
    [getAccessToken, workspaceId],
  );

  // Auto-refresh when there are active runs
  useEffect(() => {
    if (!hasActiveRuns) return;
    const interval = setInterval(() => fetchRuns(offset), POLL_INTERVAL_MS);
    return () => clearInterval(interval);
  }, [hasActiveRuns, offset, fetchRuns]);

  function handlePrev() {
    const newOffset = Math.max(0, offset - PAGE_SIZE);
    setOffset(newOffset);
    fetchRuns(newOffset);
  }

  function handleNext() {
    const newOffset = offset + PAGE_SIZE;
    if (newOffset < total) {
      setOffset(newOffset);
      fetchRuns(newOffset);
    }
  }

  function toggleSelection(runId: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(runId)) {
        next.delete(runId);
      } else if (next.size < 2) {
        next.add(runId);
      } else {
        // Already have 2 selected — replace the oldest
        const [first] = next;
        next.delete(first);
        next.add(runId);
      }
      return next;
    });
  }

  function handleCompare() {
    if (selected.size !== 2) return;
    const [baseline, candidate] = Array.from(selected);
    router.push(
      `/workspaces/${workspaceId}/compare?baseline=${baseline}&candidate=${candidate}`,
    );
  }

  if (runs.length === 0 && offset === 0) {
    return (
      <EmptyState
        icon={<Play className="size-10" />}
        title="No runs yet"
        description="Create a run to benchmark your agent deployments against challenge packs."
      />
    );
  }

  const page = Math.floor(offset / PAGE_SIZE) + 1;
  const totalPages = Math.ceil(total / PAGE_SIZE);

  return (
    <>
      {/* Compare action bar */}
      {selected.size > 0 && (
        <div className="flex items-center gap-3 mb-3">
          <span className="text-sm text-muted-foreground">
            {selected.size} run{selected.size !== 1 ? "s" : ""} selected
          </span>
          <Button
            size="sm"
            disabled={selected.size !== 2}
            onClick={handleCompare}
          >
            <GitCompare className="size-4 mr-1.5" />
            Compare
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setSelected(new Set())}
          >
            Clear
          </Button>
        </div>
      )}

      <div className="rounded-lg border border-border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10" />
              <TableHead>Name</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Mode</TableHead>
              <TableHead>Created</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {runs.map((run) => (
              <TableRow
                key={run.id}
                className={selected.has(run.id) ? "bg-muted/50" : undefined}
              >
                <TableCell className="w-10">
                  <input
                    type="checkbox"
                    checked={selected.has(run.id)}
                    onChange={() => toggleSelection(run.id)}
                    className="size-4 rounded border-border accent-primary cursor-pointer"
                  />
                </TableCell>
                <TableCell>
                  <Link
                    href={`/workspaces/${workspaceId}/runs/${run.id}`}
                    className="font-medium text-foreground hover:underline underline-offset-4"
                  >
                    {run.name}
                  </Link>
                </TableCell>
                <TableCell>
                  <Badge variant={runStatusVariant[run.status] ?? "outline"}>
                    {run.status}
                  </Badge>
                </TableCell>
                <TableCell className="text-muted-foreground text-sm">
                  {run.execution_mode === "comparison"
                    ? "Comparison"
                    : "Single Agent"}
                </TableCell>
                <TableCell className="text-muted-foreground text-sm">
                  {new Date(run.created_at).toLocaleDateString()}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between mt-4">
          <p className="text-sm text-muted-foreground">
            {total} run{total !== 1 ? "s" : ""} total
          </p>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="icon-sm"
              disabled={offset === 0}
              onClick={handlePrev}
            >
              <ChevronLeft className="size-4" />
            </Button>
            <span className="text-sm text-muted-foreground">
              {page} / {totalPages}
            </span>
            <Button
              variant="outline"
              size="icon-sm"
              disabled={offset + PAGE_SIZE >= total}
              onClick={handleNext}
            >
              <ChevronRight className="size-4" />
            </Button>
          </div>
        </div>
      )}
    </>
  );
}
