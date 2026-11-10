"use client";

import { Suspense, useMemo, useState, type ReactNode } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import {
  ArrowUpRight,
  Bot,
  Clock3,
  DollarSign,
  Grid3x3,
  Loader2,
  Package,
  Radio,
} from "lucide-react";
import useSWR from "swr";

import { createApiClient } from "@/lib/api/client";
import { compareEvalSets, getEvalSession } from "@/lib/api/eval-sets";
import type {
  AgentDeployment,
  ChallengePack,
  CompareEvalSetsResponse,
  EvalSetStatus,
  GetEvalSetResponse,
} from "@/lib/api/types";
import { useApiListQuery, useApiQuery } from "@/lib/api/swr";
import {
  EVAL_SET_ACTIVE,
  buildMatrixGrid,
  buildRefLabelMap,
  comboRepeatLabel,
  completionPercent,
  displayRef,
  evalSetLiveKey,
  evalSetStatusLabel,
  inFlightCount,
  matrixCellStateLabel,
  type LiveStatusMap,
  type MatrixCell,
  type MatrixCellState,
  type RefLabelMap,
} from "@/lib/eval-sets";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { CaseExplorer } from "./case-explorer";
import { MatrixGridView } from "./matrix-grid";

const POLL_MS = 5000;

const SET_STATUS_BADGE: Partial<
  Record<EvalSetStatus, "default" | "secondary" | "destructive" | "outline">
> = {
  queued: "outline",
  expanding: "secondary",
  running: "default",
  aggregating: "secondary",
  completed: "secondary",
  failed: "destructive",
  cancelled: "outline",
  budget_exceeded: "destructive",
};

const CELL_STATE_TONE: Record<MatrixCellState, string> = {
  queued: "text-muted-foreground",
  running: "text-amber-700 dark:text-amber-300",
  scored: "text-emerald-700 dark:text-emerald-300",
  failed: "text-red-700 dark:text-red-300",
};

export function EvalSetDetailClient({
  workspaceId,
  evalSetId,
}: {
  workspaceId: string;
  evalSetId: string;
}) {
  return (
    <Suspense fallback={<p className="text-sm text-muted-foreground">Loading…</p>}>
      <EvalSetDetailInner workspaceId={workspaceId} evalSetId={evalSetId} />
    </Suspense>
  );
}

function EvalSetDetailInner({
  workspaceId,
  evalSetId,
}: {
  workspaceId: string;
  evalSetId: string;
}) {
  const { accessToken, getAccessToken } = useAccessToken();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const tab = searchParams.get("tab") ?? "matrix";
  const [selected, setSelected] = useState<MatrixCell | null>(null);
  const [compareId, setCompareId] = useState("");
  const [compare, setCompare] = useState<CompareEvalSetsResponse | null>(null);
  const [compareError, setCompareError] = useState<string | null>(null);

  const { data: detail, error, isLoading } = useApiQuery<GetEvalSetResponse>(
    `/v1/eval-sets/${evalSetId}`,
    undefined,
    {
      refreshInterval: (response) =>
        response && EVAL_SET_ACTIVE.includes(response.eval_set.status)
          ? POLL_MS
          : 0,
    },
  );

  const { data: deploymentsData } = useApiListQuery<AgentDeployment>(
    `/v1/workspaces/${workspaceId}/agent-deployments`,
  );
  const { data: packsData } = useApiListQuery<ChallengePack>(
    `/v1/workspaces/${workspaceId}/challenge-packs`,
  );

  const sessionIds = detail?.eval_session_ids ?? [];
  const active =
    !!detail && EVAL_SET_ACTIVE.includes(detail.eval_set.status);
  // Kept after the set ends so the matrix shows final cell states.
  const liveKey = evalSetLiveKey(
    evalSetId,
    detail?.eval_set.status,
    sessionIds,
  );

  const { data: live = {} } = useSWR<LiveStatusMap>(
    liveKey,
    async () => {
      const token = (await getAccessToken()) ?? accessToken;
      if (!token) return {};
      const api = createApiClient(token);
      const nextLive: LiveStatusMap = {};
      await Promise.all(
        sessionIds.map(async (sessionId) => {
          try {
            const session = await getEvalSession(api, sessionId);
            for (const run of session.runs ?? []) {
              if (!run.matrix_key) continue;
              nextLive[run.matrix_key] = {
                status: run.status,
                runId: run.id,
              };
            }
          } catch {
            // best-effort
          }
        }),
      );
      return nextLive;
    },
    {
      refreshInterval: active ? POLL_MS : 0,
      revalidateOnFocus: active,
      // Keep the last known cells while the final fetch runs.
      keepPreviousData: true,
    },
  );

  const grid = useMemo(
    () => (detail ? buildMatrixGrid(detail, live) : null),
    [detail, live],
  );

  const labels = useMemo(
    () =>
      detail
        ? buildRefLabelMap(
            detail,
            deploymentsData?.items,
            packsData?.items,
          )
        : {},
    [detail, deploymentsData?.items, packsData?.items],
  );

  const selectedLive = useMemo(() => {
    if (!selected || !grid) return selected;
    const key = `${selected.agentRef}\0${selected.packRef}`;
    return grid.cells[key] ?? selected;
  }, [selected, grid]);

  const setTab = (value: string) => {
    const next = new URLSearchParams(searchParams.toString());
    next.set("tab", value);
    router.replace(`${pathname}?${next.toString()}`);
  };

  if (error && !detail) {
    return (
      <EmptyState
        icon={<Grid3x3 className="h-8 w-8" />}
        title="Eval set unavailable"
        description={
          error instanceof Error ? error.message : "Failed to load eval set"
        }
      />
    );
  }
  if ((isLoading && !detail) || !detail || !grid) {
    return <p className="text-sm text-muted-foreground">Loading eval set…</p>;
  }

  const set = detail.eval_set;
  const pct = completionPercent(detail, live);
  const flight = inFlightCount(live);
  const runs = detail.result?.run_count ?? detail.result?.aggregate?.runs ?? 0;
  const sessions =
    detail.eval_session_ids?.length ?? detail.result?.session_count ?? 0;
  const selectedKey = selectedLive
    ? `${selectedLive.agentRef}\0${selectedLive.packRef}`
    : null;

  return (
    <div className="space-y-6">
      <header className="space-y-4">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="min-w-0 flex-1">
            <Link
              href={`/workspaces/${workspaceId}/eval-sets`}
              className="text-xs text-muted-foreground transition hover:text-foreground hover:underline"
            >
              ← Eval sets
            </Link>
            <div className="mt-2 flex flex-wrap items-center gap-2.5">
              <h1 className="text-2xl font-semibold tracking-tight">
                {set.name}
              </h1>
              <Badge
                variant={SET_STATUS_BADGE[set.status] ?? "outline"}
                className="capitalize"
              >
                {active ? (
                  <Loader2
                    data-icon="inline-start"
                    className="size-3 animate-spin"
                  />
                ) : null}
                {evalSetStatusLabel(set.status)}
              </Badge>
              {active ? (
                <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
                  <Radio className="size-3 text-emerald-500" aria-hidden />
                  Live
                </span>
              ) : null}
            </div>
            <p className="mt-1.5 text-sm text-muted-foreground">
              {set.combination_count} combinations · {sessions} sessions ·{" "}
              {runs} runs
            </p>
          </div>
          <div className="grid w-full grid-cols-2 gap-2 sm:w-auto sm:grid-cols-4">
            <Stat
              label="Complete"
              value={`${pct}%`}
              hint={`${Math.round((pct / 100) * set.combination_count)} / ${set.combination_count}`}
            />
            <Stat
              label="In flight"
              value={String(flight)}
              accent={flight > 0 ? "amber" : undefined}
            />
            <Stat
              label="Elapsed"
              icon={<Clock3 className="size-3" />}
              value={
                set.started_at
                  ? formatElapsed(set.started_at, set.finished_at)
                  : "—"
              }
            />
            <Stat
              label="Spend"
              icon={<DollarSign className="size-3" />}
              value={
                set.spent_usd != null || set.budget_usd != null
                  ? `$${(set.spent_usd ?? 0).toFixed(2)}${
                      set.budget_usd != null
                        ? ` / $${Number(set.budget_usd).toFixed(0)}`
                        : ""
                    }`
                  : "—"
              }
            />
          </div>
        </div>

        <div className="space-y-1.5">
          <div className="flex items-center justify-between text-xs text-muted-foreground">
            <span>Overall progress</span>
            <span className="tabular-nums font-medium text-foreground">
              {pct}%
            </span>
          </div>
          <div className="h-1.5 overflow-hidden rounded-full bg-muted">
            <div
              className={cn(
                "h-full rounded-full transition-[width] duration-700 ease-out",
                set.status === "failed" || set.status === "budget_exceeded"
                  ? "bg-red-500"
                  : set.status === "completed"
                    ? "bg-emerald-500"
                    : "bg-foreground/80",
              )}
              style={{ width: `${pct}%` }}
            />
          </div>
        </div>
      </header>

      <Tabs value={tab} onValueChange={setTab}>
        <TabsList>
          <TabsTrigger value="matrix">Matrix</TabsTrigger>
          <TabsTrigger value="cases">Case explorer</TabsTrigger>
          <TabsTrigger value="compare">Compare</TabsTrigger>
        </TabsList>
        <TabsContent value="matrix" className="space-y-4">
          <MatrixGridView
            grid={grid}
            selectedKey={selectedKey}
            onSelect={setSelected}
            labels={labels}
          />
          {selectedLive ? (
            <CellDetail
              cell={selectedLive}
              labels={labels}
              workspaceId={workspaceId}
              onClear={() => setSelected(null)}
            />
          ) : (
            <div className="rounded-xl border border-dashed border-border bg-muted/10 px-4 py-6 text-center text-sm text-muted-foreground">
              Select a matrix cell to see agent, pack, and linked runs.
            </div>
          )}
        </TabsContent>
        <TabsContent value="cases">
          <CaseExplorer evalSetId={evalSetId} labels={labels} />
        </TabsContent>
        <TabsContent value="compare" className="space-y-4">
          <div className="flex flex-wrap items-end gap-2">
            <div className="min-w-[16rem] flex-1">
              <label className="text-xs text-muted-foreground">
                Compare with eval set ID
              </label>
              <Input
                value={compareId}
                onChange={(e) => setCompareId(e.target.value)}
                placeholder="previous eval set uuid"
              />
            </div>
            <Button
              type="button"
              onClick={async () => {
                if (!accessToken || !compareId.trim()) return;
                try {
                  const api = createApiClient(accessToken);
                  const res = await compareEvalSets(
                    api,
                    evalSetId,
                    compareId.trim(),
                  );
                  setCompare(res);
                  setCompareError(null);
                } catch (err) {
                  setCompareError(
                    err instanceof Error ? err.message : "Compare failed",
                  );
                }
              }}
            >
              Compare
            </Button>
          </div>
          {compareError ? (
            <p className="text-sm text-destructive">{compareError}</p>
          ) : null}
          {compare ? (
            <div className="space-y-3 overflow-x-auto">
              <p className="text-sm text-muted-foreground">
                {compare.shared_keys} shared keys · {compare.regressions.length}{" "}
                regressions
              </p>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Matrix</TableHead>
                    <TableHead>A</TableHead>
                    <TableHead>B</TableHead>
                    <TableHead>Delta</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {compare.regressions.map((r) => (
                    <TableRow key={`${r.matrix_key}:${r.case_key}`}>
                      <TableCell className="font-mono text-xs">
                        {r.matrix_key}
                      </TableCell>
                      <TableCell>{r.a_score}</TableCell>
                      <TableCell className="text-red-600 dark:text-red-400">
                        {r.b_score}
                      </TableCell>
                      <TableCell className="text-red-600 dark:text-red-400">
                        {r.delta.toFixed(3)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              {compare.regressions.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  No score regressions.
                </p>
              ) : null}
            </div>
          ) : null}
        </TabsContent>
      </Tabs>
    </div>
  );
}

function CellDetail({
  cell,
  labels,
  workspaceId,
  onClear,
}: {
  cell: MatrixCell;
  labels: RefLabelMap;
  workspaceId: string;
  onClear: () => void;
}) {
  const agentName = displayRef(cell.agentRef, labels);
  const packName = displayRef(cell.packRef, labels);
  const progress =
    cell.totalCount > 0
      ? Math.round((cell.doneCount / cell.totalCount) * 100)
      : 0;

  return (
    <div className="rounded-xl border border-border bg-card/50 p-4 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-sm font-semibold tracking-tight">
              Cell detail
            </h2>
            <span
              className={cn(
                "text-xs font-medium",
                CELL_STATE_TONE[cell.state],
              )}
            >
              {matrixCellStateLabel(cell.state)}
            </span>
          </div>
          <dl className="grid gap-2 sm:grid-cols-2">
            <div className="flex items-start gap-2 rounded-lg bg-muted/30 px-3 py-2">
              <Bot className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
              <div className="min-w-0">
                <dt className="text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
                  Agent
                </dt>
                <dd className="truncate text-sm font-medium" title={cell.agentRef}>
                  {agentName}
                </dd>
              </div>
            </div>
            <div className="flex items-start gap-2 rounded-lg bg-muted/30 px-3 py-2">
              <Package className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
              <div className="min-w-0">
                <dt className="text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
                  Pack
                </dt>
                <dd className="truncate text-sm font-medium" title={cell.packRef}>
                  {packName}
                </dd>
              </div>
            </div>
          </dl>
        </div>
        <Button type="button" variant="ghost" size="sm" onClick={onClear}>
          Clear
        </Button>
      </div>

      <div className="mt-4 space-y-1.5">
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span>
            {cell.passCount} completed · {cell.doneCount}/{cell.totalCount}{" "}
            finished
          </span>
          <span className="tabular-nums">{progress}%</span>
        </div>
        <div className="h-1 overflow-hidden rounded-full bg-muted">
          <div
            className={cn(
              "h-full rounded-full transition-[width] duration-500",
              cell.state === "failed"
                ? "bg-red-500"
                : cell.state === "scored"
                  ? "bg-emerald-500"
                  : cell.state === "running"
                    ? "bg-amber-500"
                    : "bg-muted-foreground/40",
            )}
            style={{ width: `${progress}%` }}
          />
        </div>
      </div>

      <ul className="mt-4 divide-y divide-border/70 rounded-lg border border-border/80">
        {cell.combos.map((c) => (
          <li
            key={c.matrixKey}
            className="flex flex-wrap items-center justify-between gap-2 px-3 py-2.5 text-sm"
          >
            <div className="min-w-0">
              <p className="font-medium">
                Repeat {comboRepeatLabel(c.matrixKey)}
              </p>
              <p
                className="truncate font-mono text-[11px] text-muted-foreground"
                title={c.matrixKey}
              >
                {c.matrixKey}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <Badge variant="outline" className="capitalize">
                {c.status || "queued"}
              </Badge>
              {c.runId ? (
                <Link
                  href={`/workspaces/${workspaceId}/runs/${c.runId}`}
                  className="inline-flex items-center gap-1 text-xs font-medium underline-offset-4 hover:underline"
                >
                  Open run
                  <ArrowUpRight className="size-3" />
                </Link>
              ) : (
                <span className="text-xs text-muted-foreground">No run yet</span>
              )}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}

function Stat({
  label,
  value,
  hint,
  icon,
  accent,
}: {
  label: string;
  value: string;
  hint?: string;
  icon?: ReactNode;
  accent?: "amber";
}) {
  return (
    <div
      className={cn(
        "min-w-[6.5rem] rounded-xl border border-border bg-background/80 px-3 py-2",
        accent === "amber" && "border-amber-500/30 bg-amber-500/[0.06]",
      )}
    >
      <div className="flex items-center gap-1 text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
        {icon}
        {label}
      </div>
      <div className="mt-1 text-sm font-semibold tabular-nums">{value}</div>
      {hint ? (
        <div className="mt-0.5 text-[11px] tabular-nums text-muted-foreground">
          {hint}
        </div>
      ) : null}
    </div>
  );
}

function formatElapsed(startedAt: string, finishedAt?: string | null): string {
  const start = new Date(startedAt).getTime();
  const end = finishedAt ? new Date(finishedAt).getTime() : Date.now();
  const sec = Math.max(0, Math.round((end - start) / 1000));
  if (sec < 60) return `${sec}s`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ${sec % 60}s`;
  const hr = Math.floor(min / 60);
  return `${hr}h ${min % 60}m`;
}
