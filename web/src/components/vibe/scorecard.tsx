"use client";

import { useState } from "react";
import { ArrowRight, Bookmark, Copy, Lightbulb } from "lucide-react";
import { dollars, terminal, type CaseResult, type Operation } from "@/lib/vibe";
import { CaseEvidence } from "./case-evidence";
import { originalInputs } from "./result-prompts";
import { VibeButton } from "./vibe-button";

export function VibeScorecard({
  primary = true,
  operation,
  baseline,
  eventCursor,
  loadEvidence,
  onImprove,
  onRetest,
  onNewReplies,
  onDispute,
  onRegrade,
  onNewCheck,
  onSave,
  onRecheck,
  busy,
  testJourney = false,
}: {
  primary?: boolean;
  operation: Operation;
  baseline?: Operation;
  eventCursor?: number;
  loadEvidence: (key: string) => Promise<CaseResult>;
  onImprove: (result?: CaseResult) => void;
  onRetest: () => void;
  onNewReplies?: () => void;
  onDispute?: (rule: string, result: CaseResult) => void;
  onSave?: () => void;
  onRegrade?: () => void;
  onNewCheck?: () => void;
  onRecheck?: () => void;
  busy: boolean;
  testJourney?: boolean;
}) {
  const [copiedFinding, setCopiedFinding] = useState("");
  const score = operation.scorecard;
  if (!score) return null;
  const done = terminal(operation.state);
  const provided = operation.source?.kind === "provided_conversations";
  const unit = provided
    ? score.total === 1
      ? "chat"
      : "chats"
    : score.total === 1
      ? "example"
      : "examples";
  const regraded = operation.source?.comparison === "regraded";
  const rechecked = regraded || operation.source?.comparison === "rechecked";
  const comparable = comparableGrades(operation, baseline);

  const oldResults = new Map(
    (comparable ? baseline : undefined)?.results.map((result) => [result.case_key, result]),
  );
  const newFailures = operation.results.filter(
    (result) =>
      result.verdict === "FAIL" &&
      oldResults.get(result.case_key)?.verdict === "PASS",
  );
  const regressions = rechecked ? 0 : newFailures.length;
  const fixed = rechecked
    ? 0
    : operation.results.filter(
        (result) =>
          result.verdict === "PASS" &&
          oldResults.get(result.case_key)?.verdict === "FAIL",
      ).length;
  const incomplete = (score.incomplete_cases || 0) > 0 || score.unknown > 0;
  const stopped = ["CANCELLED", "EXPIRED", "FAILED"].includes(operation.state);
  const offerSave =
    !!onSave &&
    (!!baseline || testJourney) &&
    done &&
    !stopped &&
    !rechecked &&
    score.failed === 0 &&
    !incomplete &&
    score.total > 0;
  const retryable = operation.results.some(
    (result) => !!result.error && result.error.code !== "not_evaluated",
  );
  const leading =
    done && !(testJourney && !score.failed && !incomplete && !stopped)
      ? (!rechecked && newFailures[0]) ||
        operation.results.find((result) => result.verdict === "FAIL") ||
        operation.results.find(
          (result) =>
            result.verdict === "UNKNOWN" &&
            result.error?.code !== "not_evaluated",
        ) ||
        operation.results.find((result) => result.verdict === "PASS") ||
        operation.results[0]
      : undefined;
  const remaining = operation.results.filter((result) => result !== leading);
  const findingID = `${operation.id}:${leading?.case_key || ""}`;
  const copied = copiedFinding === findingID;
  const headline = stopped
    ? "Check stopped. Results are saved."
    : !done
      ? `Checking ${score.total} ${unit}…`
      : score.total === 0
        ? "No replies were checked."
        : rechecked
          ? regraded ? "Saved replies regraded" : "Same chats rechecked"
          : regressions
            ? `${regressions} new ${regressions === 1 ? "issue" : "issues"} in this update`
            : score.failed
              ? score.failed === 1
                ? "One thing to fix"
                : `${score.failed} things to fix`
              : incomplete
                ? "Some replies still need a closer look"
                : `No issues found in ${score.total === 1 ? `this ${unit}` : `these ${score.total} ${unit}`}`;
  const evidenceVersion = `${operation.id}:${operation.state}:${eventCursor || 0}`;

  return (
    <article aria-label="Evaluation scorecard" className="space-y-5">
      <div>
        <h1
          className="text-[28px] font-semibold leading-tight tracking-tight sm:text-[32px]"
          aria-live="polite"
        >
          {testJourney && done && !stopped && score.total > 0 && !regraded
            ? `${score.passed} of ${score.total} tests passed`
            : headline}
        </h1>
        <p className="mt-3 text-sm vibe-muted">
          {testJourney ? (
            done && score.total === 0 ? (
              "No tests produced a result."
            ) : done ? (
              `${score.failed ? `${score.failed} ${score.failed === 1 ? "needs" : "need"} attention.` : ""}${score.unknown ? ` ${score.unknown} couldn't be completed.` : score.failed ? "" : "These replies met the expectations."}`
            ) : (
              regraded ? "Rechecking grades using the saved replies…" : "Sending each test to your agent and checking its reply…"
            )
          ) : (
            <>
              {done &&
                score.failed > 0 &&
                `${score.passed} passed · ${score.failed} ${score.failed === 1 ? "needs" : "need"} attention. `}
              {done
                ? `These findings cover only ${score.total === 1 ? "this" : `these ${score.total}`} ${unit}.`
                : "Completed findings are kept as the check runs."}
            </>
          )}
        </p>
        <p className="mt-2 text-xs vibe-muted">
          {regraded ? "Saved replies only. Your agent was not called again." : provided
            ? "Based on the chats you provided. Your live app was not called."
            : "Text replies from your saved instructions. Your live app was not called."}
        </p>
        {baseline && done && !rechecked && !comparable && <p className="mt-3 text-sm vibe-muted">These runs do not have matching verified grading settings. Earlier results are kept separately.</p>}
        {rechecked && (
          <p className="mt-3 text-sm vibe-muted">
            These are the same saved replies. A different grade does not show
            that your agent improved.
          </p>
        )}
        {baseline && comparable && done && !rechecked && (
          <p
            className={`mt-3 text-sm${testJourney && regressions ? " text-builder-warn" : ""}`}
          >
            {testJourney ? (
              <>
                {fixed} fixed · {regressions} new failures. Same tests and
                evaluator.
              </>
            ) : (
              <>
                {fixed} previously failing{" "}
                {provided
                  ? fixed === 1
                    ? "chat"
                    : "chats"
                  : fixed === 1
                    ? "example"
                    : "examples"}
                {fixed === 1 ? " now passes" : " now pass"} · {regressions} new{" "}
                {regressions === 1 ? "failure" : "failures"}.
                <span className="vibe-muted">
                  {" "}
                  Same expectations and evaluator.{" "}
                  {provided
                    ? "Same customer messages."
                    : "Same example messages."}
                </span>
              </>
            )}
          </p>
        )}
        {done && incomplete && (
          <p className="mt-3 text-sm vibe-muted">
            {score.unknown > 0
              ? `${score.unknown} ${score.unknown === 1 ? "result is" : "results are"} unresolved. `
              : "Some checks are incomplete. "}
            Missing evidence is not counted as a pass.
          </p>
        )}
      </div>

      {leading && (
        <div className="vibe-panel" aria-label="Leading finding">
          <CaseEvidence
            key={findingID}
            summary={leading}
            concise={testJourney}
            evidenceVersion={evidenceVersion}
            load={loadEvidence}
            onDispute={onDispute}
            onRegrade={done ? onRegrade : undefined}
            origin={operation.source?.kind}
            defaultOpen
            focused
            allowFixPrompt={!testJourney}
            onFixCopied={() => setCopiedFinding(findingID)}
            busy={busy}
          />
        </div>
      )}

      {done && testJourney && (
        <div className="flex flex-wrap items-center gap-2">
          {score.failed > 0 && (
            <VibeButton
              variant={primary ? "primary" : "secondary"}
              disabled={busy}
              onClick={() => onImprove()}
            >
              <Lightbulb />
              Help me fix this
            </VibeButton>
          )}
          <VibeButton
            variant={primary && !offerSave && score.failed === 0 ? "primary" : "secondary"}
            disabled={busy}
            onClick={onRetest}
          >
            Run tests again
            <ArrowRight />
          </VibeButton>
          {onSave && (
            <VibeButton
              variant={primary && offerSave ? "primary" : "quiet"}
              disabled={busy}
              onClick={onSave}
            >
              <Bookmark />
              Keep these tests
            </VibeButton>
          )}
        </div>
      )}
      {done && !testJourney && (
        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            {offerSave && (
              <VibeButton variant={primary ? "primary" : "secondary"} disabled={busy} onClick={onSave}>
                <Bookmark aria-hidden="true" /> Save this check
              </VibeButton>
            )}
            {retryable && onRecheck && (
              <VibeButton
                variant={primary && !score.failed ? "primary" : "secondary"}
                disabled={busy}
                onClick={onRecheck}
              >
                Recheck these chats
                <ArrowRight aria-hidden="true" />
              </VibeButton>
            )}
            <VibeButton
              variant={
                primary && !offerSave &&
                (!score.failed || copied) &&
                !(retryable && onRecheck)
                  ? "primary"
                  : "secondary"
              }
              disabled={busy}
              onClick={onNewReplies || onRetest}
            >
              Check the new answer
              <ArrowRight aria-hidden="true" />
            </VibeButton>
            {operation.results.length > 0 && (
              <CopyOriginalInputs
                key={operation.id}
                results={operation.results}
                load={loadEvidence}
              />
            )}
          </div>
          {score.failed > 0 && (
            <p className="text-xs vibe-muted">
              Fix the behavior in your app, then bring back its new replies to
              the same messages.
            </p>
          )}
        </div>
      )}

      {remaining.length > 0 &&
        (done ? (
          <details className="text-sm">
            <summary className="cursor-pointer py-2 vibe-muted">
              {testJourney && !leading ? "Test results" : "Other results"} ·{" "}
              {remaining.length}
            </summary>
            <div className="vibe-panel mt-3" aria-label="Individual results">
              {remaining.map((result) => (
                <CaseEvidence
                  key={`${operation.id}:${result.case_key}`}
                  summary={result}
                  concise={testJourney}
                  evidenceVersion={evidenceVersion}
                  load={loadEvidence}
                  onDispute={onDispute}
                  onRegrade={done ? onRegrade : undefined}
                  origin={operation.source?.kind}
                  allowFixPrompt={!testJourney}
                  busy={busy}
                />
              ))}
            </div>
          </details>
        ) : (
          <div className="vibe-panel" aria-label="Individual results">
            {remaining.map((result) => (
              <CaseEvidence
                key={`${operation.id}:${result.case_key}`}
                summary={result}
                concise={testJourney}
                evidenceVersion={evidenceVersion}
                load={loadEvidence}
                onDispute={onDispute}
                onRegrade={done ? onRegrade : undefined}
                origin={operation.source?.kind}
                busy={busy}
              />
            ))}
          </div>
        ))}

      <details className="text-xs vibe-muted">
        <summary className="cursor-pointer py-2">What was checked</summary>
        <div className="mt-2 space-y-3 leading-6">
          <p>
            {score.passed} passed · {score.failed}{" "}
            {score.failed === 1 ? "needs" : "need"} attention · {score.unknown}{" "}
            unresolved
            {(score.incomplete_cases || 0) > score.unknown
              ? ` · ${score.incomplete_cases} with incomplete coverage`
              : ""}
          </p>
          <p>
            {regraded ? "Previously saved replies were graded again. The original replies, rules and results are unchanged." : provided
              ? `${operation.source?.label || "Provided conversations"}. Every message in the provided chats was included. No replies were generated and no live agent was called.`
              : "New text replies generated here from the saved instructions. Each example starts fresh; tools, live data and preview chats are not included."}
          </p>
          <p>
            {score.checks_evaluated ?? score.evaluated} of{" "}
            {score.checks_expected ?? score.total} expectations evaluated.
          </p>
          <p>
            {!provided && !regraded && (
              <>
                Target: {operation.models.target}
                <br />
              </>
            )}
            Evaluator: {operation.models.evaluator}
          </p>
          <p>
            {operation.billing === "RECONCILING"
              ? "Provider spend is still being reconciled."
              : `Provider spend: ${dollars(operation.actual_cost_nano_usd)}.`}
          </p>
          {done && onNewCheck && (
            <VibeButton variant="quiet" disabled={busy} onClick={onNewCheck}>
              Run as a separate check
            </VibeButton>
          )}
          {done && !testJourney && (
            <div className="flex flex-wrap gap-2">
              {score.failed > 0 && (
                <VibeButton
                  variant="quiet"
                  disabled={busy}
                  onClick={() => onImprove()}
                >
                  <Lightbulb aria-hidden="true" /> Suggest a change
                </VibeButton>
              )}
              <VibeButton variant="quiet" disabled={busy} onClick={onRetest}>
                {provided ? "Compare an update" : "Run the same examples again"}
                <ArrowRight aria-hidden="true" />
              </VibeButton>
              {onSave && !offerSave && (
                <VibeButton variant="quiet" disabled={busy} onClick={onSave}>
                  <Bookmark aria-hidden="true" /> Save this check
                </VibeButton>
              )}
            </div>
          )}
        </div>
      </details>
    </article>
  );
}

function CopyOriginalInputs({
  results,
  load,
}: {
  results: CaseResult[];
  load: (key: string) => Promise<CaseResult>;
}) {
  const [copying, setCopying] = useState(false);
  const [message, setMessage] = useState("");
  const [fallback, setFallback] = useState("");
  const copy = async () => {
    setCopying(true);
    setMessage("");
    setFallback("");
    try {
      const evidence = await Promise.all(
        results.map((result) => load(result.case_key)),
      );
      if (
        evidence.some(
          (result, index) =>
            result.case_key !== results[index].case_key ||
            result.version !== results[index].version,
        )
      )
        throw new Error(
          "The saved evidence did not match these results. Reload before copying the original inputs.",
        );
      const text = originalInputs(evidence);
      try {
        await navigator.clipboard.writeText(text);
        setMessage(
          "Original inputs copied. Keep them unchanged and replace only the agent replies.",
        );
      } catch {
        setFallback(text);
        setMessage("Select the original inputs below to copy them.");
      }
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "The original inputs could not be loaded.",
      );
    } finally {
      setCopying(false);
    }
  };
  return (
    <div className="contents">
      <VibeButton
        variant="quiet"
        disabled={copying}
        loading={copying}
        onClick={() => void copy()}
      >
        <Copy aria-hidden="true" /> Copy original inputs
      </VibeButton>
      {message && (
        <p role="status" className="w-full text-xs vibe-muted">
          {message}
        </p>
      )}
      {fallback && (
        <textarea
          className="vibe-textarea min-h-40"
          aria-label="Original inputs to copy"
          readOnly
          value={fallback}
          onFocus={(event) => event.currentTarget.select()}
        />
      )}
    </div>
  );
}

// Historical runs remain readable, but model names alone cannot establish that
// the same grading contract was used. Hashes come from the admitted server plan.
export function comparableGrades(operation: Operation, baseline?: Operation) {
  return !!baseline && operation.baseline_id === baseline.id &&
    operation.grading?.version === 1 && baseline.grading?.version === 1 &&
    !!operation.grading.hash && operation.grading.hash === baseline.grading.hash &&
    operation.source?.kind === baseline.source?.kind;
}
