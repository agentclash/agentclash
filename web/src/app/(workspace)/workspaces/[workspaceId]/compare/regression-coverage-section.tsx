"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import {
  AlertTriangle,
  ArrowRightCircle,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Sparkles,
  XCircle,
} from "lucide-react";

import type {
  RunRegressionCoverage,
  RunRegressionCoverageCase,
  RunRegressionCoverageSuite,
} from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";

interface RegressionCoverageSectionProps {
  workspaceId: string;
  baselineCoverage?: RunRegressionCoverage;
  candidateCoverage?: RunRegressionCoverage;
}

interface CombinedSuiteRow {
  id: string;
  name: string;
  candidate?: RunRegressionCoverageSuite;
  baseline?: RunRegressionCoverageSuite;
  candidateUnmatched: RunRegressionCoverageCase[];
}

function warnCount(suite: RunRegressionCoverageSuite | undefined): number {
  if (!suite) return 0;
  return Math.max(0, suite.case_count - suite.pass_count - suite.fail_count);
}

export function RegressionCoverageSection({
  workspaceId,
  baselineCoverage,
  candidateCoverage,
}: RegressionCoverageSectionProps) {
  const rows = useMemo<CombinedSuiteRow[]>(() => {
    const byId = new Map<string, CombinedSuiteRow>();

    for (const suite of candidateCoverage?.suites ?? []) {
      byId.set(suite.id, {
        id: suite.id,
        name: suite.name,
        candidate: suite,
        candidateUnmatched: [],
      });
    }
    for (const suite of baselineCoverage?.suites ?? []) {
      const existing = byId.get(suite.id);
      if (existing) {
        existing.baseline = suite;
      } else {
        byId.set(suite.id, {
          id: suite.id,
          name: suite.name,
          baseline: suite,
          candidateUnmatched: [],
        });
      }
    }
    for (const uc of candidateCoverage?.unmatched_cases ?? []) {
      // Unmatched cases are not tied to a specific suite in the read model;
      // surface them in a synthetic "unassigned" row so the operator can
      // still see them rather than silently dropping the data.
      const key = "__unmatched__";
      const row = byId.get(key) ?? {
        id: key,
        name: "Unmatched cases",
        candidateUnmatched: [],
      };
      row.candidateUnmatched.push(uc);
      byId.set(key, row);
    }
    return Array.from(byId.values()).sort((a, b) => {
      if (a.id === "__unmatched__") return 1;
      if (b.id === "__unmatched__") return -1;
      return a.name.localeCompare(b.name);
    });
  }, [baselineCoverage, candidateCoverage]);

  const allSuiteIds = useMemo(
    () => rows.filter((r) => r.id !== "__unmatched__").map((r) => r.id),
    [rows],
  );

  const [selected, setSelected] = useState<string[]>([]);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const visibleRows = useMemo(() => {
    if (selected.length === 0) return rows;
    const set = new Set(selected);
    return rows.filter(
      (r) => r.id === "__unmatched__" || set.has(r.id),
    );
  }, [rows, selected]);

  const totalCandidateFails = useMemo(
    () =>
      rows.reduce(
        (acc, r) => acc + (r.candidate?.fail_count ?? 0),
        0,
      ),
    [rows],
  );
  const totalBaselineFails = useMemo(
    () =>
      rows.reduce(
        (acc, r) => acc + (r.baseline?.fail_count ?? 0),
        0,
      ),
    [rows],
  );
  const netFailDelta = totalCandidateFails - totalBaselineFails;

  const hasAnyCoverage =
    (candidateCoverage?.suites?.length ?? 0) > 0 ||
    (baselineCoverage?.suites?.length ?? 0) > 0 ||
    (candidateCoverage?.unmatched_cases?.length ?? 0) > 0;

  if (!hasAnyCoverage) return null;

  function toggleSuite(id: string) {
    setSelected((prev) =>
      prev.includes(id) ? prev.filter((s) => s !== id) : [...prev, id],
    );
  }

  return (
    <div>
      <div className="mb-3 flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold">Regression Coverage</h2>
        <span className="text-xs text-muted-foreground">
          Candidate: {totalCandidateFails} failure
          {totalCandidateFails === 1 ? "" : "s"}
          {netFailDelta !== 0 && (
            <>
              {" "}
              (
              <span
                className={cn(
                  netFailDelta > 0 ? "text-red-400" : "text-emerald-400",
                )}
              >
                {netFailDelta > 0 ? "+" : ""}
                {netFailDelta} vs baseline
              </span>
              )
            </>
          )}
        </span>
      </div>

      {allSuiteIds.length > 1 && (
        <div className="mb-3 flex flex-wrap items-center gap-1.5">
          <span className="text-xs text-muted-foreground">Filter:</span>
          {rows
            .filter((r) => r.id !== "__unmatched__")
            .map((r) => {
              const active = selected.includes(r.id);
              return (
                <button
                  key={r.id}
                  type="button"
                  onClick={() => toggleSuite(r.id)}
                  className={cn(
                    "rounded-full border px-2 py-0.5 text-xs transition-colors",
                    active
                      ? "border-foreground bg-foreground text-background"
                      : "border-border text-muted-foreground hover:border-foreground/40 hover:text-foreground",
                  )}
                >
                  {r.name}
                </button>
              );
            })}
          {selected.length > 0 && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-6 px-2 text-xs"
              onClick={() => setSelected([])}
            >
              Clear
            </Button>
          )}
        </div>
      )}

      <div className="rounded-lg border border-border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-8" />
              <TableHead>Suite</TableHead>
              <TableHead className="text-right">Cases</TableHead>
              <TableHead className="text-right">Pass</TableHead>
              <TableHead className="text-right">Fail</TableHead>
              <TableHead className="text-right">Warn</TableHead>
              <TableHead className="text-right">Δ vs baseline</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {visibleRows.map((row) => (
              <SuiteRow
                key={row.id}
                row={row}
                expanded={expandedId === row.id}
                onToggleExpand={() =>
                  setExpandedId((prev) => (prev === row.id ? null : row.id))
                }
                workspaceId={workspaceId}
              />
            ))}
            {visibleRows.length === 0 && (
              <TableRow>
                <TableCell
                  colSpan={8}
                  className="py-8 text-center text-sm text-muted-foreground"
                >
                  No suites match the current filter.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}

function SuiteRow({
  row,
  expanded,
  onToggleExpand,
  workspaceId,
}: {
  row: CombinedSuiteRow;
  expanded: boolean;
  onToggleExpand: () => void;
  workspaceId: string;
}) {
  const isUnmatched = row.id === "__unmatched__";
  const candidate = row.candidate;
  const baseline = row.baseline;
  const cDelta =
    (candidate?.fail_count ?? 0) - (baseline?.fail_count ?? 0);
  const newFailures = cDelta > 0;
  const baselineOnly = !candidate && !!baseline;
  const candidateOnly = !baseline && !!candidate && !isUnmatched;

  const ExpandIcon = expanded ? ChevronDown : ChevronRight;

  return (
    <>
      <TableRow
        className={cn(
          newFailures && !isUnmatched && "bg-red-500/5",
          baselineOnly && "text-muted-foreground",
        )}
      >
        <TableCell className="p-2 align-middle">
          <button
            type="button"
            onClick={onToggleExpand}
            className="text-muted-foreground hover:text-foreground transition-colors"
            aria-label={expanded ? "Collapse" : "Expand"}
          >
            <ExpandIcon className="size-4" />
          </button>
        </TableCell>
        <TableCell className="align-middle">
          <div className="flex items-center gap-2">
            <span className="font-medium">{row.name}</span>
            {newFailures && (
              <Badge variant="destructive">
                <AlertTriangle
                  data-icon="inline-start"
                  className="size-3"
                />
                New failures
              </Badge>
            )}
            {candidateOnly && (
              <Badge variant="outline">
                <Sparkles data-icon="inline-start" className="size-3" />
                New
              </Badge>
            )}
            {baselineOnly && (
              <Badge variant="outline">Baseline only</Badge>
            )}
          </div>
        </TableCell>
        <TableCell className="text-right font-[family-name:var(--font-mono)] text-sm">
          {candidate?.case_count ??
            row.candidateUnmatched.length ??
            baseline?.case_count ??
            0}
        </TableCell>
        <TableCell className="text-right">
          <CountCell value={candidate?.pass_count ?? 0} tone="pass" />
        </TableCell>
        <TableCell className="text-right">
          <CountCell value={candidate?.fail_count ?? 0} tone="fail" />
        </TableCell>
        <TableCell className="text-right">
          <CountCell value={warnCount(candidate)} tone="warn" />
        </TableCell>
        <TableCell className="text-right font-[family-name:var(--font-mono)] text-sm">
          {isUnmatched ? (
            <span className="text-muted-foreground">—</span>
          ) : cDelta > 0 ? (
            <span className="text-red-400">+{cDelta}</span>
          ) : cDelta < 0 ? (
            <span className="text-emerald-400">{cDelta}</span>
          ) : (
            <span className="text-muted-foreground">0</span>
          )}
        </TableCell>
        <TableCell className="text-right">
          {!isUnmatched && (
            <Link
              href={`/workspaces/${workspaceId}/regression-suites/${row.id}`}
              className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              Open suite
              <ArrowRightCircle className="size-3" />
            </Link>
          )}
        </TableCell>
      </TableRow>

      {expanded && (
        <TableRow className="bg-muted/20">
          <TableCell />
          <TableCell colSpan={7} className="pt-2 pb-4">
            <SuiteExpandedDetail
              row={row}
              workspaceId={workspaceId}
            />
          </TableCell>
        </TableRow>
      )}
    </>
  );
}

function CountCell({
  value,
  tone,
}: {
  value: number;
  tone: "pass" | "fail" | "warn";
}) {
  const color =
    value === 0
      ? "text-muted-foreground"
      : tone === "pass"
        ? "text-emerald-400"
        : tone === "fail"
          ? "text-red-400"
          : "text-amber-400";
  const Icon = tone === "pass" ? CheckCircle2 : tone === "fail" ? XCircle : AlertTriangle;
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 font-[family-name:var(--font-mono)] text-sm",
        color,
      )}
    >
      {value > 0 && <Icon className="size-3" />}
      {value}
    </span>
  );
}

function SuiteExpandedDetail({
  row,
  workspaceId,
}: {
  row: CombinedSuiteRow;
  workspaceId: string;
}) {
  const candidate = row.candidate;
  const baseline = row.baseline;

  if (row.id === "__unmatched__") {
    return (
      <div className="space-y-2">
        <p className="text-xs text-muted-foreground">
          These regression cases were selected for the candidate run but did
          not match any executed item. This usually means the underlying
          challenge identity was skipped or filtered out.
        </p>
        <ul className="space-y-1 text-sm">
          {row.candidateUnmatched.map((c) => (
            <li
              key={c.id}
              className="flex items-center justify-between gap-2"
            >
              <span>{c.title}</span>
              <Badge variant="outline">{c.outcome}</Badge>
            </li>
          ))}
        </ul>
      </div>
    );
  }

  return (
    <div className="grid gap-3 text-xs sm:grid-cols-2">
      <CountsColumn
        label="Candidate"
        suite={candidate}
        link={
          candidate
            ? `/workspaces/${workspaceId}/regression-suites/${row.id}`
            : null
        }
      />
      <CountsColumn label="Baseline" suite={baseline} link={null} />
    </div>
  );
}

function CountsColumn({
  label,
  suite,
  link,
}: {
  label: string;
  suite: RunRegressionCoverageSuite | undefined;
  link: string | null;
}) {
  return (
    <div className="rounded-md border border-border bg-background/60 p-3">
      <p className="mb-1 text-xs font-medium uppercase tracking-wide text-muted-foreground/80">
        {label}
      </p>
      {suite ? (
        <dl className="grid grid-cols-4 gap-2 text-center text-xs">
          <Stat label="Cases" value={suite.case_count} />
          <Stat label="Pass" value={suite.pass_count} tone="pass" />
          <Stat label="Fail" value={suite.fail_count} tone="fail" />
          <Stat label="Warn" value={warnCount(suite)} tone="warn" />
        </dl>
      ) : (
        <p className="text-xs text-muted-foreground">
          Not included in this run.
        </p>
      )}
      {link && (
        <Link
          href={link}
          className="mt-2 inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          View suite
          <ArrowRightCircle className="size-3" />
        </Link>
      )}
    </div>
  );
}

function Stat({
  label,
  value,
  tone,
}: {
  label: string;
  value: number;
  tone?: "pass" | "fail" | "warn";
}) {
  const color =
    value === 0 || !tone
      ? "text-foreground"
      : tone === "pass"
        ? "text-emerald-400"
        : tone === "fail"
          ? "text-red-400"
          : "text-amber-400";
  return (
    <div>
      <dt className="text-2xs uppercase tracking-wide text-muted-foreground/80">
        {label}
      </dt>
      <dd
        className={cn(
          "font-[family-name:var(--font-mono)] text-sm font-medium",
          color,
        )}
      >
        {value}
      </dd>
    </div>
  );
}
