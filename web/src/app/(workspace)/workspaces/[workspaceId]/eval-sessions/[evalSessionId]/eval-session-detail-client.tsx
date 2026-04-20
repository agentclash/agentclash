"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import {
  AlertTriangle,
  FlaskConical,
  GitCompare,
  Sigma,
} from "lucide-react";

import { createApiClient } from "@/lib/api/client";
import type {
  EvalSessionDetail,
  EvalSessionMetricAggregate,
  EvalSessionParticipantAggregate,
  EvalSessionStatus,
} from "@/lib/api/types";
import {
  deriveEvalSessionMode,
  deriveEvalSessionTitle,
  formatEvalSessionMetricName,
  formatEvalSessionRange,
  formatEvalSessionRate,
  formatEvalSessionValue,
  passMetricAggregateForEffectiveK,
  sortedAggregateDimensions,
} from "@/lib/eval-sessions";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { evalSessionStatusVariant, runStatusVariant } from "../../runs/status-variant";

const ACTIVE_STATUSES: EvalSessionStatus[] = ["queued", "running", "aggregating"];
const POLL_INTERVAL_MS = 5000;

function Section({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children: React.ReactNode;
}) {
  return (
    <section className="rounded-lg border border-border bg-card/40 p-4">
      <div className="mb-4">
        <h2 className="text-sm font-semibold tracking-tight">{title}</h2>
        {description ? (
          <p className="mt-1 text-sm text-muted-foreground">{description}</p>
        ) : null}
      </div>
      {children}
    </section>
  );
}

function StatCard({
  label,
  value,
  hint,
}: {
  label: string;
  value: string;
  hint?: string;
}) {
  return (
    <div className="rounded-lg border border-border bg-background/70 p-4">
      <div className="text-xs font-medium uppercase tracking-[0.12em] text-muted-foreground">
        {label}
      </div>
      <div className="mt-2 text-xl font-semibold tracking-tight">{value}</div>
      {hint ? <div className="mt-1 text-xs text-muted-foreground">{hint}</div> : null}
    </div>
  );
}

function MetricAggregateCard({
  label,
  aggregate,
}: {
  label: string;
  aggregate?: EvalSessionMetricAggregate | null;
}) {
  return (
    <div className="rounded-lg border border-border bg-background/70 p-4">
      <div className="text-sm font-medium">{label}</div>
      <div className="mt-2 text-2xl font-semibold tracking-tight">
        {formatEvalSessionValue(aggregate?.mean)}
      </div>
      <div className="mt-2 grid gap-1 text-xs text-muted-foreground">
        <div>Median: {formatEvalSessionValue(aggregate?.median)}</div>
        <div>Std dev: {formatEvalSessionValue(aggregate?.std_dev)}</div>
        <div>Range: {formatEvalSessionValue(aggregate?.min)} - {formatEvalSessionValue(aggregate?.max)}</div>
        <div>Interval: {formatEvalSessionRange(aggregate)}</div>
      </div>
    </div>
  );
}

function ParticipantCard({
  participant,
}: {
  participant: EvalSessionParticipantAggregate;
}) {
  const passAtAggregate = passMetricAggregateForEffectiveK(participant.pass_at_k);
  const passPowAggregate = passMetricAggregateForEffectiveK(participant.pass_pow_k);

  return (
    <div className="rounded-lg border border-border bg-background/70 p-4">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h3 className="text-sm font-semibold">{participant.label}</h3>
          <p className="mt-1 text-xs text-muted-foreground">
            Lane {participant.lane_index}
          </p>
        </div>
        {participant.metric_routing ? (
          <Badge variant="outline">
            {participant.metric_routing.primary_metric === "pass_pow_k"
              ? "Primary: pass^k"
              : "Primary: pass@k"}
          </Badge>
        ) : null}
      </div>

      <div className="mt-4 grid gap-3 md:grid-cols-3">
        <MetricAggregateCard label="Overall" aggregate={participant.overall} />
        <MetricAggregateCard label="pass@k" aggregate={passAtAggregate} />
        <MetricAggregateCard label="pass^k" aggregate={passPowAggregate} />
      </div>

      {participant.metric_routing ? (
        <div className="mt-4 rounded-lg border border-border bg-card/60 p-3 text-sm">
          <div className="font-medium">Metric routing</div>
          <div className="mt-2 text-muted-foreground">
            {participant.metric_routing.reasoning}
          </div>
          <div className="mt-3 grid gap-2 text-xs text-muted-foreground md:grid-cols-3">
            <div>
              Reliability weight:{" "}
              {formatEvalSessionRate(participant.metric_routing.reliability_weight)}
            </div>
            <div>Effective k: {participant.metric_routing.effective_k}</div>
            <div>
              Composite AgentScore:{" "}
              {formatEvalSessionRate(participant.metric_routing.composite_agent_score)}
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}

export function EvalSessionDetailClient({
  workspaceId,
  initialDetail,
}: {
  workspaceId: string;
  initialDetail: EvalSessionDetail;
}) {
  const { getAccessToken } = useAccessToken();
  const [detail, setDetail] = useState(initialDetail);

  const fetchDetail = useCallback(async () => {
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const nextDetail = await api.get<EvalSessionDetail>(
        `/v1/eval-sessions/${detail.eval_session.id}`,
      );
      setDetail(nextDetail);
    } catch {
      // Keep the current data on background refresh failures.
    }
  }, [detail.eval_session.id, getAccessToken]);

  const isActive = ACTIVE_STATUSES.includes(detail.eval_session.status);

  useEffect(() => {
    if (!isActive) return;
    const interval = setInterval(fetchDetail, POLL_INTERVAL_MS);
    return () => clearInterval(interval);
  }, [fetchDetail, isActive]);

  const aggregateResult = detail.aggregate_result;
  const title = deriveEvalSessionTitle(detail);
  const executionMode = deriveEvalSessionMode(detail.runs, aggregateResult);
  const passAtAggregate = passMetricAggregateForEffectiveK(aggregateResult?.pass_at_k);
  const passPowAggregate = passMetricAggregateForEffectiveK(aggregateResult?.pass_pow_k);
  const dimensions = sortedAggregateDimensions(aggregateResult);
  const taskProperties =
    detail.eval_session.routing_task_snapshot.task.task_properties;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3 text-sm text-muted-foreground">
        <Link
          href={`/workspaces/${workspaceId}/runs`}
          className="hover:text-foreground transition-colors"
        >
          Runs
        </Link>
        <span>/</span>
        <span className="text-foreground">{title}</span>
      </div>

      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="text-xl font-semibold tracking-tight">{title}</h1>
          <Badge
            variant={evalSessionStatusVariant[detail.eval_session.status] ?? "outline"}
          >
            {detail.eval_session.status}
          </Badge>
          {executionMode ? (
            <Badge variant="outline">
              {executionMode === "comparison" ? "Comparison Session" : "Single-Agent Session"}
            </Badge>
          ) : null}
        </div>

        <div className="grid gap-3 md:grid-cols-4">
          <StatCard
            label="Repetitions"
            value={String(detail.eval_session.repetitions)}
            hint="Independent child runs requested for this session"
          />
          <StatCard
            label="Child Runs"
            value={String(detail.summary.run_counts.total)}
            hint={`${detail.summary.run_counts.completed} completed · ${detail.summary.run_counts.failed} failed`}
          />
          <StatCard
            label="Effective k"
            value={
              aggregateResult?.metric_routing?.effective_k != null
                ? String(aggregateResult.metric_routing.effective_k)
                : "—"
            }
            hint="k used for pass@k / pass^k summaries"
          />
          <StatCard
            label="Primary Metric"
            value={
              aggregateResult?.metric_routing?.primary_metric === "pass_pow_k"
                ? "pass^k"
                : aggregateResult?.metric_routing?.primary_metric === "pass_at_k"
                  ? "pass@k"
                  : "—"
            }
            hint="Chosen by metric routing guidance"
          />
        </div>
      </div>

      {detail.evidence_warnings.length > 0 ? (
        <Section
          title="Evidence Warnings"
          description="Warnings are carried through from the session read model and aggregate evidence document."
        >
          <div className="space-y-2">
            {detail.evidence_warnings.map((warning) => (
              <div
                key={warning}
                className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-100"
              >
                <AlertTriangle className="mt-0.5 size-4 shrink-0" />
                <span>{warning}</span>
              </div>
            ))}
          </div>
        </Section>
      ) : null}

      <Section
        title="Configuration"
        description="The session config is the exact snapshot persisted with the repeated eval request."
      >
        <div className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2 text-sm">
            <div>
              <span className="text-muted-foreground">Aggregation method:</span>{" "}
              {detail.eval_session.aggregation_config.method}
            </div>
            <div>
              <span className="text-muted-foreground">Variance reporting:</span>{" "}
              {detail.eval_session.aggregation_config.report_variance ? "On" : "Off"}
            </div>
            <div>
              <span className="text-muted-foreground">Confidence interval:</span>{" "}
              {formatEvalSessionRate(detail.eval_session.aggregation_config.confidence_interval)}
            </div>
            <div>
              <span className="text-muted-foreground">Reliability weight override:</span>{" "}
              {formatEvalSessionRate(
                detail.eval_session.aggregation_config.reliability_weight,
              )}
            </div>
            <div>
              <span className="text-muted-foreground">Success threshold:</span>{" "}
              {formatEvalSessionRate(
                "min_pass_rate" in detail.eval_session.success_threshold_config
                  ? detail.eval_session.success_threshold_config.min_pass_rate
                  : undefined,
              )}
            </div>
          </div>

          <div className="space-y-2 text-sm">
            <div className="font-medium">Task properties</div>
            <div>
              <span className="text-muted-foreground">Side effects:</span>{" "}
              {taskProperties?.has_side_effects ? "Yes" : "No / unspecified"}
            </div>
            <div>
              <span className="text-muted-foreground">Autonomy:</span>{" "}
              {taskProperties?.autonomy ?? "Unspecified"}
            </div>
            <div>
              <span className="text-muted-foreground">Step count:</span>{" "}
              {taskProperties?.step_count ?? "Unspecified"}
            </div>
            <div>
              <span className="text-muted-foreground">Output type:</span>{" "}
              {taskProperties?.output_type ?? "Unspecified"}
            </div>
          </div>
        </div>
      </Section>

      <Section
        title="Aggregate Result"
        description="Session-level statistics built from the child run scorecards."
      >
        {aggregateResult ? (
          <div className="space-y-4">
            <div className="grid gap-3 md:grid-cols-3">
              <MetricAggregateCard label="Overall" aggregate={aggregateResult.overall} />
              <MetricAggregateCard label="pass@k" aggregate={passAtAggregate} />
              <MetricAggregateCard label="pass^k" aggregate={passPowAggregate} />
            </div>

            {aggregateResult.metric_routing ? (
              <div className="rounded-lg border border-border bg-card/60 p-4">
                <div className="flex items-center gap-2 text-sm font-medium">
                  <Sigma className="size-4" />
                  Metric Routing
                </div>
                <p className="mt-2 text-sm text-muted-foreground">
                  {aggregateResult.metric_routing.reasoning}
                </p>
                <div className="mt-3 grid gap-2 text-sm md:grid-cols-4">
                  <div>
                    <span className="text-muted-foreground">Source:</span>{" "}
                    {formatEvalSessionMetricName(aggregateResult.metric_routing.source)}
                  </div>
                  <div>
                    <span className="text-muted-foreground">Reliability weight:</span>{" "}
                    {formatEvalSessionRate(aggregateResult.metric_routing.reliability_weight)}
                  </div>
                  <div>
                    <span className="text-muted-foreground">Primary metric:</span>{" "}
                    {aggregateResult.metric_routing.primary_metric === "pass_pow_k"
                      ? "pass^k"
                      : "pass@k"}
                  </div>
                  <div>
                    <span className="text-muted-foreground">AgentScore:</span>{" "}
                    {formatEvalSessionRate(
                      aggregateResult.metric_routing.composite_agent_score,
                    )}
                  </div>
                </div>
              </div>
            ) : null}

            {aggregateResult.comparison ? (
              <div className="rounded-lg border border-border bg-card/60 p-4">
                <div className="flex items-center gap-2 text-sm font-medium">
                  <GitCompare className="size-4" />
                  Repeated-session comparison
                </div>
                <div className="mt-3 grid gap-2 text-sm md:grid-cols-4">
                  <div>
                    <span className="text-muted-foreground">Status:</span>{" "}
                    {formatEvalSessionMetricName(aggregateResult.comparison.status)}
                  </div>
                  <div>
                    <span className="text-muted-foreground">Reason:</span>{" "}
                    {aggregateResult.comparison.reason_code
                      ? formatEvalSessionMetricName(aggregateResult.comparison.reason_code)
                      : "—"}
                  </div>
                  <div>
                    <span className="text-muted-foreground">Compared metric:</span>{" "}
                    {aggregateResult.comparison.compared_metric === "pass_pow_k"
                      ? "pass^k"
                      : aggregateResult.comparison.compared_metric === "pass_at_k"
                        ? "pass@k"
                        : "—"}
                  </div>
                  <div>
                    <span className="text-muted-foreground">Winner:</span>{" "}
                    {aggregateResult.comparison.winner_label ?? "No clear winner"}
                  </div>
                </div>
              </div>
            ) : null}

            {dimensions.length > 0 ? (
              <div className="rounded-lg border border-border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Dimension</TableHead>
                      <TableHead>Mean</TableHead>
                      <TableHead>Median</TableHead>
                      <TableHead>Std Dev</TableHead>
                      <TableHead>Range</TableHead>
                      <TableHead>Interval</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {dimensions.map(([key, aggregate]) => (
                      <TableRow key={key}>
                        <TableCell className="font-medium">
                          {formatEvalSessionMetricName(key)}
                        </TableCell>
                        <TableCell>{formatEvalSessionValue(aggregate.mean)}</TableCell>
                        <TableCell>{formatEvalSessionValue(aggregate.median)}</TableCell>
                        <TableCell>{formatEvalSessionValue(aggregate.std_dev)}</TableCell>
                        <TableCell>
                          {formatEvalSessionValue(aggregate.min)} - {formatEvalSessionValue(aggregate.max)}
                        </TableCell>
                        <TableCell>{formatEvalSessionRange(aggregate)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            ) : null}

            {aggregateResult.participants?.length ? (
              <div className="space-y-3">
                <div className="text-sm font-medium">Participants</div>
                <div className="grid gap-3 xl:grid-cols-2">
                  {aggregateResult.participants.map((participant) => (
                    <ParticipantCard
                      key={`${participant.lane_index}-${participant.label}`}
                      participant={participant}
                    />
                  ))}
                </div>
              </div>
            ) : null}
          </div>
        ) : (
          <div className="flex items-start gap-3 rounded-lg border border-dashed border-border p-4 text-sm text-muted-foreground">
            <FlaskConical className="size-4 shrink-0" />
            <span>
              Aggregate output is not available yet. The session may still be running, aggregating, or waiting on persisted results.
            </span>
          </div>
        )}
      </Section>

      <Section
        title="Child Runs"
        description="Each repeated eval session fans out into regular child runs that still use the existing run detail, replay, and scorecard flows."
      >
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Mode</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {detail.runs.map((run) => (
                <TableRow key={run.id}>
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
                  <TableCell className="text-sm text-muted-foreground">
                    {run.execution_mode === "comparison" ? "Comparison" : "Single Agent"}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {new Date(run.created_at).toLocaleString()}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </Section>
    </div>
  );
}
