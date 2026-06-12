"use client";

import Link from "next/link";
import { ArrowUpRight, Play } from "lucide-react";
import {
  Sheet,
  SheetContent,
  SheetTitle,
} from "@/components/ui/sheet";
import { Badge } from "@/components/ui/badge";
import type { FailureReviewItem } from "@/lib/api/types";
import { cn } from "@/lib/utils";

interface FailureDetailDrawerProps {
  item: FailureReviewItem | null;
  workspaceId: string;
  onClose: () => void;
}

function humanize(value: string): string {
  return value.replace(/_/g, " ");
}

export function FailureDetailDrawer({
  item,
  workspaceId,
  onClose,
}: FailureDetailDrawerProps) {
  const open = item != null;

  return (
    <Sheet
      open={open}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
    >
      <SheetContent
        side="right"
        className="w-full sm:max-w-xl p-0 flex flex-col gap-0"
      >
        <SheetTitle className="sr-only">
          {item ? `Failure: ${item.case_key}` : "Failure detail"}
        </SheetTitle>
        {item && <FailureDetailBody item={item} workspaceId={workspaceId} />}
      </SheetContent>
    </Sheet>
  );
}

function FailureDetailBody({
  item,
  workspaceId,
}: {
  item: FailureReviewItem;
  workspaceId: string;
}) {
  const firstReplay = item.replay_step_refs[0];
  const replayHref = firstReplay
    ? `/workspaces/${workspaceId}/runs/${item.run_id}/agents/${item.run_agent_id}/replay?step=${firstReplay.sequence_number}`
    : undefined;

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Header */}
      <div className="border-b border-border px-6 pt-6 pb-4">
        <div className="flex items-center gap-2 text-2xs uppercase tracking-[0.18em] text-muted-foreground mb-2">
          <span>Failure review</span>
          <span className="text-muted-foreground/50">·</span>
          <span className="normal-case tracking-normal font-[family-name:var(--font-mono)] text-xs">
            {item.challenge_key}
          </span>
        </div>
        <h2 className="text-base font-medium text-foreground tracking-tight mb-1">
          {item.case_key}
        </h2>
        {item.headline && (
          <p className="text-sm text-muted-foreground leading-relaxed">
            {item.headline}
          </p>
        )}

        <div className="flex flex-wrap items-center gap-1.5 mt-3">
          <Badge variant="destructive">{humanize(item.failure_state)}</Badge>
          <Badge variant="outline">{humanize(item.failure_class)}</Badge>
          <Badge variant="secondary">{item.severity}</Badge>
          <Badge variant="secondary">{humanize(item.evidence_tier)}</Badge>
          {item.promotable && (
            <Badge
              variant="outline"
              className="border-emerald-500/40 text-emerald-400"
            >
              Promotable
            </Badge>
          )}
        </div>
      </div>

      {/* Scrollable body */}
      <div className="flex-1 overflow-y-auto px-6 py-5 space-y-6">
        {replayHref && (
          <Link
            href={replayHref}
            className="group flex items-center justify-between gap-3 rounded-lg border border-border px-4 py-3 hover:border-foreground/25 hover:bg-muted/30 transition-colors"
          >
            <div className="flex items-center gap-3 min-w-0">
              <Play className="size-4 text-foreground/70 shrink-0" />
              <div className="min-w-0">
                <div className="text-2xs uppercase tracking-[0.18em] text-muted-foreground">
                  Open replay
                </div>
                <div className="text-sm text-foreground font-[family-name:var(--font-mono)] truncate">
                  step #{firstReplay?.sequence_number} ·{" "}
                  {firstReplay?.event_type}
                </div>
              </div>
            </div>
            <ArrowUpRight className="size-4 text-muted-foreground group-hover:text-foreground" />
          </Link>
        )}

        {item.detail && (
          <Section title="Detail">
            <p className="text-sm text-foreground/90 leading-relaxed whitespace-pre-wrap">
              {item.detail}
            </p>
          </Section>
        )}

        <Section title="Likely issue area">
          <div className="space-y-3">
            <div className="flex items-center gap-2 flex-wrap">
              <Badge variant="outline">{item.remediation.label}</Badge>
              <span className="text-xs text-muted-foreground font-[family-name:var(--font-mono)]">
                {item.remediation.area}
              </span>
            </div>
            <p className="text-sm text-foreground/90 leading-relaxed">
              {item.remediation.summary}
            </p>
            {item.recommended_action && (
              <div className="rounded-md border border-border bg-background/60 p-3">
                <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground/80">
                  Recommended action
                </p>
                <p className="mt-1 text-sm text-foreground/90 leading-relaxed whitespace-pre-wrap">
                  {item.recommended_action}
                </p>
              </div>
            )}
            {item.remediation.evidence.length > 0 && (
              <ul className="space-y-1.5">
                {item.remediation.evidence.map((evidence, i) => (
                  <li
                    key={i}
                    className="text-xs text-muted-foreground leading-snug"
                  >
                    {evidence}
                  </li>
                ))}
              </ul>
            )}
          </div>
        </Section>

        {item.failed_dimensions.length > 0 && (
          <Section title="Failed dimensions">
            <div className="flex flex-wrap gap-1.5">
              {item.failed_dimensions.map((d) => (
                <Badge
                  key={d}
                  variant="outline"
                  className="font-[family-name:var(--font-mono)]"
                >
                  {d}
                </Badge>
              ))}
            </div>
          </Section>
        )}

        {item.failed_checks.length > 0 && (
          <Section title="Failed checks">
            <ul className="space-y-1.5">
              {item.failed_checks.map((c, i) => (
                <li
                  key={i}
                  className="font-[family-name:var(--font-mono)] text-xs text-foreground/85 bg-muted/40 border border-border rounded px-2 py-1.5"
                >
                  {c}
                </li>
              ))}
            </ul>
          </Section>
        )}

        {item.judge_refs.length > 0 && (
          <Section title="Judge evidence">
            <div className="divide-y divide-border border border-border rounded-lg overflow-hidden">
              {item.judge_refs.map((j, i) => (
                <div
                  key={`${j.key}-${i}`}
                  className="px-3 py-2.5 flex items-start gap-3"
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-[family-name:var(--font-mono)] text-xs text-foreground/90 truncate">
                        {j.key}
                      </span>
                      <Badge variant="secondary">{j.kind}</Badge>
                      {j.verdict && (
                        <Badge
                          variant="outline"
                          className={cn(
                            j.verdict === "fail" && "text-destructive",
                          )}
                        >
                          {j.verdict}
                        </Badge>
                      )}
                    </div>
                    {j.reason && (
                      <p className="text-xs text-muted-foreground mt-1 leading-snug">
                        {j.reason}
                      </p>
                    )}
                  </div>
                  {j.normalized_score != null && (
                    <span className="font-[family-name:var(--font-mono)] text-xs tabular-nums text-foreground/75">
                      {(j.normalized_score * 100).toFixed(1)}
                    </span>
                  )}
                </div>
              ))}
            </div>
          </Section>
        )}

        {item.metric_refs.length > 0 && (
          <Section title="Metrics">
            <div className="divide-y divide-border border border-border rounded-lg overflow-hidden">
              {item.metric_refs.map((m, i) => (
                <div
                  key={`${m.key}-${i}`}
                  className="px-3 py-2 flex items-center gap-3"
                >
                  <span className="font-[family-name:var(--font-mono)] text-xs text-foreground/90 flex-1 truncate">
                    {m.key}
                  </span>
                  <Badge variant="secondary">{m.metric_type}</Badge>
                  <span className="font-[family-name:var(--font-mono)] text-xs tabular-nums text-muted-foreground min-w-16 text-right">
                    {formatMetric(m)}
                  </span>
                </div>
              ))}
            </div>
          </Section>
        )}

        {item.replay_step_refs.length > 1 && (
          <Section title="Other replay steps">
            <div className="space-y-1.5">
              {item.replay_step_refs.slice(1).map((r, i) => (
                <Link
                  key={i}
                  href={`/workspaces/${workspaceId}/runs/${item.run_id}/agents/${item.run_agent_id}/replay?step=${r.sequence_number}`}
                  className="flex items-center justify-between gap-3 rounded-md border border-border px-3 py-2 hover:bg-muted/30 transition-colors text-xs"
                >
                  <span className="font-[family-name:var(--font-mono)] text-foreground/85 truncate">
                    #{r.sequence_number} · {r.event_type}
                  </span>
                  <ArrowUpRight className="size-3.5 text-muted-foreground" />
                </Link>
              ))}
            </div>
          </Section>
        )}

        {item.artifact_refs.length > 0 && (
          <Section title="Artifacts">
            <ul className="space-y-1">
              {item.artifact_refs.map((a, i) => (
                <li
                  key={`${a.key}-${i}`}
                  className="text-xs font-[family-name:var(--font-mono)] text-foreground/80 truncate"
                >
                  {a.key}
                  {a.media_type && (
                    <span className="text-muted-foreground ml-1.5">
                      · {a.media_type}
                    </span>
                  )}
                </li>
              ))}
            </ul>
          </Section>
        )}
      </div>
    </div>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <h3 className="text-2xs uppercase tracking-[0.2em] text-muted-foreground mb-2 font-medium">
        {title}
      </h3>
      {children}
    </div>
  );
}

function formatMetric(m: {
  numeric_value?: number;
  boolean_value?: boolean;
  text_value?: string;
  unit?: string;
}): string {
  if (m.numeric_value != null) {
    const v = m.numeric_value.toLocaleString(undefined, {
      maximumFractionDigits: 4,
    });
    return m.unit ? `${v} ${m.unit}` : v;
  }
  if (m.boolean_value != null) return m.boolean_value ? "true" : "false";
  if (m.text_value != null) return m.text_value;
  return "—";
}
