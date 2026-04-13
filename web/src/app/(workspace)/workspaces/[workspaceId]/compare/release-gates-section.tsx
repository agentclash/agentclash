"use client";

import { useState, useEffect } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import type {
  ReleaseGate,
  ReleaseGateVerdict,
  ListReleaseGatesResponse,
} from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import {
  Loader2,
  ChevronDown,
  ChevronRight,
} from "lucide-react";
import { EvaluateReleaseGateDialog } from "./evaluate-release-gate-dialog";
import { CiHint } from "./ci-hint";
import { VERDICT_CONFIG, outcomeColor } from "./verdict-config";

function VerdictBadge({ verdict }: { verdict: ReleaseGateVerdict }) {
  const config = VERDICT_CONFIG[verdict] ?? VERDICT_CONFIG.insufficient_evidence;
  const Icon = config.icon;
  return (
    <Badge variant={config.variant}>
      <Icon data-icon="inline-start" className="size-3" />
      {config.label}
    </Badge>
  );
}

function EvidenceBadge({ status }: { status: string }) {
  if (status === "sufficient") {
    return (
      <span className="text-xs text-emerald-400">Evidence: sufficient</span>
    );
  }
  return (
    <span className="text-xs text-amber-400">Evidence: insufficient</span>
  );
}

// --- Gate card ---

function GateCard({ gate }: { gate: ReleaseGate }) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="rounded-lg border border-border p-4">
      <div className="flex items-center justify-between mb-1">
        <div className="flex items-center gap-2">
          <VerdictBadge verdict={gate.verdict} />
          <span className="text-sm font-medium">{gate.policy_key}</span>
          <span className="text-xs text-muted-foreground">
            v{gate.policy_version}
          </span>
        </div>
        <EvidenceBadge status={gate.evidence_status} />
      </div>
      <p className="text-sm text-muted-foreground mb-1">{gate.summary}</p>
      <div className="flex items-center gap-3 text-xs text-muted-foreground">
        <span>Reason: {gate.reason_code}</span>
        <span>&middot;</span>
        <span>{new Date(gate.generated_at).toLocaleString()}</span>
      </div>

      {/* Expandable details */}
      {gate.evaluation_details && (
        <div className="mt-2">
          <button
            onClick={() => setExpanded(!expanded)}
            className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
          >
            {expanded ? (
              <ChevronDown className="size-3" />
            ) : (
              <ChevronRight className="size-3" />
            )}
            Details
          </button>
          {expanded && (
            <div className="mt-2 rounded-md bg-muted/30 px-3 py-2 text-xs space-y-1">
              {gate.evaluation_details.triggered_conditions &&
                gate.evaluation_details.triggered_conditions.length > 0 && (
                  <div>
                    <span className="font-medium">Triggered conditions:</span>
                    <ul className="list-disc list-inside ml-2 mt-0.5">
                      {gate.evaluation_details.triggered_conditions.map(
                        (c, i) => (
                          <li key={i}>{c}</li>
                        ),
                      )}
                    </ul>
                  </div>
                )}
              {gate.evaluation_details.dimension_results &&
                Object.keys(gate.evaluation_details.dimension_results).length >
                  0 && (
                  <div>
                    <span className="font-medium">Dimension results:</span>
                    <div className="mt-0.5 space-y-0.5 ml-2">
                      {Object.entries(
                        gate.evaluation_details.dimension_results,
                      ).map(([dim, result]) => (
                        <div key={dim} className="flex items-center gap-2">
                          <span className="font-medium">{dim}:</span>
                          <span className={outcomeColor(result.outcome)}>{result.outcome}</span>
                          {result.worsening_delta != null && (
                            <span className="text-muted-foreground">
                              (delta: {(result.worsening_delta * 100).toFixed(1)}
                              %)
                            </span>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              {gate.evaluation_details.warnings &&
                gate.evaluation_details.warnings.length > 0 && (
                  <div>
                    <span className="font-medium">Warnings:</span>
                    <ul className="list-disc list-inside ml-2 mt-0.5">
                      {gate.evaluation_details.warnings.map((w, i) => (
                        <li key={i}>{w}</li>
                      ))}
                    </ul>
                  </div>
                )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// --- Main section ---

interface ReleaseGatesSectionProps {
  baselineRunId: string;
  candidateRunId: string;
}

export function ReleaseGatesSection({
  baselineRunId,
  candidateRunId,
}: ReleaseGatesSectionProps) {
  const { getAccessToken } = useAccessToken();
  const [gates, setGates] = useState<ReleaseGate[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshCounter, setRefreshCounter] = useState(0);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const res = await api.get<ListReleaseGatesResponse>(
          "/v1/release-gates",
          {
            params: {
              baseline_run_id: baselineRunId,
              candidate_run_id: candidateRunId,
            },
          },
        );
        if (!cancelled) setGates(res.release_gates ?? []);
      } catch {
        // Gates are supplementary — silently fail
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [getAccessToken, baselineRunId, candidateRunId, refreshCounter]);

  function handleEvaluated() {
    setRefreshCounter((c) => c + 1);
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-sm font-semibold">Release Gates</h2>
        <EvaluateReleaseGateDialog
          baselineRunId={baselineRunId}
          candidateRunId={candidateRunId}
          onEvaluated={handleEvaluated}
        />
      </div>

      {loading ? (
        <div className="rounded-lg border border-border p-6 text-center">
          <Loader2 className="size-5 animate-spin mx-auto mb-2 text-muted-foreground" />
          <p className="text-sm text-muted-foreground">
            Loading release gates...
          </p>
        </div>
      ) : gates.length === 0 ? (
        <div className="rounded-lg border border-border bg-card p-6 text-center text-sm text-muted-foreground">
          No release gates evaluated yet. Use the button above to evaluate a
          policy against this comparison.
        </div>
      ) : (
        <div className="space-y-3">
          {gates.map((gate) => (
            <GateCard key={gate.id} gate={gate} />
          ))}
        </div>
      )}

      <CiHint
        baselineRunId={baselineRunId}
        candidateRunId={candidateRunId}
      />
    </div>
  );
}
