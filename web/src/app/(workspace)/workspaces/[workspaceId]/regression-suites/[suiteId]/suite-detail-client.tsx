"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { History, ListChecks } from "lucide-react";

import type {
  ChallengePack,
  RegressionCase,
  RegressionCaseStatus,
  RegressionSeverity,
  RegressionSuite,
} from "@/lib/api/types";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

import {
  CaseStatusBadge,
  SeverityBadge,
  SuiteStatusBadge,
} from "../badges";
import { EditSuiteDialog } from "./edit-suite-dialog";
import { SuiteRunHistory } from "./suite-run-history";

type CaseStatusFilter = "all" | RegressionCaseStatus;
type SeverityFilter = "all" | RegressionSeverity;

interface SuiteDetailClientProps {
  workspaceId: string;
  suite: RegressionSuite;
  cases: RegressionCase[];
  sourcePack: ChallengePack | null;
}

export function SuiteDetailClient({
  workspaceId,
  suite,
  cases,
  sourcePack,
}: SuiteDetailClientProps) {
  const [statusFilter, setStatusFilter] = useState<CaseStatusFilter>("all");
  const [severityFilter, setSeverityFilter] = useState<SeverityFilter>("all");

  const filteredCases = useMemo(() => {
    return cases.filter((c) => {
      if (statusFilter !== "all" && c.status !== statusFilter) return false;
      if (severityFilter !== "all" && c.severity !== severityFilter) return false;
      return true;
    });
  }, [cases, statusFilter, severityFilter]);

  const packLabel = sourcePack ? sourcePack.name : suite.source_challenge_pack_id;
  const packHref = sourcePack
    ? `/workspaces/${workspaceId}/challenge-packs/${sourcePack.id}`
    : null;

  return (
    <div className="space-y-6">
      <PageHeader
        title={suite.name}
        breadcrumbs={[
          {
            label: "Regression Suites",
            href: `/workspaces/${workspaceId}/regression-suites`,
          },
          { label: suite.name },
        ]}
        actions={<EditSuiteDialog workspaceId={workspaceId} suite={suite} />}
      />

      <div className="rounded-lg border border-border bg-card/30 p-4 space-y-3">
        {suite.description && (
          <p className="text-sm text-muted-foreground">
            {suite.description}
          </p>
        )}
        <dl className="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-2 lg:grid-cols-4">
          <MetaRow label="Status">
            <SuiteStatusBadge status={suite.status} />
          </MetaRow>
          <MetaRow label="Default Severity">
            <SeverityBadge severity={suite.default_gate_severity} />
          </MetaRow>
          <MetaRow label="Source Pack">
            {packHref ? (
              <Link
                href={packHref}
                className="text-foreground hover:underline underline-offset-4"
              >
                {packLabel}
              </Link>
            ) : (
              <span
                className="text-muted-foreground"
                title={suite.source_challenge_pack_id}
              >
                {packLabel}
              </span>
            )}
          </MetaRow>
          <MetaRow label="Cases">
            <span className="text-foreground">{cases.length}</span>
          </MetaRow>
        </dl>
      </div>

      <Tabs defaultValue="cases" className="w-full">
        <TabsList>
          <TabsTrigger value="cases">
            <ListChecks className="size-4" />
            Cases
          </TabsTrigger>
          <TabsTrigger value="history">
            <History className="size-4" />
            Run History
          </TabsTrigger>
        </TabsList>

        <TabsContent value="cases" className="pt-4 space-y-4">
          {cases.length === 0 ? (
            <EmptyState
              icon={<ListChecks className="size-10" />}
              title="No regression cases yet"
              description="Promote a failure from a run to get started."
              action={undefined}
            />
          ) : (
            <>
              <div className="flex flex-wrap items-center gap-2">
                <FilterSelect
                  label="Status"
                  value={statusFilter}
                  onChange={(v) => setStatusFilter(v as CaseStatusFilter)}
                  options={[
                    { value: "all", label: "All" },
                    { value: "active", label: "Active" },
                    { value: "muted", label: "Muted" },
                    { value: "archived", label: "Archived" },
                  ]}
                />
                <FilterSelect
                  label="Severity"
                  value={severityFilter}
                  onChange={(v) => setSeverityFilter(v as SeverityFilter)}
                  options={[
                    { value: "all", label: "All" },
                    { value: "info", label: "Info" },
                    { value: "warning", label: "Warning" },
                    { value: "blocking", label: "Blocking" },
                  ]}
                />
                <span className="ml-auto text-xs text-muted-foreground">
                  {filteredCases.length} of {cases.length}
                </span>
              </div>

              <div className="rounded-lg border border-border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Title</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Severity</TableHead>
                      <TableHead>Failure Class</TableHead>
                      <TableHead>Created</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filteredCases.length === 0 ? (
                      <TableRow>
                        <TableCell
                          colSpan={5}
                          className="py-8 text-center text-sm text-muted-foreground"
                        >
                          No cases match the current filters.
                        </TableCell>
                      </TableRow>
                    ) : (
                      filteredCases.map((c) => (
                        <TableRow key={c.id}>
                          <TableCell>
                            <Link
                              href={`/workspaces/${workspaceId}/regression-suites/${suite.id}/cases/${c.id}`}
                              className="font-medium text-foreground hover:underline underline-offset-4"
                            >
                              {c.title}
                            </Link>
                            {c.description && (
                              <p className="mt-0.5 max-w-md truncate text-xs text-muted-foreground">
                                {c.description}
                              </p>
                            )}
                          </TableCell>
                          <TableCell>
                            <CaseStatusBadge status={c.status} />
                          </TableCell>
                          <TableCell>
                            <SeverityBadge severity={c.severity} />
                          </TableCell>
                          <TableCell className="text-sm text-muted-foreground">
                            {c.failure_class}
                          </TableCell>
                          <TableCell className="text-sm text-muted-foreground">
                            {new Date(c.created_at).toLocaleDateString()}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </div>
            </>
          )}

          <p className="text-xs text-muted-foreground">
            Need to add a case? Promote a failing run from{" "}
            <Link
              href={`/workspaces/${workspaceId}/runs`}
              className="underline underline-offset-4 hover:text-foreground"
            >
              Runs
            </Link>
            .
          </p>
        </TabsContent>

        <TabsContent value="history" className="pt-4">
          <SuiteRunHistory workspaceId={workspaceId} suiteId={suite.id} />
        </TabsContent>
      </Tabs>
    </div>
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

function FilterSelect<T extends string>({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: T;
  onChange: (v: T) => void;
  options: { value: T; label: string }[];
}) {
  return (
    <div className="flex items-center gap-1.5">
      <span className="text-xs text-muted-foreground">{label}</span>
      <Select value={value} onValueChange={(v) => v && onChange(v as T)}>
        <SelectTrigger size="sm" className="min-w-[7rem]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((o) => (
            <SelectItem key={o.value} value={o.value}>
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
