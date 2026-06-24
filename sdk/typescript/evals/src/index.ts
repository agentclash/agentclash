export type AgentEvalResult = {
  input: string;
  output?: string;
  toolCalls?: Array<{ name: string; arguments?: Record<string, unknown> }>;
  metadata?: { latencyMs?: number; costUsd?: number };
};

export type MetricResult = {
  key: string;
  name: string;
  passed: boolean;
  reason?: string;
  evidence?: Record<string, unknown>;
};

export type EvalReport = {
  schema_version: 1;
  report_id: string;
  generated_at: string;
  runner: { name: "@agentclash/evals"; version: string; language: "typescript" };
  summary: {
    total: number;
    passed: number;
    failed: number;
    skipped: number;
    errored: number;
    metric_failures: number;
  };
  cases: Array<{
    case: { case_id: string; name: string; input?: string };
    status: "passed" | "failed";
    metrics: MetricResult[];
    agent_result?: AgentEvalResult;
  }>;
  failures: Array<{ case_id: string; metric_key: string; reason: string }>;
  exit_code: 0 | 1;
};

export class Contains {
  constructor(private expected: string, private key = "contains") {}
  evaluate(result: AgentEvalResult): MetricResult {
    const output = result.output ?? "";
    const passed = output.includes(this.expected);
    return {
      key: this.key,
      name: "Contains",
      passed,
      reason: passed ? `output contains ${this.expected}` : `missing ${this.expected}`,
      evidence: { expected: this.expected },
    };
  }
}

export async function evaluate(
  result: string | AgentEvalResult,
  metrics: Array<{ evaluate: (result: AgentEvalResult) => MetricResult | Promise<MetricResult> }>,
): Promise<EvalReport> {
  const agentResult: AgentEvalResult =
    typeof result === "string" ? { input: result, output: result } : result;
  const metricResults = await Promise.all(metrics.map((metric) => metric.evaluate(agentResult)));
  const failures = metricResults.filter((metric) => !metric.passed);
  return {
    schema_version: 1,
    report_id: `rpt-${Math.random().toString(16).slice(2, 10)}`,
    generated_at: new Date().toISOString(),
    runner: { name: "@agentclash/evals", version: "0.1.0-alpha", language: "typescript" },
    summary: {
      total: 1,
      passed: failures.length ? 0 : 1,
      failed: failures.length ? 1 : 0,
      skipped: 0,
      errored: 0,
      metric_failures: failures.length,
    },
    cases: [
      {
        case: { case_id: "eval_case", name: "eval_case", input: agentResult.input },
        status: failures.length ? "failed" : "passed",
        metrics: metricResults,
        agent_result: agentResult,
      },
    ],
    failures: failures.map((metric) => ({
      case_id: "eval_case",
      metric_key: metric.key,
      reason: metric.reason ?? "metric failed",
    })),
    exit_code: failures.length ? 1 : 0,
  };
}

export async function assertAgent(
  result: string | AgentEvalResult,
  metrics: Array<{ evaluate: (result: AgentEvalResult) => MetricResult | Promise<MetricResult> }>,
): Promise<EvalReport> {
  const report = await evaluate(result, metrics);
  if (report.exit_code !== 0) {
    throw new Error(report.failures.map((f) => `${f.metric_key}: ${f.reason}`).join("; "));
  }
  return report;
}
