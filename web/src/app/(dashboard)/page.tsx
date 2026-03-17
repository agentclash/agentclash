"use client";

import { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import { useAuthStore } from "@/lib/stores/auth";
import { api, type RunResponse, type ListRunsResponse } from "@/lib/api/client";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { RunStatusBadge } from "@/components/domain/run-status-badge";
import { Plus, ChevronLeft, ChevronRight } from "lucide-react";

function formatRelativeTime(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  if (diffSec < 60) return "just now";
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  const diffDays = Math.floor(diffHr / 24);
  return `${diffDays}d ago`;
}

export default function DashboardPage() {
  const { activeWorkspaceId } = useAuthStore();
  const [data, setData] = useState<ListRunsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [offset, setOffset] = useState(0);
  const limit = 20;

  const fetchRuns = useCallback(async () => {
    if (!activeWorkspaceId) return;
    setLoading(true);
    setError("");
    try {
      const result = await api.listRuns(activeWorkspaceId, limit, offset);
      setData(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load runs");
    } finally {
      setLoading(false);
    }
  }, [activeWorkspaceId, offset]);

  useEffect(() => {
    fetchRuns();
  }, [fetchRuns]);

  const hasNext = data ? offset + limit < data.total : false;
  const hasPrev = offset > 0;

  return (
    <div className="max-w-5xl">
      <PageHeader
        eyebrow="Dashboard"
        title="Runs"
        description="All agent evaluation runs in this workspace"
        actions={
          <Link href="/runs/new">
            <Button size="sm">
              <Plus className="size-4" data-icon="inline-start" />
              New Run
            </Button>
          </Link>
        }
      />

      {error && (
        <div className="rounded-lg border border-status-fail/20 bg-status-fail/5 p-4 mb-6">
          <p className="text-sm text-status-fail">{error}</p>
        </div>
      )}

      {loading ? (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      ) : data && data.items.length > 0 ? (
        <>
          <div className="rounded-xl border border-border overflow-hidden">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.06em] text-text-4">
                    Name
                  </TableHead>
                  <TableHead className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.06em] text-text-4">
                    Status
                  </TableHead>
                  <TableHead className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.06em] text-text-4">
                    Mode
                  </TableHead>
                  <TableHead className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.06em] text-text-4 text-right">
                    Created
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.items.map((run: RunResponse) => (
                  <TableRow key={run.id} className="group">
                    <TableCell>
                      <Link
                        href={`/runs/${run.id}`}
                        className="text-sm font-medium text-text-1 group-hover:text-ds-accent transition-colors"
                      >
                        {run.name || run.id.slice(0, 8)}
                      </Link>
                      <p className="text-[11px] text-text-3 font-[family-name:var(--font-mono)]">
                        {run.id.slice(0, 8)}
                      </p>
                    </TableCell>
                    <TableCell>
                      <RunStatusBadge status={run.status} />
                    </TableCell>
                    <TableCell>
                      <span className="text-xs text-text-2 font-[family-name:var(--font-mono)]">
                        {run.execution_mode}
                      </span>
                    </TableCell>
                    <TableCell className="text-right">
                      <span className="text-xs text-text-3">
                        {formatRelativeTime(run.created_at)}
                      </span>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          {/* Pagination */}
          <div className="flex items-center justify-between mt-4">
            <span className="text-xs text-text-3">
              {offset + 1}–{Math.min(offset + limit, data.total)} of {data.total}
            </span>
            <div className="flex gap-1">
              <Button
                variant="outline"
                size="icon-sm"
                disabled={!hasPrev}
                onClick={() => setOffset((o) => Math.max(0, o - limit))}
              >
                <ChevronLeft className="size-4" />
              </Button>
              <Button
                variant="outline"
                size="icon-sm"
                disabled={!hasNext}
                onClick={() => setOffset((o) => o + limit)}
              >
                <ChevronRight className="size-4" />
              </Button>
            </div>
          </div>
        </>
      ) : (
        <div className="text-center py-20">
          <p className="text-text-3 text-sm mb-4">No runs yet</p>
          <Link href="/runs/new">
            <Button size="sm">
              <Plus className="size-4" data-icon="inline-start" />
              Create your first run
            </Button>
          </Link>
        </div>
      )}
    </div>
  );
}
