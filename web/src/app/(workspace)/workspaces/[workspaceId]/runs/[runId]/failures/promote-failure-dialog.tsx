"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { ChevronDown, ChevronRight, Loader2 } from "lucide-react";
import { toast } from "sonner";

import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import {
  buildPromotionOverrides,
  defaultPromotionSeverityForFailure,
  listRegressionSuites,
  promoteFailure,
} from "@/lib/api/regression";
import type {
  FailureReviewItem,
  FailureReviewPromotionMode,
  RegressionSeverity,
  RegressionSuite,
} from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const SEVERITY_OPTIONS: RegressionSeverity[] = ["info", "warning", "blocking"];

interface PromoteFailureDialogProps {
  workspaceId: string;
  runId: string;
  item: FailureReviewItem | null;
  sourceChallengePackId?: string;
  sourceChallengePackName?: string | null;
  onClose: () => void;
}

function humanize(value: string): string {
  return value.replace(/_/g, " ");
}

function promotionModeLabel(mode: FailureReviewPromotionMode): string {
  return mode === "full_executable" ? "Full executable" : "Output only";
}

export function PromoteFailureDialog({
  workspaceId,
  runId,
  item,
  sourceChallengePackId,
  sourceChallengePackName,
  onClose,
}: PromoteFailureDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();

  const open = item != null;
  const [title, setTitle] = useState("");
  const [failureSummary, setFailureSummary] = useState("");
  const [selectedSuiteId, setSelectedSuiteId] = useState("");
  const [promotionMode, setPromotionMode] =
    useState<FailureReviewPromotionMode>("full_executable");
  const [severity, setSeverity] = useState<RegressionSeverity>("warning");
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [judgeThresholds, setJudgeThresholds] = useState<Record<string, string>>(
    {},
  );
  const [assertionToggles, setAssertionToggles] = useState<
    Record<string, boolean | undefined>
  >({});
  const [loadingSuites, setLoadingSuites] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [suiteLoadError, setSuiteLoadError] = useState<string | null>(null);
  const [eligibleSuites, setEligibleSuites] = useState<RegressionSuite[]>([]);

  const judgeKeys = useMemo(
    () =>
      Array.from(new Set((item?.judge_refs ?? []).map((judge) => judge.key))).sort(),
    [item],
  );
  const assertionKeys = useMemo(
    () => Array.from(new Set(item?.failed_checks ?? [])).sort(),
    [item],
  );

  useEffect(() => {
    if (!item) {
      setEligibleSuites([]);
      setLoadingSuites(false);
      setSuiteLoadError(null);
      setSubmitting(false);
      return;
    }

    const defaultMode = item.promotion_mode_available.includes("full_executable")
      ? "full_executable"
      : item.promotion_mode_available[0] ?? "output_only";

    setTitle(item.headline || "");
    setFailureSummary(item.detail || "");
    setSelectedSuiteId("");
    setPromotionMode(defaultMode);
    setSeverity(defaultPromotionSeverityForFailure(item.failure_class));
    setShowAdvanced(false);
    setJudgeThresholds({});
    setAssertionToggles({});
    setSuiteLoadError(null);
    setEligibleSuites([]);

    if (!sourceChallengePackId) {
      setLoadingSuites(false);
      setSuiteLoadError("This run's source challenge pack could not be resolved.");
      return;
    }

    let cancelled = false;
    setLoadingSuites(true);

    (async () => {
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const suites: RegressionSuite[] = [];
        let offset = 0;

        while (true) {
          const page = await listRegressionSuites(api, workspaceId, {
            limit: 100,
            offset,
          });
          suites.push(...page.items);
          offset += page.limit;
          if (suites.length >= page.total || page.items.length === 0) break;
        }

        if (cancelled) return;

        const filtered = suites.filter(
          (suite) =>
            suite.status === "active" &&
            suite.source_challenge_pack_id === sourceChallengePackId,
        );
        setEligibleSuites(filtered);
        setSelectedSuiteId(filtered[0]?.id ?? "");
      } catch (err) {
        if (cancelled) return;
        setSuiteLoadError(
          err instanceof ApiError
            ? err.message
            : err instanceof Error
              ? err.message
              : "Failed to load regression suites",
        );
      } finally {
        if (!cancelled) setLoadingSuites(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [getAccessToken, item, sourceChallengePackId, workspaceId]);

  const createSuiteHref = sourceChallengePackId
    ? `/workspaces/${workspaceId}/regression-suites?create=1&sourcePackId=${sourceChallengePackId}`
    : `/workspaces/${workspaceId}/regression-suites?create=1`;

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!item?.challenge_identity_id || submitting) return;

    const trimmedTitle = title.trim();
    if (!trimmedTitle) {
      toast.error("Title is required");
      return;
    }
    if (!selectedSuiteId) {
      toast.error("Select a destination suite");
      return;
    }

    const parsedThresholds: Record<string, number | undefined> = {};
    for (const [key, rawValue] of Object.entries(judgeThresholds)) {
      const trimmed = rawValue.trim();
      if (!trimmed) {
        parsedThresholds[key] = undefined;
        continue;
      }
      const value = Number(trimmed);
      if (!Number.isFinite(value)) {
        toast.error(`Judge threshold for ${key} must be a number`);
        return;
      }
      parsedThresholds[key] = value;
    }

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const result = await promoteFailure(
        api,
        workspaceId,
        runId,
        item.challenge_identity_id,
        {
          suite_id: selectedSuiteId,
          promotion_mode: promotionMode,
          title: trimmedTitle,
          failure_summary: failureSummary.trim(),
          severity,
          validator_overrides: buildPromotionOverrides({
            judgeThresholdOverrides: parsedThresholds,
            assertionToggles,
          }),
        },
      );

      const caseHref = `/workspaces/${workspaceId}/regression-suites/${result.case.suite_id}/cases/${result.case.id}`;
      if (result.created) {
        toast.success("Failure promoted", {
          action: {
            label: "Open case",
            onClick: () => router.push(caseHref),
          },
        });
      } else {
        toast("Already promoted - open case", {
          action: {
            label: "Open case",
            onClick: () => router.push(caseHref),
          },
        });
      }

      onClose();
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to promote failure",
      );
    } finally {
      setSubmitting(false);
    }
  }

  function handleAssertionToggle(key: string, next: boolean) {
    setAssertionToggles((prev) => ({
      ...prev,
      [key]: prev[key] === next ? undefined : next,
    }));
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Promote failure</DialogTitle>
          <DialogDescription>
            Turn this failure into a reusable regression case for future runs.
          </DialogDescription>
        </DialogHeader>

        {item ? (
          <form
            id="promote-failure-form"
            onSubmit={handleSubmit}
            className="space-y-5"
          >
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-1.5 sm:col-span-2">
                <label
                  htmlFor="promote-failure-title"
                  className="text-xs font-medium text-muted-foreground"
                >
                  Title
                </label>
                <Input
                  id="promote-failure-title"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  placeholder="Policy regression"
                  required
                />
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-medium text-muted-foreground">
                  Destination suite
                </label>
                {loadingSuites ? (
                  <div className="flex h-10 items-center rounded-lg border border-input px-3 text-sm text-muted-foreground">
                    <Loader2 className="mr-2 size-4 animate-spin" />
                    Loading suites...
                  </div>
                ) : suiteLoadError ? (
                  <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
                    {suiteLoadError}
                  </div>
                ) : eligibleSuites.length === 0 ? (
                  <div className="rounded-lg border border-dashed border-border px-3 py-3 text-sm text-muted-foreground">
                    <p>
                      No active regression suites match{" "}
                      {sourceChallengePackName ?? "this challenge pack"}.
                    </p>
                    <Link
                      href={createSuiteHref}
                      className="mt-2 inline-flex font-medium text-foreground hover:underline underline-offset-4"
                      onClick={onClose}
                    >
                      Create suite
                    </Link>
                  </div>
                ) : (
                  <Select
                    value={selectedSuiteId}
                    onValueChange={(value) => value && setSelectedSuiteId(value)}
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder="Select a suite" />
                    </SelectTrigger>
                    <SelectContent>
                      {eligibleSuites.map((suite) => (
                        <SelectItem key={suite.id} value={suite.id}>
                          {suite.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-medium text-muted-foreground">
                  Promotion mode
                </label>
                <div className="rounded-lg border border-input p-1">
                  {item.promotion_mode_available.map((mode) => (
                    <label
                      key={mode}
                      className="flex cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-muted/40"
                    >
                      <input
                        type="radio"
                        name="promotion-mode"
                        value={mode}
                        checked={promotionMode === mode}
                        onChange={() => setPromotionMode(mode)}
                      />
                      <span>{promotionModeLabel(mode)}</span>
                    </label>
                  ))}
                </div>
              </div>

              <div className="space-y-1.5 sm:col-span-2">
                <label className="text-xs font-medium text-muted-foreground">
                  Severity
                </label>
                <div className="grid gap-2 sm:grid-cols-3">
                  {SEVERITY_OPTIONS.map((option) => (
                    <label
                      key={option}
                      className="flex cursor-pointer items-center gap-2 rounded-lg border border-input px-3 py-2 text-sm hover:bg-muted/40"
                    >
                      <input
                        type="radio"
                        name="promotion-severity"
                        value={option}
                        checked={severity === option}
                        onChange={() => setSeverity(option)}
                      />
                      <span className="flex-1">{humanize(option)}</span>
                    </label>
                  ))}
                </div>
                <p className="text-xs text-muted-foreground">
                  Default matches the backend rule for {humanize(item.failure_class)}.
                </p>
              </div>

              <div className="space-y-1.5 sm:col-span-2">
                <label
                  htmlFor="promote-failure-summary"
                  className="text-xs font-medium text-muted-foreground"
                >
                  Failure summary
                </label>
                <textarea
                  id="promote-failure-summary"
                  value={failureSummary}
                  onChange={(e) => setFailureSummary(e.target.value)}
                  rows={5}
                  className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring/50 resize-y"
                />
              </div>
            </div>

            <div className="rounded-lg border border-border">
              <button
                type="button"
                onClick={() => setShowAdvanced((current) => !current)}
                className="flex w-full items-center justify-between px-4 py-3 text-left text-sm font-medium"
              >
                <span>Advanced validator overrides</span>
                {showAdvanced ? (
                  <ChevronDown className="size-4 text-muted-foreground" />
                ) : (
                  <ChevronRight className="size-4 text-muted-foreground" />
                )}
              </button>

              {showAdvanced && (
                <div className="space-y-5 border-t border-border px-4 py-4">
                  <div className="space-y-3">
                    <div>
                      <h3 className="text-sm font-medium">Judge thresholds</h3>
                      <p className="text-xs text-muted-foreground">
                        Leave blank to keep the suite default threshold.
                      </p>
                    </div>
                    {judgeKeys.length === 0 ? (
                      <p className="text-sm text-muted-foreground">
                        No judge thresholds are available for this failure.
                      </p>
                    ) : (
                      <div className="space-y-2">
                        {judgeKeys.map((key) => {
                          const judge = item.judge_refs.find((entry) => entry.key === key);
                          return (
                            <div
                              key={key}
                              className="grid gap-2 rounded-lg border border-input px-3 py-2 sm:grid-cols-[minmax(0,1fr)_12rem]"
                            >
                              <div className="min-w-0">
                                <div className="truncate font-[family-name:var(--font-mono)] text-xs text-foreground">
                                  {key}
                                </div>
                                {judge?.reason && (
                                  <p className="mt-1 text-xs text-muted-foreground">
                                    {judge.reason}
                                  </p>
                                )}
                              </div>
                              <Input
                                type="number"
                                min="0"
                                max="1"
                                step="0.01"
                                value={judgeThresholds[key] ?? ""}
                                onChange={(e) =>
                                  setJudgeThresholds((prev) => ({
                                    ...prev,
                                    [key]: e.target.value,
                                  }))
                                }
                                placeholder={
                                  judge?.normalized_score != null
                                    ? String(judge.normalized_score)
                                    : "0.90"
                                }
                              />
                            </div>
                          );
                        })}
                      </div>
                    )}
                  </div>

                  <div className="space-y-3">
                    <div>
                      <h3 className="text-sm font-medium">Assertion toggles</h3>
                      <p className="text-xs text-muted-foreground">
                        Leave both unchecked to inherit the suite default. Click
                        a selected option again to clear the override.
                      </p>
                    </div>
                    {assertionKeys.length === 0 ? (
                      <p className="text-sm text-muted-foreground">
                        No assertion toggles are available for this failure.
                      </p>
                    ) : (
                      <div className="space-y-2">
                        {assertionKeys.map((key) => (
                          <div
                            key={key}
                            className="grid gap-3 rounded-lg border border-input px-3 py-2 sm:grid-cols-[minmax(0,1fr)_5rem_5rem]"
                          >
                            <div className="min-w-0 font-[family-name:var(--font-mono)] text-xs text-foreground">
                              {key}
                            </div>
                            <label className="flex items-center justify-center gap-2 text-xs text-muted-foreground">
                              <input
                                type="checkbox"
                                checked={assertionToggles[key] === true}
                                onChange={() => handleAssertionToggle(key, true)}
                              />
                              On
                            </label>
                            <label className="flex items-center justify-center gap-2 text-xs text-muted-foreground">
                              <input
                                type="checkbox"
                                checked={assertionToggles[key] === false}
                                onChange={() => handleAssertionToggle(key, false)}
                              />
                              Off
                            </label>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>
          </form>
        ) : null}

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button
            type="submit"
            form="promote-failure-form"
            disabled={
              submitting ||
              loadingSuites ||
              !item?.challenge_identity_id ||
              !selectedSuiteId
            }
          >
            {submitting ? <Loader2 className="size-4 animate-spin" /> : "Promote"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
