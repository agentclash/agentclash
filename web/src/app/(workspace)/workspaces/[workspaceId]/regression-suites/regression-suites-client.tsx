"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Check, MoreHorizontal, ShieldAlert, XCircle } from "lucide-react";

import { createApiClient } from "@/lib/api/client";
import { useApiListQuery, useApiMutator, usePaginatedApiQuery } from "@/lib/api/swr";
import { ApiError } from "@/lib/api/errors";
import type {
  ChallengePack,
  PatchRegressionCaseInput,
  PatchRegressionSuiteInput,
  RegressionCase,
  RegressionSuite,
  RegressionSuiteStatus,
} from "@/lib/api/types";
import { workspacePageSizes, workspaceResourceKeys } from "@/lib/workspace-resource";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { Button } from "@/components/ui/button";
import { ConfirmProvider, useConfirm } from "@/components/ui/confirm-dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import { PaginationControls } from "@/components/ui/pagination-controls";
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

import {
  CaseStatusBadge,
  MaintenanceBadge,
  SeverityBadge,
  SuiteStatusBadge,
  ValidationBadge,
} from "./badges";
import { CreateSuiteDialog } from "./create-suite-dialog";

const PAGE_SIZE = workspacePageSizes.suites;

type StatusFilter = "all" | RegressionSuiteStatus;

export function RegressionSuitesClient({ workspaceId }: { workspaceId: string }) {
  return (
    <ConfirmProvider>
      <RegressionSuitesInner workspaceId={workspaceId} />
    </ConfirmProvider>
  );
}

function ProposedCaseValidationSummary({
  regressionCase,
}: {
  regressionCase: RegressionCase;
}) {
  const validation = regressionCase.validation;
  const detail =
    validation.reproduction_rate === undefined
      ? `${validation.run_count}/${validation.required_runs} runs`
      : `${formatPercent(validation.reproduction_rate)} repro`;

  return (
    <div className="flex flex-col items-start gap-1">
      <div className="flex flex-wrap gap-1">
        <ValidationBadge status={validation.status} />
        <MaintenanceBadge status={validation.maintenance_status} />
      </div>
      <span className="text-xs text-muted-foreground">{detail}</span>
    </div>
  );
}

function formatPercent(value: number): string {
  return `${Math.round(value * 100)}%`;
}

function shortID(value: string): string {
  return value.slice(0, 8);
}

function RegressionSuitesInner({
  workspaceId,
}: {
  workspaceId: string;
}) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { getAccessToken } = useAccessToken();
  const { mutate } = useApiMutator();
  const confirm = useConfirm();
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [pending, setPending] = useState<string | null>(null);
  const offset = Math.max(
    0,
    Number.parseInt(searchParams.get("offset") ?? "0", 10) || 0,
  );
  const caseOffset = Math.max(
    0,
    Number.parseInt(searchParams.get("caseOffset") ?? "0", 10) || 0,
  );
  const initialCreateOpen = searchParams.get("create") === "1";
  const initialCreatePackId = searchParams.get("sourcePackId") ?? undefined;
  const {
    data: suitesPage,
    error: suitesError,
    isLoading: suitesLoading,
  } = usePaginatedApiQuery<RegressionSuite>(
    `/v1/workspaces/${workspaceId}/regression-suites`,
    { limit: PAGE_SIZE, offset },
  );
  const {
    data: proposedCasesPage,
    error: proposedCasesError,
    isLoading: proposedCasesLoading,
  } = usePaginatedApiQuery<RegressionCase>(
    `/v1/workspaces/${workspaceId}/regression-cases`,
    {
      status: "proposed",
      limit: workspacePageSizes.regressionCases,
      offset: caseOffset,
    },
  );
  const {
    data: packsResponse,
    error: packsError,
    isLoading: packsLoading,
  } = useApiListQuery<ChallengePack>(
    `/v1/workspaces/${workspaceId}/challenge-packs`,
  );
  const suites = suitesPage?.items ?? [];
  const total = suitesPage?.total ?? 0;
  const proposedCases = proposedCasesPage?.items ?? [];
  const proposedTotal = proposedCasesPage?.total ?? 0;
  const packs = packsResponse?.items ?? [];

  const packsById = new Map(packs.map((p) => [p.id, p]));
  const suitesById = new Map(suites.map((s) => [s.id, s]));
  const hasAnyPack = packs.length > 0;

  useEffect(() => {
    if (!initialCreateOpen) return;
    const url = new URL(window.location.href);
    url.searchParams.delete("create");
    url.searchParams.delete("sourcePackId");
    router.replace(`${url.pathname}${url.search}`, { scroll: false });
  }, [initialCreateOpen, router]);

  const visible = suites.filter((s) =>
    statusFilter === "all" ? true : s.status === statusFilter,
  );

  function handlePageChange(next: number) {
    const url = new URL(window.location.href);
    url.searchParams.set("offset", String(next));
    router.push(`${url.pathname}${url.search}`);
  }

  function handleCasePageChange(next: number) {
    const url = new URL(window.location.href);
    if (next === 0) {
      url.searchParams.delete("caseOffset");
    } else {
      url.searchParams.set("caseOffset", String(next));
    }
    router.push(`${url.pathname}${url.search}`);
  }

  async function patchStatus(
    suite: RegressionSuite,
    next: RegressionSuiteStatus,
  ) {
    setPending(suite.id);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const body: PatchRegressionSuiteInput = { status: next };
      await api.patch<RegressionSuite>(
        `/v1/workspaces/${workspaceId}/regression-suites/${suite.id}`,
        body,
      );
      toast.success(
        next === "archived" ? "Suite archived" : "Suite reactivated",
      );
      await mutate(workspaceResourceKeys.regressionSuites(workspaceId, offset));
    } catch (err) {
      if (err instanceof ApiError) {
        toast.error(err.message);
      } else {
        toast.error("Failed to update suite");
      }
    } finally {
      setPending(null);
    }
  }

  async function handleArchive(suite: RegressionSuite) {
    const ok = await confirm({
      title: `Archive "${suite.name}"?`,
      description:
        "Archived suites stop collecting new cases. You can reactivate from the actions menu.",
      confirmLabel: "Archive",
      variant: "danger",
    });
    if (!ok) return;
    await patchStatus(suite, "archived");
  }

  async function handleReactivate(suite: RegressionSuite) {
    await patchStatus(suite, "active");
  }

  async function patchCaseStatus(
    regressionCase: RegressionCase,
    next: "active" | "rejected",
  ) {
    setPending(`case:${regressionCase.id}`);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const body: PatchRegressionCaseInput = { status: next };
      await api.patch<RegressionCase>(
        `/v1/workspaces/${workspaceId}/regression-cases/${regressionCase.id}`,
        body,
      );
      toast.success(next === "active" ? "Case promoted" : "Case rejected");
      await Promise.all([
        mutate(workspaceResourceKeys.regressionCases(workspaceId, "proposed", caseOffset)),
        mutate(workspaceResourceKeys.regressionSuites(workspaceId, offset)),
      ]);
    } catch (err) {
      if (err instanceof ApiError) {
        toast.error(err.message);
      } else {
        toast.error("Failed to update case");
      }
    } finally {
      setPending(null);
    }
  }

  async function handlePromoteCase(regressionCase: RegressionCase) {
    await patchCaseStatus(regressionCase, "active");
  }

  async function handleRejectCase(regressionCase: RegressionCase) {
    const ok = await confirm({
      title: `Reject "${regressionCase.title}"?`,
      description:
        "Rejected cases leave the triage queue and stay available in the source suite history.",
      confirmLabel: "Reject",
      variant: "danger",
    });
    if (!ok) return;
    await patchCaseStatus(regressionCase, "rejected");
  }

  if ((suitesLoading && !suitesPage) || (packsLoading && !packsResponse)) {
    return <WorkspaceListLoading rows={6} />;
  }

  if (suitesError || packsError || proposedCasesError) {
    return (
      <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
        Failed to load regression data.
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Regression Suites"
        breadcrumbs={[{ label: "Regression Suites" }]}
        actions={
          hasAnyPack ? (
            <CreateSuiteDialog
              workspaceId={workspaceId}
              packs={packs}
              initialOpen={initialCreateOpen}
              initialPackId={initialCreatePackId}
              offset={offset}
            />
          ) : null
        }
      />

      {!hasAnyPack && (
        <div className="flex items-start gap-2 rounded-lg border border-border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
          <ShieldAlert className="mt-0.5 size-4 shrink-0" />
          <span>
            Publish an active challenge pack before creating regression
            suites. Cases are always anchored to a pack.
          </span>
        </div>
      )}

      {proposedCasesLoading && !proposedCasesPage ? (
        <div className="rounded-lg border border-border px-4 py-3 text-sm text-muted-foreground">
          Loading proposed cases...
        </div>
      ) : proposedTotal > 0 ? (
        <section className="space-y-3">
          <div className="flex items-center justify-between gap-2">
            <div>
              <h2 className="text-sm font-semibold text-foreground">
                Proposed Cases
              </h2>
              <p className="text-xs text-muted-foreground">
                {proposedTotal} waiting for review
              </p>
            </div>
          </div>

          <div className="rounded-lg border border-border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Case</TableHead>
                  <TableHead>Suite</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Severity</TableHead>
                  <TableHead>Failure Class</TableHead>
                  <TableHead>Validation</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="w-[12rem]" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {proposedCases.map((regressionCase) => {
                  const suite = suitesById.get(regressionCase.suite_id);
                  const disabled = pending === `case:${regressionCase.id}`;
                  return (
                    <TableRow key={regressionCase.id}>
                      <TableCell>
                        <Link
                          href={`/workspaces/${workspaceId}/regression-suites/${regressionCase.suite_id}/cases/${regressionCase.id}`}
                          className="font-medium text-foreground hover:underline underline-offset-4"
                        >
                          {regressionCase.title}
                        </Link>
                        {regressionCase.failure_summary && (
                          <p className="mt-0.5 max-w-md truncate text-xs text-muted-foreground">
                            {regressionCase.failure_summary}
                          </p>
                        )}
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        <Link
                          href={`/workspaces/${workspaceId}/regression-suites/${regressionCase.suite_id}`}
                          className="hover:text-foreground hover:underline underline-offset-4"
                        >
                          {suite?.name ?? shortID(regressionCase.suite_id)}
                        </Link>
                      </TableCell>
                      <TableCell>
                        <CaseStatusBadge status={regressionCase.status} />
                      </TableCell>
                      <TableCell>
                        <SeverityBadge severity={regressionCase.severity} />
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        {regressionCase.failure_class}
                      </TableCell>
                      <TableCell>
                        <ProposedCaseValidationSummary
                          regressionCase={regressionCase}
                        />
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        {new Date(regressionCase.created_at).toLocaleDateString()}
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center justify-end gap-1.5">
                          <Button
                            variant="outline"
                            size="sm"
                            disabled={disabled}
                            onClick={() => handlePromoteCase(regressionCase)}
                          >
                            <Check className="size-4" />
                            Promote
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            disabled={disabled}
                            onClick={() => handleRejectCase(regressionCase)}
                            title="Reject"
                          >
                            <XCircle className="size-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>

          <PaginationControls
            offset={caseOffset}
            total={proposedTotal}
            pageSize={workspacePageSizes.regressionCases}
            onPrev={() =>
              handleCasePageChange(
                Math.max(0, caseOffset - workspacePageSizes.regressionCases),
              )
            }
            onNext={() => {
              const next = caseOffset + workspacePageSizes.regressionCases;
              if (next < proposedTotal) handleCasePageChange(next);
            }}
          />
        </section>
      ) : null}

      {suites.length === 0 ? (
        <EmptyState
          icon={<ShieldAlert className="size-10" />}
          title="No regression suites yet"
          description="Create a suite to curate regression cases promoted from failed runs."
        />
      ) : (
        <div className="space-y-3">
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground">Status</span>
              <Select
                value={statusFilter}
                onValueChange={(v) => v && setStatusFilter(v as StatusFilter)}
              >
                <SelectTrigger size="sm" className="min-w-[8rem]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All</SelectItem>
                  <SelectItem value="active">Active</SelectItem>
                  <SelectItem value="archived">Archived</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <span className="text-xs text-muted-foreground">
              {visible.length} of {total}
            </span>
          </div>

          <div className="rounded-lg border border-border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Source Pack</TableHead>
                  <TableHead>Cases</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Default Severity</TableHead>
                  <TableHead>Updated</TableHead>
                  <TableHead className="w-8" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {visible.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={7}
                      className="py-8 text-center text-sm text-muted-foreground"
                    >
                      No suites match the current filter.
                    </TableCell>
                  </TableRow>
                ) : (
                  visible.map((suite) => {
                    const pack = packsById.get(suite.source_challenge_pack_id);
                    const isArchived = suite.status === "archived";
                    const disabled = pending === suite.id;
                    return (
                      <TableRow key={suite.id}>
                        <TableCell>
                          <Link
                            href={`/workspaces/${workspaceId}/regression-suites/${suite.id}`}
                            className="font-medium text-foreground hover:underline underline-offset-4"
                          >
                            {suite.name}
                          </Link>
                          {suite.description && (
                            <p className="mt-0.5 max-w-md truncate text-xs text-muted-foreground">
                              {suite.description}
                            </p>
                          )}
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {pack ? (
                            <Link
                              href={`/workspaces/${workspaceId}/challenge-packs/${pack.id}`}
                              className="hover:text-foreground hover:underline underline-offset-4"
                            >
                              {pack.name}
                            </Link>
                          ) : (
                            <span title={suite.source_challenge_pack_id}>
                              {"\u2014"}
                            </span>
                          )}
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {suite.case_count}
                        </TableCell>
                        <TableCell>
                          <SuiteStatusBadge status={suite.status} />
                        </TableCell>
                        <TableCell>
                          <SeverityBadge
                            severity={suite.default_gate_severity}
                          />
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {new Date(suite.updated_at).toLocaleDateString()}
                        </TableCell>
                        <TableCell>
                          <DropdownMenu>
                            <DropdownMenuTrigger
                              render={
                                <Button
                                  variant="ghost"
                                  size="icon-xs"
                                  disabled={disabled}
                                />
                              }
                            >
                              <MoreHorizontal className="size-4" />
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              {isArchived ? (
                                <DropdownMenuItem
                                  onClick={() => handleReactivate(suite)}
                                >
                                  Reactivate
                                </DropdownMenuItem>
                              ) : (
                                <DropdownMenuItem
                                  className="text-destructive"
                                  onClick={() => handleArchive(suite)}
                                >
                                  Archive
                                </DropdownMenuItem>
                              )}
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </TableCell>
                      </TableRow>
                    );
                  })
                )}
              </TableBody>
            </Table>
          </div>

          <PaginationControls
            offset={offset}
            total={total}
            pageSize={PAGE_SIZE}
            onPrev={() => handlePageChange(Math.max(0, offset - PAGE_SIZE))}
            onNext={() => {
              const next = offset + PAGE_SIZE;
              if (next < total) handlePageChange(next);
            }}
          />
        </div>
      )}
    </div>
  );
}
