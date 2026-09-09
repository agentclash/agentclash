"use client";
import { Button } from "@/components/ui/button";
import { dollars, terminal, type CaseResult, type Operation } from "@/lib/vibe";
import { CaseEvidence } from "./case-evidence";

export function VibeScorecard({
  operation,
  baseline,
  eventCursor,
  loadEvidence,
  onImprove,
  onRetest,
  busy,
}: {
  operation: Operation;
  baseline?: Operation;
  eventCursor?: number;
  loadEvidence: (key: string) => Promise<CaseResult>;
  onImprove: () => void;
  onRetest: () => void;
  busy: boolean;
}) {
  const score = operation.scorecard;
  if (!score) return null;
  const total = Math.max(score.total, 1);
  return (
    <article
      aria-label="Evaluation scorecard"
      className="rounded-2xl border border-builder-border bg-builder-surface p-5 sm:p-7"
    >
      <div className="mb-5 flex items-start justify-between gap-4">
        <div>
          <p className="font-mono text-[10px] uppercase tracking-[0.15em] text-builder-fg-muted">
            {operation.kind === "retest" ? "Retest" : "First check"}
          </p>
          <h2 className="mt-2 text-xl font-semibold tracking-tight">
            {!terminal(operation.state)
              ? "Checking your agent…"
              : score.unknown === score.total
                ? "We need more evidence"
                : score.failed
                  ? "Here’s where your agent needs work"
                  : score.unknown
                    ? "Promising, with gaps to check"
                    : "Your agent passed these examples"}
          </h2>
        </div>
        <span className="rounded-full border border-builder-border px-3 py-1 font-mono text-[10px]">
          {score.total} cases
        </span>
      </div>
      <div
        className="mb-4 flex h-3 overflow-hidden rounded-full bg-builder-surface"
        aria-label={`${score.passed} passed, ${score.failed} failed, ${score.unknown} unknown`}
      >
        <div
          className="bg-builder-fg"
          style={{ width: `${(score.passed / total) * 100}%` }}
        />
        <div
          className="bg-builder-warn"
          style={{ width: `${(score.failed / total) * 100}%` }}
        />
        <div
          className="bg-builder-fg-faint"
          style={{ width: `${(score.unknown / total) * 100}%` }}
        />
      </div>
      <div className="mb-5 grid grid-cols-3 gap-3">
        {[
          [score.passed, "Passed"],
          [score.failed, "Failed"],
          [score.unknown, "Unknown"],
        ].map(([number, label]) => (
          <div key={label}>
            <div className="text-3xl font-semibold tracking-tight tabular-nums">
              {number}
            </div>
            <div className="mt-1 text-xs text-builder-fg-muted">{label}</div>
          </div>
        ))}
      </div>
      <p className="mb-4 text-xs leading-5 text-builder-fg-muted">
        {score.evaluated} of {score.total} cases evaluated. Unknown means
        missing or incomplete evaluation, not a wrong answer. This small check
        covers these examples.
      </p>
      {score.checks_expected !== undefined && (
        <p className="mb-4 text-xs text-builder-fg-muted">
          {score.checks_evaluated} of {score.checks_expected} checks completed.
          {!!score.incomplete_cases &&
            ` ${score.incomplete_cases} cases have incomplete coverage, including any known failures.`}
        </p>
      )}
      {baseline?.scorecard && terminal(operation.state) && (
        <p className="mb-4 rounded-lg border border-builder-border p-3 text-xs leading-5">
          Same examples and evaluator: {baseline.scorecard.passed} →{" "}
          {score.passed} passed · {baseline.scorecard.failed} → {score.failed}{" "}
          failed · {baseline.scorecard.unknown} → {score.unknown} unknown.
        </p>
      )}
      <div className="divide-y divide-builder-border border-y border-builder-border">
        {operation.results.map((result) => (
          <CaseEvidence
            key={`${result.version}:${result.case_key}`}
            summary={result}
            evidenceVersion={`${operation.id}:${operation.state}:${eventCursor ?? ""}`}
            load={loadEvidence}
          />
        ))}
      </div>
      <p className="mt-4 text-xs text-builder-fg-muted">
        Agent: {operation.models.target.split("/").pop()} · Evaluator:{" "}
        {operation.models.evaluator.split("/").pop()}
      </p>
      <p className="mt-2 text-xs text-builder-fg-muted">
        {operation.billing === "RECONCILING"
          ? "Billing: reconciling. The operation’s reservation remains held until accounting is resolved."
          : `Provider spend: ${dollars(operation.actual_cost_nano_usd)}.`}
      </p>
      {terminal(operation.state) && (
        <div className="mt-5 flex flex-wrap gap-2">
          <Button onClick={onImprove} disabled={busy} size="sm">
            Improve the instructions
          </Button>
          <Button
            onClick={onRetest}
            disabled={busy}
            variant="outline"
            size="sm"
          >
            Retest the same examples
          </Button>
        </div>
      )}
    </article>
  );
}
