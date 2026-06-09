"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { ArrowLeft, Loader2 } from "lucide-react";

import { compareAgentTryouts } from "@/lib/api/agent-tryouts";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { AgentTryoutCompareResult } from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  formatTryoutLatency,
  tryoutModelLabel,
  tryoutStatusVariant,
} from "../status";

export function CompareTryoutsClient({ workspaceId }: { workspaceId: string }) {
  const searchParams = useSearchParams();
  const { getAccessToken } = useAccessToken();
  const [result, setResult] = useState<AgentTryoutCompareResult>();
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(true);

  const ids = useMemo(
    () =>
      (searchParams.get("ids") ?? "")
        .split(",")
        .map((value) => value.trim())
        .filter(Boolean),
    [searchParams],
  );

  useEffect(() => {
    let cancelled = false;
    async function load() {
      if (ids.length < 2 || ids.length > 4) {
        setError("Pick between 2 and 4 tryouts to compare.");
        setLoading(false);
        return;
      }
      setLoading(true);
      try {
        const token = await getAccessToken();
        const api = createApiClient(token ?? undefined);
        const response = await compareAgentTryouts(api, workspaceId, ids);
        if (!cancelled) {
          setResult(response);
          setError(undefined);
        }
      } catch (err) {
        if (!cancelled) {
          setError(
            err instanceof ApiError ? err.message : "Failed to compare tryouts",
          );
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [getAccessToken, workspaceId, ids]);

  return (
    <div>
      <Link
        href={`/workspaces/${workspaceId}/agent-tryouts`}
        className="mb-3 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="size-3.5" />
        Agent Tryouts
      </Link>
      <h1 className="mb-6 text-lg font-semibold tracking-tight">
        Compare tryouts
      </h1>

      {loading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="size-6 animate-spin text-muted-foreground" />
        </div>
      ) : error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          {error}
        </div>
      ) : result ? (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Tryout</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Model</TableHead>
                <TableHead>Cost</TableHead>
                <TableHead>Latency</TableHead>
                <TableHead>Outcome</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {result.participants.map((participant) => (
                <TableRow key={participant.id}>
                  <TableCell className="font-medium">
                    <Link
                      href={`/workspaces/${workspaceId}/agent-tryouts/${participant.id}`}
                      className="hover:underline"
                    >
                      {participant.template_slug}
                    </Link>
                    {participant.parent_tryout_id ? (
                      <Badge variant="outline" className="ml-2">
                        rerun
                      </Badge>
                    ) : null}
                  </TableCell>
                  <TableCell>
                    <Badge variant={tryoutStatusVariant(participant.status)}>
                      {participant.status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {tryoutModelLabel(participant)}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {typeof participant.actual_cost_usd === "number"
                      ? `$${participant.actual_cost_usd.toFixed(2)}`
                      : `≤ $${participant.cost_limit_usd.toFixed(2)}`}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatTryoutLatency(participant.latency_ms)}
                  </TableCell>
                  <TableCell className="max-w-72 truncate text-muted-foreground">
                    {typeof participant.summary?.message === "string"
                      ? participant.summary.message
                      : "—"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      ) : null}
    </div>
  );
}
