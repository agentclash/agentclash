"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { MoreHorizontal, ShieldAlert } from "lucide-react";

import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  ChallengePack,
  PatchRegressionSuiteInput,
  RegressionSuite,
  RegressionSuiteStatus,
} from "@/lib/api/types";
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

import { SeverityBadge, SuiteStatusBadge } from "./badges";
import { CreateSuiteDialog } from "./create-suite-dialog";

const PAGE_SIZE = 50;

type StatusFilter = "all" | RegressionSuiteStatus;

interface RegressionSuitesClientProps {
  workspaceId: string;
  suites: RegressionSuite[];
  total: number;
  offset: number;
  packs: ChallengePack[];
  initialCreateOpen?: boolean;
  initialCreatePackId?: string;
}

export function RegressionSuitesClient(props: RegressionSuitesClientProps) {
  return (
    <ConfirmProvider>
      <RegressionSuitesInner {...props} />
    </ConfirmProvider>
  );
}

function RegressionSuitesInner({
  workspaceId,
  suites,
  total,
  offset,
  packs,
  initialCreateOpen,
  initialCreatePackId,
}: RegressionSuitesClientProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const confirm = useConfirm();
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [pending, setPending] = useState<string | null>(null);

  const packsById = new Map(packs.map((p) => [p.id, p]));
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
      router.refresh();
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
