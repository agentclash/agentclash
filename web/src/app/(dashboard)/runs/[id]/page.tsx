"use client";

import { useEffect, useState, use } from "react";
import Link from "next/link";
import { api, type RunResponse, type RunAgentResponse } from "@/lib/api/client";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { RunStatusBadge, AgentStatusBadge } from "@/components/domain/run-status-badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ArrowLeft, PlayCircle, FileText, BarChart3, GitCompareArrows } from "lucide-react";

function formatTimestamp(dateStr?: string): string {
  if (!dateStr) return "—";
  return new Date(dateStr).toLocaleString();
}

function formatDuration(start?: string, end?: string): string {
  if (!start || !end) return "—";
  const ms = new Date(end).getTime() - new Date(start).getTime();
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  return `${(ms / 60000).toFixed(1)}m`;
}

export default function RunDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [run, setRun] = useState<RunResponse | null>(null);
  const [agents, setAgents] = useState<RunAgentResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    async function load() {
      setLoading(true);
      try {
        const [runData, agentsData] = await Promise.all([
          api.getRun(id),
          api.listRunAgents(id),
        ]);
        setRun(runData);
        setAgents(agentsData.items);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load run");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [id]);

  if (loading) {
    return (
      <div className="max-w-5xl space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-4 w-64" />
        <div className="grid grid-cols-4 gap-4 mt-6">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </div>
        <Skeleton className="h-64 mt-6" />
      </div>
    );
  }

  if (error || !run) {
    return (
      <div className="max-w-5xl">
        <div className="rounded-lg border border-status-fail/20 bg-status-fail/5 p-6">
          <p className="text-sm text-status-fail">{error || "Run not found"}</p>
          <Link href="/" className="text-xs text-text-3 hover:text-text-1 mt-2 inline-block">
            Back to dashboard
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-5xl">
      <div className="mb-4">
        <Link
          href="/"
          className="inline-flex items-center gap-1.5 text-xs text-text-3 hover:text-text-1 transition-colors"
        >
          <ArrowLeft className="size-3" />
          Back to runs
        </Link>
      </div>

      <PageHeader
        eyebrow="Run Detail"
        title={run.name || `Run ${run.id.slice(0, 8)}`}
        actions={<RunStatusBadge status={run.status} />}
      />

      {/* Summary cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-[10px] font-semibold uppercase tracking-[0.14em] text-text-4">
              Status
            </CardTitle>
          </CardHeader>
          <CardContent>
            <RunStatusBadge status={run.status} />
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-[10px] font-semibold uppercase tracking-[0.14em] text-text-4">
              Agents
            </CardTitle>
          </CardHeader>
          <CardContent>
            <span className="text-2xl font-[family-name:var(--font-mono)] font-medium text-text-1">
              {agents.length}
            </span>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-[10px] font-semibold uppercase tracking-[0.14em] text-text-4">
              Duration
            </CardTitle>
          </CardHeader>
          <CardContent>
            <span className="text-lg font-[family-name:var(--font-mono)] text-text-1">
              {formatDuration(run.started_at, run.finished_at)}
            </span>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-[10px] font-semibold uppercase tracking-[0.14em] text-text-4">
              Mode
            </CardTitle>
          </CardHeader>
          <CardContent>
            <span className="text-sm font-[family-name:var(--font-mono)] text-text-2">
              {run.execution_mode}
            </span>
          </CardContent>
        </Card>
      </div>

      <Tabs defaultValue="agents" className="w-full">
        <TabsList className="mb-4">
          <TabsTrigger value="agents">Agents ({agents.length})</TabsTrigger>
          <TabsTrigger value="details">Details</TabsTrigger>
        </TabsList>

        <TabsContent value="agents">
          {agents.length === 0 ? (
            <div className="text-center py-12 text-text-3 text-sm">
              No agents in this run
            </div>
          ) : (
            <div className="rounded-xl border border-border overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.06em] text-text-4">
                      Lane
                    </TableHead>
                    <TableHead className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.06em] text-text-4">
                      Agent
                    </TableHead>
                    <TableHead className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.06em] text-text-4">
                      Status
                    </TableHead>
                    <TableHead className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.06em] text-text-4">
                      Duration
                    </TableHead>
                    <TableHead className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.06em] text-text-4 text-right">
                      Actions
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {agents.map((agent) => (
                    <TableRow key={agent.id} className="group">
                      <TableCell>
                        <span className="font-[family-name:var(--font-mono)] text-sm text-text-2">
                          {agent.lane_index}
                        </span>
                      </TableCell>
                      <TableCell>
                        <p className="text-sm font-medium text-text-1">
                          {agent.label}
                        </p>
                        <p className="text-[11px] text-text-3 font-[family-name:var(--font-mono)]">
                          {agent.id.slice(0, 8)}
                        </p>
                      </TableCell>
                      <TableCell>
                        <AgentStatusBadge status={agent.status} />
                        {agent.failure_reason && (
                          <p className="text-[11px] text-status-fail mt-1 max-w-xs truncate">
                            {agent.failure_reason}
                          </p>
                        )}
                      </TableCell>
                      <TableCell>
                        <span className="font-[family-name:var(--font-mono)] text-xs text-text-2">
                          {formatDuration(agent.started_at, agent.finished_at)}
                        </span>
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex items-center justify-end gap-1">
                          <Link href={`/replays/${agent.id}`}>
                            <Button variant="ghost" size="icon-xs" title="Replay">
                              <PlayCircle className="size-3.5" />
                            </Button>
                          </Link>
                          <Link href={`/scorecards/${agent.id}`}>
                            <Button variant="ghost" size="icon-xs" title="Scorecard">
                              <BarChart3 className="size-3.5" />
                            </Button>
                          </Link>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}

          {/* Compare shortcut */}
          {agents.length >= 2 && (
            <div className="mt-4 flex items-center gap-2">
              <GitCompareArrows className="size-4 text-text-3" />
              <span className="text-xs text-text-3">
                Compare agents across runs on the{" "}
                <Link href="/compare" className="text-ds-accent hover:underline">
                  Compare page
                </Link>
              </span>
            </div>
          )}
        </TabsContent>

        <TabsContent value="details">
          <div className="rounded-xl border border-border overflow-hidden">
            <div className="divide-y divide-border">
              {[
                { label: "Run ID", value: run.id },
                { label: "Workspace", value: run.workspace_id },
                { label: "Challenge Pack Version", value: run.challenge_pack_version_id },
                { label: "Input Set", value: run.challenge_input_set_id || "—" },
                { label: "Execution Mode", value: run.execution_mode },
                { label: "Temporal Workflow", value: run.temporal_workflow_id || "—" },
                { label: "Created", value: formatTimestamp(run.created_at) },
                { label: "Queued", value: formatTimestamp(run.queued_at) },
                { label: "Started", value: formatTimestamp(run.started_at) },
                { label: "Finished", value: formatTimestamp(run.finished_at) },
              ].map((row) => (
                <div key={row.label} className="flex items-center justify-between px-4 py-3">
                  <span className="text-xs text-text-3">{row.label}</span>
                  <span className="text-xs text-text-1 font-[family-name:var(--font-mono)] max-w-sm truncate">
                    {row.value}
                  </span>
                </div>
              ))}
            </div>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
