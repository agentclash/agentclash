"use client";

import { useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  ReleaseGate,
  EvaluateReleaseGateResponse,
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
import { JsonField } from "@/components/ui/json-field";
import { ShieldCheck, Loader2 } from "lucide-react";
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
  baselineRunId: string;
  candidateRunId: string;
  onEvaluated: () => void;
}

export function EvaluateReleaseGateDialog({
  baselineRunId,
  candidateRunId,
  onEvaluated,
}: EvaluateReleaseGateDialogProps) {
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [policyJson, setPolicyJson] = useState(DEFAULT_POLICY);
  const [jsonError, setJsonError] = useState<string>();
  const [evaluating, setEvaluating] = useState(false);
  const [apiError, setApiError] = useState<string>();
  const [result, setResult] = useState<ReleaseGate | null>(null);

  function handleOpenChange(next: boolean) {
    setOpen(next);
    if (next) {
      // Reset state when opening
      setJsonError(undefined);
      setApiError(undefined);
      setResult(null);
    }
  }

  async function handleEvaluate() {
    setJsonError(undefined);
    setApiError(undefined);

    let policy: unknown;
    try {
      policy = JSON.parse(policyJson);
    } catch {
      setJsonError("Invalid JSON. Please check the syntax.");
      return;
    }

    setEvaluating(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const res = await api.post<EvaluateReleaseGateResponse>(
        "/v1/release-gates/evaluate",
        {
          baseline_run_id: baselineRunId,
          candidate_run_id: candidateRunId,
          policy,
        },
      );
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

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger render={<Button variant="outline" size="sm" />}>
        <ShieldCheck className="size-4 mr-1.5" />
        Evaluate Release Gate
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Evaluate Release Gate</DialogTitle>
          <DialogDescription>
            Define a release gate policy and evaluate it against this
            comparison. The default policy checks all standard dimensions.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <JsonField
            label="Policy JSON"
            description="Release gate policy with dimension thresholds. Leave as default for standard evaluation."
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
