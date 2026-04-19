"use client";

import Link from "next/link";
import { History, PlayCircle } from "lucide-react";

import type { RegressionCase, RegressionSuite } from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";

import { CaseStatusBadge, SeverityBadge } from "../../../badges";
import { EditCaseDialog } from "./edit-case-dialog";

interface CaseDetailClientProps {
  workspaceId: string;
  suite: RegressionSuite;
  regressionCase: RegressionCase;
}

export function CaseDetailClient({
  workspaceId,
  suite,
  regressionCase: c,
}: CaseDetailClientProps) {
  const replayHref =
    c.source_run_id && c.source_run_agent_id
      ? `/workspaces/${workspaceId}/runs/${c.source_run_id}/agents/${c.source_run_agent_id}/replay`
      : null;

  return (
    <div className="space-y-6">
      <PageHeader
        title={c.title}
        breadcrumbs={[
          {
            label: "Regression Suites",
            href: `/workspaces/${workspaceId}/regression-suites`,
          },
          {
            label: suite.name,
            href: `/workspaces/${workspaceId}/regression-suites/${suite.id}`,
          },
          { label: c.title },
        ]}
        actions={
          <div className="flex items-center gap-2">
            {replayHref && (
              <Link
                href={replayHref}
                className={buttonVariants({
                  variant: "outline",
                  size: "sm",
                })}
              >
                <PlayCircle
                  data-icon="inline-start"
                  className="size-4"
                />
                Open Replay
              </Link>
            )}
            <EditCaseDialog
              workspaceId={workspaceId}
              regressionCase={c}
            />
          </div>
        }
      />

      <div className="rounded-lg border border-border bg-card/30 p-4 space-y-3">
        {c.description && (
          <p className="text-sm text-muted-foreground">{c.description}</p>
        )}
        <dl className="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-2 lg:grid-cols-4">
          <MetaRow label="Status">
            <CaseStatusBadge status={c.status} />
          </MetaRow>
          <MetaRow label="Severity">
            <SeverityBadge severity={c.severity} />
          </MetaRow>
          <MetaRow label="Promotion Mode">
            <Badge variant="outline">{c.promotion_mode}</Badge>
          </MetaRow>
          <MetaRow label="Evidence Tier">
            <Badge variant="outline">{c.evidence_tier}</Badge>
          </MetaRow>
        </dl>
      </div>

      <Section title="Provenance">
        <dl className="grid gap-x-6 gap-y-3 text-sm sm:grid-cols-2">
          <ProvenanceRow
            label="Source Run"
            value={c.source_run_id ?? null}
            href={
              c.source_run_id
                ? `/workspaces/${workspaceId}/runs/${c.source_run_id}`
                : null
            }
          />
          <ProvenanceRow
            label="Source Run Agent"
            value={c.source_run_agent_id ?? null}
            href={
              c.source_run_id && c.source_run_agent_id
                ? `/workspaces/${workspaceId}/runs/${c.source_run_id}/agents/${c.source_run_agent_id}/scorecard`
                : null
            }
          />
          <ProvenanceRow
            label="Challenge Pack Version"
            value={c.source_challenge_pack_version_id}
          />
          <ProvenanceRow
            label="Challenge Identity"
            value={c.source_challenge_identity_id}
          />
          <ProvenanceRow label="Case Key" value={c.source_case_key} />
          <ProvenanceRow
            label="Item Key"
            value={c.source_item_key ?? null}
          />
          <ProvenanceRow
            label="Input Set"
            value={c.source_challenge_input_set_id ?? null}
          />
          <ProvenanceRow
            label="Replay"
            value={c.source_replay_id ?? null}
          />
        </dl>
      </Section>

      <Section title="Promotion">
        <dl className="grid gap-x-6 gap-y-3 text-sm sm:grid-cols-2">
          <MetaRow label="Failure Class">
            <span className="font-[family-name:var(--font-mono)] text-xs">
              {c.failure_class}
            </span>
          </MetaRow>
          <MetaRow label="Promoted At">
            <span className="text-muted-foreground">
              {new Date(
                c.latest_promotion?.created_at ?? c.created_at,
              ).toLocaleString()}
            </span>
          </MetaRow>
          <MetaRow label="Promoted By">
            <span className="font-[family-name:var(--font-mono)] text-xs text-muted-foreground">
              {c.latest_promotion?.promoted_by_user_id ?? "\u2014"}
            </span>
          </MetaRow>
          <MetaRow label="Event Refs">
            <span className="text-muted-foreground">
              {c.latest_promotion?.source_event_refs?.length ?? 0}
            </span>
          </MetaRow>
        </dl>
        {c.failure_summary && (
          <p className="mt-3 text-sm text-muted-foreground whitespace-pre-wrap">
            {c.failure_summary}
          </p>
        )}
        {c.latest_promotion?.promotion_reason && (
          <div className="mt-3 rounded-md border border-border bg-background/60 p-3">
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground/80">
              Promotion Reason
            </p>
            <p className="mt-1 text-sm text-muted-foreground whitespace-pre-wrap">
              {c.latest_promotion.promotion_reason}
            </p>
          </div>
        )}
      </Section>

      <Section title="Payload Snapshot">
        <JsonViewer value={c.payload_snapshot} />
      </Section>

      <Section title="Expected Contract">
        <JsonViewer value={c.expected_contract} />
      </Section>

      {c.validator_overrides && (
        <Section title="Validator Overrides">
          <JsonViewer value={c.validator_overrides} />
        </Section>
      )}

      {Object.keys(c.metadata ?? {}).length > 0 && (
        <Section title="Metadata">
          <JsonViewer value={c.metadata} defaultOpen={false} />
        </Section>
      )}

      {c.latest_promotion && (
        <Section title="Promotion Snapshot">
          <JsonViewer
            value={c.latest_promotion.promotion_snapshot}
            defaultOpen={false}
          />
        </Section>
      )}

      <Section title="Recent Outcomes">
        <EmptyState
          icon={<History className="size-10" />}
          title="No recent outcomes"
          description="Run-history data will appear here once regression runs execute against this suite."
        />
      </Section>
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
    <section className="rounded-lg border border-border bg-card/30 p-4">
      <h2 className="mb-3 text-sm font-semibold tracking-tight">{title}</h2>
      {children}
    </section>
  );
}

function MetaRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-center gap-2">
      <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground/80">
        {label}
      </dt>
      <dd className="flex items-center">{children}</dd>
    </div>
  );
}

function ProvenanceRow({
  label,
  value,
  href,
}: {
  label: string;
  value: string | null;
  href?: string | null;
}) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground/80">
        {label}
      </dt>
      <dd className="font-[family-name:var(--font-mono)] text-xs">
        {value ? (
          href ? (
            <Link
              href={href}
              className="text-foreground hover:underline underline-offset-4 break-all"
            >
              {value}
            </Link>
          ) : (
            <span className="text-foreground break-all">{value}</span>
          )
        ) : (
          <span className="text-muted-foreground">{"\u2014"}</span>
        )}
      </dd>
    </div>
  );
}

function JsonViewer({
  value,
  defaultOpen = true,
}: {
  value: unknown;
  defaultOpen?: boolean;
}) {
  const formatted = JSON.stringify(value, null, 2);
  const isEmpty =
    formatted === "{}" || formatted === "[]" || formatted === "null";
  if (isEmpty) {
    return (
      <p className="text-xs text-muted-foreground">
        {"\u2014 empty"}
      </p>
    );
  }
  return (
    <details open={defaultOpen} className="group">
      <summary className="cursor-pointer select-none text-xs text-muted-foreground hover:text-foreground transition-colors">
        <span className="group-open:hidden">Show</span>
        <span className="hidden group-open:inline">Hide</span>
      </summary>
      <pre className="mt-2 max-h-96 overflow-auto rounded-md border border-border bg-background p-3 text-xs leading-relaxed font-[family-name:var(--font-mono)]">
        {formatted}
      </pre>
    </details>
  );
}
