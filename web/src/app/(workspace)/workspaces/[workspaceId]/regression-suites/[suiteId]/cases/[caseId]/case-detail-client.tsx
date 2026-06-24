"use client";

import Link from "next/link";
import { ArrowUpRight, PlayCircle } from "lucide-react";

import type { RegressionCase, RegressionSuite } from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { PageHeader } from "@/components/ui/page-header";

import {
  CaseStatusBadge,
  MaintenanceBadge,
  SeverityBadge,
  ValidationBadge,
} from "../../../badges";
import { SuiteRunHistory } from "../../suite-run-history";
import { EditCaseDialog } from "./edit-case-dialog";

interface CaseDetailClientProps {
  workspaceId: string;
  suite: RegressionSuite;
  regressionCase: RegressionCase;
}

const curationLinkLabels = [
  ["candidate_run", "Candidate Run"],
  ["scorecard", "Scorecard"],
  ["replay", "Replay"],
  ["comparison", "Comparison"],
  ["release_gate", "Release Gate"],
] as const;

type CurationLink = {
  key: (typeof curationLinkLabels)[number][0];
  label: (typeof curationLinkLabels)[number][1];
  href: string;
};

const taxonomyRowLabels = [
  ["source", "Source"],
  ["failure_mode", "Failure Mode"],
  ["severity_hint", "Severity Hint"],
  ["gate_verdict", "Gate Verdict"],
  ["gate_reason_code", "Reason Code"],
  ["reason_code", "Reason Code"],
  ["triggered_condition", "Triggered Condition"],
  ["scorecard_dimension", "Scorecard Dimension"],
  ["review_failure_class", "Review Class"],
  ["review_failure_state", "Review State"],
] as const;

export function CaseDetailClient({
  workspaceId,
  suite,
  regressionCase: c,
}: CaseDetailClientProps) {
  const replayHref =
    c.source_run_id && c.source_run_agent_id
      ? `/workspaces/${workspaceId}/runs/${c.source_run_id}/agents/${c.source_run_agent_id}/replay`
      : null;
  const curationLinks = getCurationLinks(c.metadata);
  const taxonomyRows = getTaxonomyRows(c.metadata);
  const hasCurationMetadata =
    curationLinks.length > 0 || taxonomyRows.length > 0;

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

      <Section title="Validation">
        <dl className="grid gap-x-6 gap-y-3 text-sm sm:grid-cols-2">
          <MetaRow label="Status">
            <ValidationBadge status={c.validation.status} />
          </MetaRow>
          <MetaRow label="Maintenance">
            <MaintenanceBadge status={c.validation.maintenance_status} />
          </MetaRow>
          <MetaRow label="Scored Runs">
            <span className="text-muted-foreground">
              {c.validation.run_count} ({c.validation.failure_count} fail /{" "}
              {c.validation.pass_count} pass)
            </span>
          </MetaRow>
          <MetaRow label="Reproduction">
            <span className="text-muted-foreground">
              {c.validation.reproduction_rate === undefined
                ? "\u2014"
                : `${formatPercent(c.validation.reproduction_rate)} / ${formatPercent(c.validation.reproduction_threshold)}`}
            </span>
          </MetaRow>
          <MetaRow label="Last Outcome">
            <span className="text-muted-foreground">
              {c.validation.last_outcome ?? "\u2014"}
            </span>
          </MetaRow>
          <MetaRow label="Last Validated">
            <span className="text-muted-foreground">
              {c.validation.last_validated_at
                ? new Date(c.validation.last_validated_at).toLocaleString()
                : "\u2014"}
            </span>
          </MetaRow>
          <MetaRow label="Remaining Runs">
            <span className="text-muted-foreground">
              {c.validation.remaining_runs}
            </span>
          </MetaRow>
        </dl>
        <p className="mt-3 text-sm text-muted-foreground">
          {c.validation.recommended_action}
        </p>
        <p className="mt-1 text-sm text-muted-foreground">
          {c.validation.maintenance_action}
        </p>
      </Section>

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
            label="Eval Pack Version"
            value={c.source_eval_pack_version_id}
          />
          <ProvenanceRow
            label="Challenge Identity"
            value={c.source_challenge_identity_id}
          />
          {c.source_challenge_key && (
            <ProvenanceRow
              label="Challenge Key"
              value={c.source_challenge_key}
            />
          )}
          <ProvenanceRow label="Case Key" value={c.source_case_key} />
          <ProvenanceRow
            label="Item Key"
            value={c.source_item_key ?? null}
          />
          {c.source_failure_cluster_key && (
            <ProvenanceRow
              label="Failure Cluster"
              value={c.source_failure_cluster_key}
            />
          )}
          {c.source_failure_fingerprint && (
            <ProvenanceRow
              label="Failure Fingerprint"
              value={c.source_failure_fingerprint}
            />
          )}
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

      {hasCurationMetadata && (
        <Section title="CI Curation">
          {taxonomyRows.length > 0 && (
            <dl className="grid gap-x-6 gap-y-3 text-sm sm:grid-cols-2">
              {taxonomyRows.map((row) => (
                <MetaRow key={row.label} label={row.label}>
                  <span className="font-[family-name:var(--font-mono)] text-xs text-muted-foreground">
                    {row.value}
                  </span>
                </MetaRow>
              ))}
            </dl>
          )}
          {curationLinks.length > 0 && (
            <div className="mt-4 flex flex-wrap gap-2">
              {curationLinks.map((link) => (
                <Link
                  key={link.key}
                  href={link.href}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={buttonVariants({
                    variant: "outline",
                    size: "sm",
                  })}
                >
                  {link.label}
                  <ArrowUpRight data-icon="inline-end" className="size-3.5" />
                </Link>
              ))}
            </div>
          )}
        </Section>
      )}

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
        <p className="mb-2 text-xs text-muted-foreground">
          Runs that executed the parent suite. Per-matched-case outcomes
          are not exposed by the list-runs read model yet, so use the run
          link to drill into the scorecard.
        </p>
        <SuiteRunHistory
          workspaceId={workspaceId}
          suiteId={suite.id}
          emptyTitle="This case has not executed in the last 20 runs."
          emptyDescription="Once the parent suite runs in this workspace, the most recent outcomes will appear here."
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

function formatPercent(value: number): string {
  return `${Math.round(value * 100)}%`;
}

function getCurationLinks(metadata: Record<string, unknown>) {
  const links = readRecord(metadata.curation_links);
  if (!links) return [];

  const result: CurationLink[] = [];
  for (const [key, label] of curationLinkLabels) {
    const href = readExternalURL(links[key]);
    if (href) {
      result.push({ key, label, href });
    }
  }
  return result;
}

function getTaxonomyRows(metadata: Record<string, unknown>) {
  const taxonomy = readRecord(metadata.failure_taxonomy);
  if (!taxonomy) return [];

  const seenLabels = new Set<string>();
  const rows: Array<{ label: string; value: string }> = [];
  for (const [key, label] of taxonomyRowLabels) {
    if (seenLabels.has(label)) continue;
    const value = readString(taxonomy[key]);
    if (!value) continue;
    seenLabels.add(label);
    rows.push({ label, value });
  }
  return rows;
}

function readRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function readString(value: unknown): string | null {
  if (typeof value !== "string") return null;
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

function readExternalURL(value: unknown): string | null {
  const href = readString(value);
  if (!href) return null;

  try {
    const parsed = new URL(href);
    return parsed.protocol === "http:" || parsed.protocol === "https:"
      ? href
      : null;
  } catch {
    return null;
  }
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
