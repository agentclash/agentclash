import type { EvaluateDatasetGateResponse } from "@/lib/api/types";

function escapeXml(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&apos;");
}

export function buildDatasetGateJUnit(
  result: EvaluateDatasetGateResponse,
): string {
  const gate = result.gate;
  const regressions = gate.regressions ?? [];
  const failedThresholds = gate.failed_thresholds ?? [];
  const failures = regressions.length + failedThresholds.length;
  let tests = failures;
  if (tests === 0) {
    tests = 1;
  }

  const cases: string[] = [];

  for (const row of regressions) {
    const message = `${row.reason} baseline=${row.baseline_verdict ?? ""} candidate=${row.candidate_verdict ?? ""}`;
    cases.push(`    <testcase name="${escapeXml(row.dataset_example_id)}" classname="dataset-gate.${escapeXml(row.reason)}">
      <failure message="${escapeXml(row.reason)}" type="regression">${escapeXml(message)}</failure>
    </testcase>`);
  }

  for (const threshold of failedThresholds) {
    if (!threshold.trim()) continue;
    const body = `threshold ${threshold} failed (pass_rate=${gate.pass_rate} regressions=${gate.regression_count})`;
    cases.push(`    <testcase name="${escapeXml(threshold)}" classname="dataset-gate.threshold">
      <failure message="${escapeXml(threshold)}" type="threshold">${escapeXml(body)}</failure>
    </testcase>`);
  }

  if (cases.length === 0) {
    cases.push(
      `    <testcase name="dataset-gate" classname="dataset-gate" />`,
    );
  }

  return `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="${tests}" failures="${failures}">
  <testsuite name="dataset-gate" tests="${tests}" failures="${failures}">
${cases.join("\n")}
  </testsuite>
</testsuites>
`;
}

export function downloadDatasetGateJUnit(
  result: EvaluateDatasetGateResponse,
  datasetId: string,
): void {
  const xml = buildDatasetGateJUnit(result);
  const blob = new Blob([xml], { type: "application/xml" });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = `dataset-gate-${datasetId.slice(0, 8)}.xml`;
  anchor.click();
  URL.revokeObjectURL(url);
}
