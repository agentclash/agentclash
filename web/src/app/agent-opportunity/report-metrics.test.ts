import { describe, expect, it } from "vitest";
import type { AgentOpportunityReport } from "@/lib/agent-opportunity";
import { deriveOpportunityMetrics, fitCopy } from "./report-metrics";

const sampleReport: AgentOpportunityReport = {
  analyzedUrl: "https://example.com/",
  companyName: "Example",
  generatedAt: "2026-06-12T00:00:00.000Z",
  agentFitScore: 72,
  fitLevel: "moderate",
  shouldBuildAgent: "narrow_pilot",
  honestVerdict: "Pilot one workflow first.",
  summary: "Support-heavy business.",
  useCases: [
    {
      title: "Support triage",
      workflow: "Route tickets",
      fit: "high",
      estimatedMonthlyHoursSaved: "20-40",
      estimatedMonthlySavingsUsd: "$1,500-$4,000",
      complexity: "medium",
      why: "Clear support workflows.",
      firstEvalTasks: ["Refund", "Pricing"],
    },
  ],
  risks: [
    {
      risk: "Wrong answer",
      severity: "medium",
      mitigation: "Escalate low confidence.",
    },
    {
      risk: "Policy miss",
      severity: "high",
      mitigation: "Evaluate edge cases.",
    },
  ],
  evaluationPack: {
    name: "Support starter",
    recommendedCases: 25,
    adversarialCases: 8,
    successCriteria: ["90% routing"],
  },
  nextSteps: ["Collect tickets"],
  evidenceLimitations: ["Public pages only."],
};

describe("deriveOpportunityMetrics", () => {
  it("derives bounded dashboard metrics from a report", () => {
    const metrics = deriveOpportunityMetrics(sampleReport);
    expect(metrics.workflowFit).toBe(88);
    expect(metrics.roiSignal).toBeGreaterThan(0);
    expect(metrics.roiSignal).toBeLessThanOrEqual(100);
    expect(metrics.riskProfile).toBe(38);
    expect(metrics.evalReadiness).toBeGreaterThan(0);
  });
});

describe("fitCopy", () => {
  it("maps verdict enums to readable labels", () => {
    expect(fitCopy.narrow_pilot).toBe("Pilot narrowly");
  });
});
