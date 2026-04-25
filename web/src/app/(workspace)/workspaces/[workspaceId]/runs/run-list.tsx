"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { Run, RunStatus } from "@/lib/api/types";
import { usePaginatedApiQuery } from "@/lib/api/swr";
import { workspacePageSizes } from "@/lib/workspace-resource";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
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
const POLL_INTERVAL_MS = 5000;

const PAGE_SIZE = workspacePageSizes.runs;

export function RunList({ workspaceId }: { workspaceId: string }) {
  const router = useRouter();
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const { data, error, isLoading } = usePaginatedApiQuery<Run>(
    `/v1/workspaces/${workspaceId}/runs`,
    { limit: PAGE_SIZE, offset },
    {
      refreshInterval: (response) =>
        response?.items.some((run) => ACTIVE_STATUSES.includes(run.status))
          ? POLL_INTERVAL_MS
          : 0,
    },
  );
  const runs = data?.items ?? [];
  const total = data?.total ?? 0;

  function handlePrev() {
    setOffset((current) => Math.max(0, current - PAGE_SIZE));
  }

  function handleNext() {
    const newOffset = offset + PAGE_SIZE;
    if (newOffset < total) {
      setOffset(newOffset);
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

  if (isLoading && !data) {
    return <WorkspaceListLoading rows={6} />;
  }

  if (error) {
    return (
      <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
        Failed to load runs.
      </div>
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
