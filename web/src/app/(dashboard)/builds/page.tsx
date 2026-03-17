"use client";

import { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import { useAuthStore } from "@/lib/stores/auth";
import { api, type AgentBuildResponse, type ListAgentBuildsResponse } from "@/lib/api/client";
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
import { Plus } from "lucide-react";

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

type BuildStatusVariant = "pass" | "fail" | "warn" | "pending" | "neutral";

function getBuildStatusVariant(status: string): BuildStatusVariant {
  switch (status) {
    case "active": return "pass";
    case "archived": return "neutral";
    case "draft": return "pending";
    default: return "neutral";
  }
}

const variantStyles: Record<BuildStatusVariant, string> = {
  pass: "text-status-pass bg-status-pass/10",
  fail: "text-status-fail bg-status-fail/10",
  warn: "text-status-warn bg-status-warn/10",
  pending: "text-text-3 bg-surface",
  neutral: "text-text-2 bg-surface",
};

function BuildStatusBadge({ status }: { status: string }) {
  const variant = getBuildStatusVariant(status);
  return (
    <span
      className={`
        inline-flex items-center
        font-[family-name:var(--font-mono)] text-[11px] font-semibold
        uppercase tracking-[0.06em]
        px-2.5 py-1 rounded
        ${variantStyles[variant]}
      `}
    >
      {status}
    </span>
  );
}

export default function BuildsPage() {
  const { activeWorkspaceId } = useAuthStore();
  const [data, setData] = useState<ListAgentBuildsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const fetchBuilds = useCallback(async () => {
    if (!activeWorkspaceId) return;
    setLoading(true);
    setError("");
    try {
      const result = await api.listAgentBuilds(activeWorkspaceId);
      setData(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load builds");
    } finally {
      setLoading(false);
    }
  }, [activeWorkspaceId]);

  useEffect(() => {
    fetchBuilds();
  }, [fetchBuilds]);

  return (
    <div className="max-w-5xl">
      <PageHeader
        eyebrow="Agent Authoring"
        title="Builds"
        description="Agent build definitions in this workspace"
        actions={
          <Link href="/builds/new">
            <Button size="sm">
              <Plus className="size-4" data-icon="inline-start" />
              New Build
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
                <TableHead className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.06em] text-text-4 text-right">
                  Updated
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.items.map((build: AgentBuildResponse) => (
                <TableRow key={build.id} className="group">
                  <TableCell>
                    <Link
                      href={`/builds/${build.id}`}
                      className="text-sm font-medium text-text-1 group-hover:text-ds-accent transition-colors"
                    >
                      {build.name}
                    </Link>
                    <p className="text-[11px] text-text-3 font-[family-name:var(--font-mono)]">
                      {build.id.slice(0, 8)}
                    </p>
                  </TableCell>
                  <TableCell>
                    <BuildStatusBadge status={build.lifecycle_status} />
                  </TableCell>
                  <TableCell className="text-right">
                    <span className="text-xs text-text-3">
                      {formatRelativeTime(build.updated_at)}
                    </span>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      ) : (
        <div className="text-center py-20">
          <p className="text-text-3 text-sm mb-4">No builds yet</p>
          <Link href="/builds/new">
            <Button size="sm">
              <Plus className="size-4" data-icon="inline-start" />
              Create your first build
            </Button>
          </Link>
        </div>
      )}
    </div>
  );
}
