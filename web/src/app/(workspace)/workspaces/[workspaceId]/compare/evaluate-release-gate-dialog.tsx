"use client";

import { useEffect, useRef, useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { listRegressionSuites } from "@/lib/api/regression";
import {
  EMPTY_REGRESSION_GATE_RULES_DRAFT,
  evaluateReleaseGate,
  normalizeRegressionGateRules,
  regressionGateRulesToDraft,
  type RegressionGateRulesDraft,
} from "@/lib/api/release-gates";
import type {
  RegressionGateRules,
  RegressionSuite,
  ReleaseGate,
  ReleaseGatePolicy,
} from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { JsonField } from "@/components/ui/json-field";
import { ShieldCheck, Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { RegressionViolationsList } from "./regression-violations-list";
import { VERDICT_CONFIG, outcomeColor } from "./verdict-config";

const DEFAULT_POLICY = JSON.stringify(
  {
    policy_key: "default",
    policy_version: 1,
    require_comparable: true,
    require_evidence_quality: true,
    fail_on_candidate_failure: true,
    fail_on_both_failed_differently: true,
    required_dimensions: ["correctness", "reliability", "latency", "cost"],
    dimensions: {
      correctness: { warn_delta: 0.02, fail_delta: 0.05 },
      reliability: { warn_delta: 0.02, fail_delta: 0.05 },
      latency: { warn_delta: 0.05, fail_delta: 0.15 },
      cost: { warn_delta: 0.1, fail_delta: 0.25 },
    },
  },
  null,
  2,
);

interface EvaluateReleaseGateDialogProps {
  workspaceId: string;
  baselineRunId: string;
  candidateRunId: string;
  candidateRunAgentId?: string;
  onEvaluated: () => void;
}

/**
 * Re-serialise a parsed policy object with normalised regression rules
 * merged in. Returns `undefined` if the input JSON is not a parseable
 * object — caller can surface that via `jsonError`.
 */
function mergeRulesIntoJson(
  raw: string,
  rules: RegressionGateRules | undefined,
): string | undefined {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return undefined;
  }
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    return undefined;
  }
  const obj = { ...(parsed as Record<string, unknown>) };
  if (rules) {
    obj.regression_gate_rules = rules;
  } else {
    delete obj.regression_gate_rules;
  }
  return JSON.stringify(obj, null, 2);
}

export function EvaluateReleaseGateDialog({
  workspaceId,
  baselineRunId,
  candidateRunId,
  candidateRunAgentId,
  onEvaluated,
}: EvaluateReleaseGateDialogProps) {
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [policyJson, setPolicyJson] = useState(DEFAULT_POLICY);
  const [rulesDraft, setRulesDraft] = useState<RegressionGateRulesDraft>(
    { ...EMPTY_REGRESSION_GATE_RULES_DRAFT },
  );
  const [jsonError, setJsonError] = useState<string>();
  const [evaluating, setEvaluating] = useState(false);
  const [apiError, setApiError] = useState<string>();
  const [result, setResult] = useState<ReleaseGate | null>(null);
  const [suites, setSuites] = useState<RegressionSuite[]>([]);
  const [suitesError, setSuitesError] = useState<string>();

  // When the structured form changes, we write the normalised rules into
  // the JSON text. `skipNextSyncRef` prevents the other direction — the
  // effect below that hydrates rules from JSON — from firing on the same
  // change, which would otherwise cause spurious re-renders.
  const skipNextSyncRef = useRef(false);

  function handleOpenChange(next: boolean) {
    setOpen(next);
    if (next) {
      setJsonError(undefined);
      setApiError(undefined);
      setResult(null);
    }
  }

  // Fetch active suites for the suite scope multi-select on open.
  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    (async () => {
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const res = await listRegressionSuites(api, workspaceId, {
          limit: 100,
        });
        if (!cancelled) {
          setSuites(
            (res.items ?? []).filter((s) => s.status === "active"),
          );
          setSuitesError(undefined);
        }
      } catch {
        if (!cancelled) {
          setSuitesError("Could not load regression suites.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, workspaceId, getAccessToken]);

  // When the user edits the JSON by hand, mirror the regression_gate_rules
  // field back into the structured form so the two views stay in sync.
  useEffect(() => {
    if (skipNextSyncRef.current) {
      skipNextSyncRef.current = false;
      return;
    }
    let parsed: unknown;
    try {
      parsed = JSON.parse(policyJson);
    } catch {
      return;
    }
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return;
    }
    const rawRules =
      (parsed as { regression_gate_rules?: RegressionGateRules })
        .regression_gate_rules;
    const nextDraft = regressionGateRulesToDraft(rawRules);
    setRulesDraft((prev) => {
      if (
        prev.noBlockingRegressionFailure === nextDraft.noBlockingRegressionFailure &&
        prev.noNewBlockingFailureVsBaseline ===
          nextDraft.noNewBlockingFailureVsBaseline &&
        prev.maxWarningRegressionFailures ===
          nextDraft.maxWarningRegressionFailures &&
        prev.suiteIds.length === nextDraft.suiteIds.length &&
        prev.suiteIds.every((id, i) => id === nextDraft.suiteIds[i])
      ) {
        return prev;
      }
      return nextDraft;
    });
  }, [policyJson]);

  function updateDraft(mutate: (draft: RegressionGateRulesDraft) => void) {
    setRulesDraft((prev) => {
      const next = { ...prev, suiteIds: [...prev.suiteIds] };
      mutate(next);
      const merged = mergeRulesIntoJson(
        policyJson,
        normalizeRegressionGateRules(next),
      );
      if (merged !== undefined) {
        skipNextSyncRef.current = true;
        setPolicyJson(merged);
      }
      return next;
    });
  }

  function toggleSuiteId(suiteId: string) {
    updateDraft((d) => {
      if (d.suiteIds.includes(suiteId)) {
        d.suiteIds = d.suiteIds.filter((id) => id !== suiteId);
      } else {
        d.suiteIds = [...d.suiteIds, suiteId];
      }
    });
  }

  async function handleEvaluate() {
    setJsonError(undefined);
    setApiError(undefined);

    let policy: ReleaseGatePolicy;
    try {
      policy = JSON.parse(policyJson) as ReleaseGatePolicy;
    } catch {
      setJsonError("Invalid JSON. Please check the syntax.");
      return;
    }

    const normalisedRules = normalizeRegressionGateRules(rulesDraft);
    if (normalisedRules) {
      policy = { ...policy, regression_gate_rules: normalisedRules };
    } else {
      const rest = { ...policy };
      delete (rest as { regression_gate_rules?: unknown }).regression_gate_rules;
      policy = rest;
    }

    setEvaluating(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const res = await evaluateReleaseGate(api, {
        baseline_run_id: baselineRunId,
        candidate_run_id: candidateRunId,
        policy,
      });
      setResult(res.release_gate);
      onEvaluated();
    } catch (err) {
      setApiError(
        err instanceof ApiError
          ? err.message
          : "Failed to evaluate release gate",
      );
    } finally {
      setEvaluating(false);
    }
  }

  const resultConfig = result
    ? VERDICT_CONFIG[result.verdict] ?? VERDICT_CONFIG.insufficient_evidence
    : null;
  const capInvalid =
    rulesDraft.maxWarningRegressionFailures != null &&
    rulesDraft.maxWarningRegressionFailures < 0;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger render={<Button variant="outline" size="sm" />}>
        <ShieldCheck className="size-4 mr-1.5" />
        Evaluate Release Gate
      </DialogTrigger>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Evaluate Release Gate</DialogTitle>
          <DialogDescription>
            Define a release gate policy and evaluate it against this
            comparison. The default policy checks all standard dimensions.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {/* Regression rules (structured) */}
          <section className="rounded-lg border border-border bg-card/30 p-3 space-y-3">
            <div>
              <p className="text-sm font-medium">Regression rules</p>
              <p className="text-xs text-muted-foreground">
                Drive the policy&apos;s <code>regression_gate_rules</code>{" "}
                block without editing JSON. Leaving everything off and the
                scope empty omits the field entirely, preserving existing
                policy fingerprints.
              </p>
            </div>

            <ToggleRow
              label="No blocking regression failure"
              description="Fail the gate when any blocking regression case fails on the candidate."
              checked={rulesDraft.noBlockingRegressionFailure}
              disabled={evaluating}
              onChange={(value) =>
                updateDraft((d) => {
                  d.noBlockingRegressionFailure = value;
                })
              }
            />

            <ToggleRow
              label="No new blocking failure vs baseline"
              description="Fail the gate only for blocking regression cases that passed on baseline but failed on candidate."
              checked={rulesDraft.noNewBlockingFailureVsBaseline}
              disabled={evaluating}
              onChange={(value) =>
                updateDraft((d) => {
                  d.noNewBlockingFailureVsBaseline = value;
                })
              }
            />

            <div>
              <label className="flex flex-col gap-1 text-xs font-medium">
                Max warning regression failures
                <Input
                  type="number"
                  min={0}
                  step={1}
                  disabled={evaluating}
                  value={
                    rulesDraft.maxWarningRegressionFailures == null
                      ? ""
                      : rulesDraft.maxWarningRegressionFailures
                  }
                  onChange={(e) => {
                    const raw = e.target.value;
                    updateDraft((d) => {
                      if (raw === "") {
                        d.maxWarningRegressionFailures = null;
                        return;
                      }
                      const n = Number.parseInt(raw, 10);
                      d.maxWarningRegressionFailures = Number.isFinite(n)
                        ? n
                        : null;
                    });
                  }}
                  className={cn(capInvalid && "border-destructive")}
                />
              </label>
              <p className="mt-1 text-2xs text-muted-foreground">
                Leave blank to disable. 0 = allow no warning failures.
              </p>
              {capInvalid && (
                <p className="mt-1 text-2xs text-destructive">
                  Must be zero or greater. Negative values are dropped at
                  submit.
                </p>
              )}
            </div>

            <div>
              <p className="text-xs font-medium mb-1">
                Suite scope{" "}
                <span className="text-muted-foreground font-normal">
                  (empty = all suites)
                </span>
              </p>
              {suitesError && (
                <p className="text-2xs text-destructive">{suitesError}</p>
              )}
              {suites.length === 0 && !suitesError ? (
                <p className="text-2xs text-muted-foreground">
                  No active regression suites in this workspace.
                </p>
              ) : (
                <div className="flex flex-wrap gap-1.5">
                  {suites.map((suite) => {
                    const active = rulesDraft.suiteIds.includes(suite.id);
                    return (
                      <button
                        key={suite.id}
                        type="button"
                        disabled={evaluating}
                        onClick={() => toggleSuiteId(suite.id)}
                        className={cn(
                          "rounded-full border px-2 py-0.5 text-xs transition-colors disabled:opacity-50",
                          active
                            ? "border-foreground bg-foreground text-background"
                            : "border-border text-muted-foreground hover:border-foreground/40 hover:text-foreground",
                        )}
                      >
                        {suite.name}
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          </section>

          <JsonField
            label="Policy JSON"
            description="Release gate policy with dimension thresholds and optional regression_gate_rules (kept in sync with the controls above)."
            value={policyJson}
            onChange={setPolicyJson}
            error={jsonError}
            disabled={evaluating}
            rows={12}
          />

          {apiError && (
            <div className="rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2 text-xs text-destructive">
              {apiError}
            </div>
          )}

          {result && resultConfig && (
            <div
              className={`rounded-lg border ${resultConfig.border} ${resultConfig.bg} px-4 py-3`}
            >
              <div className="flex items-center gap-2 mb-2">
                <Badge variant={resultConfig.variant}>
                  <resultConfig.icon
                    data-icon="inline-start"
                    className="size-3"
                  />
                  {resultConfig.label}
                </Badge>
                <span className="text-xs text-muted-foreground">
                  {result.reason_code}
                </span>
              </div>
              <p className="text-sm text-muted-foreground">{result.summary}</p>
              <div className="flex items-center gap-2 mt-1 text-xs text-muted-foreground">
                <span>
                  Evidence:{" "}
                  <span
                    className={
                      result.evidence_status === "sufficient"
                        ? "text-emerald-400"
                        : "text-amber-400"
                    }
                  >
                    {result.evidence_status}
                  </span>
                </span>
              </div>

              {/* Dimension results */}
              {result.evaluation_details?.dimension_results &&
                Object.keys(result.evaluation_details.dimension_results)
                  .length > 0 && (
                  <div className="mt-3 space-y-1">
                    <p className="text-xs font-medium">Dimension Results:</p>
                    {Object.entries(
                      result.evaluation_details.dimension_results,
                    ).map(([dim, res]) => (
                      <div
                        key={dim}
                        className="flex items-center gap-2 text-xs"
                      >
                        <span className="font-medium w-24">{dim}</span>
                        <span className={outcomeColor(res.outcome)}>
                          {res.outcome}
                        </span>
                        {res.worsening_delta != null && (
                          <span className="text-muted-foreground">
                            ({(res.worsening_delta * 100).toFixed(1)}% delta)
                          </span>
                        )}
                      </div>
                    ))}
                  </div>
                )}

              <RegressionViolationsList
                workspaceId={workspaceId}
                candidateRunId={candidateRunId}
                candidateRunAgentId={candidateRunAgentId}
                violations={
                  result.evaluation_details?.regression_violations ?? []
                }
              />
            </div>
          )}
        </div>

        <DialogFooter>
          <Button onClick={handleEvaluate} disabled={evaluating}>
            {evaluating && (
              <Loader2
                data-icon="inline-start"
                className="size-4 animate-spin"
              />
            )}
            {evaluating ? "Evaluating..." : "Evaluate"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function ToggleRow({
  label,
  description,
  checked,
  disabled,
  onChange,
}: {
  label: string;
  description: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (next: boolean) => void;
}) {
  return (
    <label className="flex gap-2 cursor-pointer select-none">
      <input
        type="checkbox"
        className="mt-0.5"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
      />
      <div>
        <p className="text-xs font-medium">{label}</p>
        <p className="text-2xs text-muted-foreground">{description}</p>
      </div>
    </label>
  );
}
