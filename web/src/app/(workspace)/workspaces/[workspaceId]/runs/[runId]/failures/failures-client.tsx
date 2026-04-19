"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useTransition,
} from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import {
  AlertTriangle,
  ChevronDown,
  ChevronRight,
  Inbox,
  Play,
  X,
} from "lucide-react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { listRunFailures } from "@/lib/api/failure-reviews";
import type {
  FailureReviewEvidenceTier,
  FailureReviewFailureClass,
  FailureReviewItem,
  FailureReviewSeverity,
  ListRunFailuresResponse,
  RunAgent,
} from "@/lib/api/types";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorBoundary } from "@/components/ui/error-boundary";
import { FailureDetailDrawer } from "./failure-detail-drawer";
import { PromoteFailureDialog } from "./promote-failure-dialog";

// --- Filter enums + labels ---

const SEVERITY_OPTIONS: FailureReviewSeverity[] = [
  "info",
  "warning",
  "blocking",
];

const FAILURE_CLASS_OPTIONS: FailureReviewFailureClass[] = [
  "incorrect_final_output",
  "tool_selection_error",
  "tool_argument_error",
  "retrieval_grounding_failure",
  "policy_violation",
  "timeout_or_budget_exhaustion",
  "sandbox_failure",
  "malformed_output",
  "flaky_non_deterministic",
  "insufficient_evidence",
  "other",
];

const EVIDENCE_TIER_OPTIONS: FailureReviewEvidenceTier[] = [
  "none",
  "native_structured",
  "hosted_structured",
  "hosted_black_box",
  "derived_summary",
];

function humanize(value: string): string {
  return value.replace(/_/g, " ");
}

const severityVariant: Record<
  FailureReviewSeverity,
  "default" | "secondary" | "outline" | "destructive"
> = {
  info: "secondary",
  warning: "outline",
  blocking: "destructive",
};

const failureStateVariant: Record<
  FailureReviewItem["failure_state"],
  "default" | "secondary" | "outline" | "destructive"
> = {
  failed: "destructive",
  warning: "outline",
  flaky: "outline",
  incomplete_evidence: "secondary",
};

// --- URL <-> state helpers ---

interface Filters {
  agentId?: string;
  severity?: FailureReviewSeverity;
  failureClass?: FailureReviewFailureClass;
  evidenceTier?: FailureReviewEvidenceTier;
}

function parseFilters(params: URLSearchParams): Filters {
  return {
    agentId: params.get("agent") ?? undefined,
    severity:
      (params.get("severity") as FailureReviewSeverity | null) ?? undefined,
    failureClass:
      (params.get("class") as FailureReviewFailureClass | null) ?? undefined,
    evidenceTier:
      (params.get("tier") as FailureReviewEvidenceTier | null) ?? undefined,
  };
}

function filtersToQuery(filters: Filters): string {
  const sp = new URLSearchParams();
  if (filters.agentId) sp.set("agent", filters.agentId);
  if (filters.severity) sp.set("severity", filters.severity);
  if (filters.failureClass) sp.set("class", filters.failureClass);
  if (filters.evidenceTier) sp.set("tier", filters.evidenceTier);
  const s = sp.toString();
  return s ? `?${s}` : "";
}

function filtersEqual(a: Filters, b: Filters): boolean {
  return (
    a.agentId === b.agentId &&
    a.severity === b.severity &&
    a.failureClass === b.failureClass &&
    a.evidenceTier === b.evidenceTier
  );
}

// --- Component ---

interface FailuresClientProps {
  workspaceId: string;
  runId: string;
  runName: string;
  agents: RunAgent[];
  initialPage: ListRunFailuresResponse;
  initialLimit: number;
  sourceChallengePackId?: string;
  sourceChallengePackName?: string | null;
}

export function FailuresClient(props: FailuresClientProps) {
  return (
    <ErrorBoundary>
      <FailuresClientInner {...props} />
    </ErrorBoundary>
  );
}

function FailuresClientInner({
  workspaceId,
  runId,
  agents,
  initialPage,
  initialLimit,
  sourceChallengePackId,
  sourceChallengePackName,
}: FailuresClientProps) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { getAccessToken } = useAccessToken();

  const urlFilters = useMemo(
    () => parseFilters(searchParams),
    [searchParams],
  );

  // Drive data from filters. Initial SSR page corresponds to "no filters".
  const [items, setItems] = useState<FailureReviewItem[]>(initialPage.items);
  const [cursor, setCursor] = useState<string | undefined>(
    initialPage.next_cursor,
  );
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<FailureReviewItem | null>(null);
  const [promoting, setPromoting] = useState<FailureReviewItem | null>(null);
  const [, startTransition] = useTransition();

  // Track the filter set currently reflected in `items` so we don't refetch
  // the initial SSR page when the URL already matches empty filters.
  const activeFiltersRef = useRef<Filters>({});

  const anyFilterActive = useMemo(
    () => !!(urlFilters.agentId || urlFilters.severity || urlFilters.failureClass || urlFilters.evidenceTier),
    [urlFilters],
  );

  // Refetch when filters change. Skip the initial render if URL has no
  // filters — SSR already covered it.
  useEffect(() => {
    if (filtersEqual(urlFilters, activeFiltersRef.current)) return;

    let cancelled = false;
    const controller = new AbortController();
    setLoading(true);
    setError(null);

    (async () => {
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const res = await listRunFailures(api, workspaceId, runId, {
          limit: initialLimit,
          signal: controller.signal,
          ...urlFilters,
        });
        if (cancelled) return;
        setItems(res.items);
        setCursor(res.next_cursor);
        activeFiltersRef.current = urlFilters;
      } catch (err) {
        if (cancelled) return;
        if (err instanceof ApiError) {
          setError(err.message);
        } else if (err instanceof Error && err.name !== "AbortError") {
          setError(err.message);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [urlFilters, workspaceId, runId, initialLimit, getAccessToken]);

  const updateFilter = useCallback(
    <K extends keyof Filters>(key: K, value: Filters[K]) => {
      const next: Filters = { ...urlFilters, [key]: value };
      const query = filtersToQuery(next);
      startTransition(() => {
        router.replace(
          `/workspaces/${workspaceId}/runs/${runId}/failures${query}`,
          { scroll: false },
        );
      });
    },
    [urlFilters, router, workspaceId, runId],
  );

  const clearFilters = useCallback(() => {
    startTransition(() => {
      router.replace(`/workspaces/${workspaceId}/runs/${runId}/failures`, {
        scroll: false,
      });
    });
  }, [router, workspaceId, runId]);

  const loadMore = useCallback(async () => {
    if (!cursor || loadingMore) return;
    setLoadingMore(true);
    setError(null);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const res = await listRunFailures(api, workspaceId, runId, {
        limit: initialLimit,
        cursor,
        ...urlFilters,
      });
      setItems((prev) => [...prev, ...res.items]);
      setCursor(res.next_cursor);
    } catch (err) {
      if (err instanceof ApiError) setError(err.message);
      else if (err instanceof Error) setError(err.message);
    } finally {
      setLoadingMore(false);
    }
  }, [cursor, loadingMore, getAccessToken, workspaceId, runId, initialLimit, urlFilters]);

  const agentLabel = useMemo(() => {
    const map = new Map<string, string>();
    for (const a of agents) map.set(a.id, a.label);
    return map;
  }, [agents]);

  const groups = useMemo(() => groupByChallenge(items), [items]);

  // Default-collapse the accordion when the list is large.
  const largeList = items.length > 20;

  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-lg font-semibold tracking-tight mb-1">Failures</h1>
        <p className="text-sm text-muted-foreground">
          Per-case failure review for this run. Use filters to narrow by agent,
          severity, failure class, or evidence tier.
        </p>
      </header>

      <FilterBar
        agents={agents}
        filters={urlFilters}
        onChange={updateFilter}
        onClear={anyFilterActive ? clearFilters : undefined}
      />

      {loading ? (
        <ListSkeleton />
      ) : error ? (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-6 text-center text-sm text-destructive">
          <AlertTriangle className="size-5 mx-auto mb-2" />
          {error}
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          icon={<Inbox className="size-8" />}
          title="No failures recorded for this run."
          description={
            anyFilterActive
              ? "No failures match the current filters."
              : "This run completed without any failed cases."
          }
          action={
            anyFilterActive
              ? { label: "Clear filters", onClick: clearFilters }
              : undefined
          }
        />
      ) : (
        <div className="space-y-3">
          {groups.map((group) => (
            <ChallengeGroup
              key={group.challengeKey}
              challengeKey={group.challengeKey}
              items={group.items}
              agentLabels={agentLabel}
              defaultOpen={!largeList}
              workspaceId={workspaceId}
              runId={runId}
              onSelect={setSelected}
              onPromote={setPromoting}
              sourceChallengePackId={sourceChallengePackId}
            />
          ))}

          {cursor && (
            <div className="flex justify-center pt-2">
              <Button
                variant="outline"
                size="sm"
                onClick={loadMore}
                disabled={loadingMore}
              >
                {loadingMore ? "Loading…" : "Load more"}
              </Button>
            </div>
          )}
        </div>
      )}

      <FailureDetailDrawer
        item={selected}
        onClose={() => setSelected(null)}
        workspaceId={workspaceId}
      />
      <PromoteFailureDialog
        workspaceId={workspaceId}
        runId={runId}
        item={promoting}
        sourceChallengePackId={sourceChallengePackId}
        sourceChallengePackName={sourceChallengePackName}
        onClose={() => setPromoting(null)}
      />
    </div>
  );
}

// --- Sub-components ---

function FilterBar({
  agents,
  filters,
  onChange,
  onClear,
}: {
  agents: RunAgent[];
  filters: Filters;
  onChange: <K extends keyof Filters>(key: K, value: Filters[K]) => void;
  onClear?: () => void;
}) {
  return (
    <div className="flex flex-wrap items-end gap-3 rounded-lg border border-border bg-card/40 p-3">
      <FilterSelect
        label="Agent"
        value={filters.agentId ?? ""}
        onChange={(v) => onChange("agentId", v || undefined)}
        options={[
          { value: "", label: "All agents" },
          ...agents.map((a) => ({ value: a.id, label: a.label })),
        ]}
      />
      <FilterSelect
        label="Severity"
        value={filters.severity ?? ""}
        onChange={(v) =>
          onChange(
            "severity",
            (v || undefined) as FailureReviewSeverity | undefined,
          )
        }
        options={[
          { value: "", label: "All severities" },
          ...SEVERITY_OPTIONS.map((s) => ({ value: s, label: humanize(s) })),
        ]}
      />
      <FilterSelect
        label="Failure class"
        value={filters.failureClass ?? ""}
        onChange={(v) =>
          onChange(
            "failureClass",
            (v || undefined) as FailureReviewFailureClass | undefined,
          )
        }
        options={[
          { value: "", label: "All classes" },
          ...FAILURE_CLASS_OPTIONS.map((c) => ({
            value: c,
            label: humanize(c),
          })),
        ]}
      />
      <FilterSelect
        label="Evidence tier"
        value={filters.evidenceTier ?? ""}
        onChange={(v) =>
          onChange(
            "evidenceTier",
            (v || undefined) as FailureReviewEvidenceTier | undefined,
          )
        }
        options={[
          { value: "", label: "All tiers" },
          ...EVIDENCE_TIER_OPTIONS.map((t) => ({
            value: t,
            label: humanize(t),
          })),
        ]}
      />

      {onClear && (
        <Button
          variant="ghost"
          size="sm"
          onClick={onClear}
          className="ml-auto"
        >
          <X className="size-3.5 mr-1" />
          Clear
        </Button>
      )}
    </div>
  );
}

function FilterSelect({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
        {label}
      </span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={cn(
          "h-8 rounded-md border border-input bg-transparent px-2.5 text-sm",
          "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 outline-none",
          "dark:bg-input/30",
        )}
      >
        {options.map((opt) => (
          <option key={opt.value} value={opt.value}>
            {opt.label}
          </option>
        ))}
      </select>
    </label>
  );
}

interface ChallengeGroupProps {
  challengeKey: string;
  items: FailureReviewItem[];
  agentLabels: Map<string, string>;
  defaultOpen: boolean;
  workspaceId: string;
  runId: string;
  onSelect: (item: FailureReviewItem) => void;
  onPromote: (item: FailureReviewItem) => void;
  sourceChallengePackId?: string;
}

function ChallengeGroup({
  challengeKey,
  items,
  agentLabels,
  defaultOpen,
  workspaceId,
  runId,
  onSelect,
  onPromote,
  sourceChallengePackId,
}: ChallengeGroupProps) {
  const [open, setOpen] = useState(defaultOpen);
  const blockingCount = items.filter((i) => i.severity === "blocking").length;

  return (
    <div className="rounded-lg border border-border overflow-hidden">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="w-full flex items-center gap-2 px-4 py-2.5 text-left hover:bg-muted/40 transition-colors"
      >
        {open ? (
          <ChevronDown className="size-4 text-muted-foreground" />
        ) : (
          <ChevronRight className="size-4 text-muted-foreground" />
        )}
        <span className="font-medium text-sm truncate">{challengeKey}</span>
        <span className="text-xs text-muted-foreground ml-2">
          {items.length} failure{items.length === 1 ? "" : "s"}
        </span>
        {blockingCount > 0 && (
          <Badge variant="destructive" className="ml-1">
            {blockingCount} blocking
          </Badge>
        )}
      </button>

      {open && (
        <ul className="divide-y divide-border border-t border-border">
          {items.map((item) => (
            <FailureRow
              key={`${item.run_agent_id}:${item.item_key}`}
              item={item}
              agentLabel={agentLabels.get(item.run_agent_id) ?? "Agent"}
              workspaceId={workspaceId}
              runId={runId}
              onSelect={onSelect}
              onPromote={onPromote}
              sourceChallengePackId={sourceChallengePackId}
            />
          ))}
        </ul>
      )}
    </div>
  );
}

function FailureRow({
  item,
  agentLabel,
  workspaceId,
  runId,
  onSelect,
  onPromote,
  sourceChallengePackId,
}: {
  item: FailureReviewItem;
  agentLabel: string;
  workspaceId: string;
  runId: string;
  onSelect: (item: FailureReviewItem) => void;
  onPromote: (item: FailureReviewItem) => void;
  sourceChallengePackId?: string;
}) {
  const firstReplayStep = item.replay_step_refs[0]?.sequence_number;
  const replayHref = firstReplayStep
    ? `/workspaces/${workspaceId}/runs/${runId}/agents/${item.run_agent_id}/replay?step=${firstReplayStep}`
    : undefined;
  const canPromote = item.promotable && Boolean(item.challenge_identity_id);
  const promoteDisabled = !sourceChallengePackId || !item.challenge_identity_id;

  return (
    <li className="px-4 py-3 hover:bg-muted/30 transition-colors">
      <div className="flex items-start gap-3">
        <button
          type="button"
          onClick={() => onSelect(item)}
          className="min-w-0 flex-1 text-left"
        >
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-medium text-sm truncate">{item.case_key}</span>
            <span className="text-xs text-muted-foreground">· {agentLabel}</span>
            <Badge variant={failureStateVariant[item.failure_state]}>
              {humanize(item.failure_state)}
            </Badge>
            <Badge variant="outline">{humanize(item.failure_class)}</Badge>
            <Badge variant={severityVariant[item.severity]}>
              {item.severity}
            </Badge>
            <Badge variant="secondary">{humanize(item.evidence_tier)}</Badge>
          </div>

          {item.headline && (
            <p className="mt-2 text-sm text-foreground/90 line-clamp-2">
              {item.headline}
            </p>
          )}
        </button>

        {canPromote && (
          <Button
            variant="outline"
            size="sm"
            disabled={promoteDisabled}
            onClick={() => onPromote(item)}
          >
            Promote
          </Button>
        )}
      </div>

      <div className="mt-2 flex items-center gap-3 text-xs text-muted-foreground flex-wrap">
        {item.failed_dimensions.length > 0 && (
          <span>
            Failed:{" "}
            <span className="text-foreground/80 font-[family-name:var(--font-mono)]">
              {item.failed_dimensions.join(", ")}
            </span>
          </span>
        )}
        {replayHref && (
          <Link
            href={replayHref}
            className="flex items-center gap-1 hover:text-foreground transition-colors"
          >
            <Play className="size-3" />
            Replay step #{firstReplayStep}
          </Link>
        )}
      </div>
    </li>
  );
}

function ListSkeleton() {
  return (
    <div className="space-y-3">
      {Array.from({ length: 3 }).map((_, i) => (
        <div
          key={i}
          className="rounded-lg border border-border p-4 space-y-3"
        >
          <Skeleton className="h-5 w-40" />
          <Skeleton className="h-4 w-full" />
          <div className="flex gap-2">
            <Skeleton className="h-5 w-20" />
            <Skeleton className="h-5 w-24" />
            <Skeleton className="h-5 w-16" />
          </div>
        </div>
      ))}
    </div>
  );
}

interface ChallengeGroupData {
  challengeKey: string;
  items: FailureReviewItem[];
}

function groupByChallenge(items: FailureReviewItem[]): ChallengeGroupData[] {
  const groups = new Map<string, FailureReviewItem[]>();
  for (const item of items) {
    const key = item.challenge_key;
    const existing = groups.get(key);
    if (existing) existing.push(item);
    else groups.set(key, [item]);
  }
  return Array.from(groups.entries()).map(([challengeKey, items]) => ({
    challengeKey,
    items,
  }));
}
