import type {
  AgentOpportunityReport,
  AgentOpportunityRisk,
  AgentOpportunityUseCase,
} from "@/lib/agent-opportunity";

const fitPoints: Record<AgentOpportunityUseCase["fit"], number> = {
  high: 88,
  medium: 58,
  low: 28,
};

const severityPoints: Record<AgentOpportunityRisk["severity"], number> = {
  low: 82,
  medium: 52,
  high: 24,
};

const verdictPoints: Record<AgentOpportunityReport["shouldBuildAgent"], number> = {
  strong_fit: 90,
  narrow_pilot: 72,
  eval_first: 48,
  not_yet: 22,
};

export type OpportunityMetrics = {
  workflowFit: number;
  roiSignal: number;
  riskProfile: number;
  evalReadiness: number;
};

export function deriveOpportunityMetrics(
  report: AgentOpportunityReport,
): OpportunityMetrics {
  const workflowFit =
    report.useCases.length === 0
      ? report.agentFitScore
      : Math.round(
          report.useCases.reduce((sum, useCase) => sum + fitPoints[useCase.fit], 0) /
            report.useCases.length,
        );

  const roiSignal = Math.round(
    report.agentFitScore * 0.55 + workflowFit * 0.45,
  );

  const riskProfile =
    report.risks.length === 0
      ? 55
      : Math.round(
          report.risks.reduce(
            (sum, risk) => sum + severityPoints[risk.severity],
            0,
          ) / report.risks.length,
        );

  const evalReadiness = Math.round(
    verdictPoints[report.shouldBuildAgent] * 0.6 + riskProfile * 0.4,
  );

  return { workflowFit, roiSignal, riskProfile, evalReadiness };
}

export const fitCopy: Record<AgentOpportunityReport["shouldBuildAgent"], string> = {
  not_yet: "Do not build yet",
  narrow_pilot: "Pilot narrowly",
  strong_fit: "Strong fit",
  eval_first: "Evaluate first",
};

export const fitTone: Record<
  AgentOpportunityReport["shouldBuildAgent"],
  "good" | "warn" | "bad" | "neutral"
> = {
  not_yet: "bad",
  narrow_pilot: "warn",
  strong_fit: "good",
  eval_first: "neutral",
};

export const confidenceCopy: Record<AgentOpportunityReport["fitLevel"], string> = {
  low: "Low confidence",
  moderate: "Moderate confidence",
  high: "High confidence",
};
