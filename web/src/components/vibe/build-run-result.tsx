"use client";

import { useEffect, useRef } from "react";
import { Lightbulb } from "lucide-react";
import { WEB_EVENTS } from "@/lib/analytics/events";
import { captureBuildEvent } from "@/lib/vibe-build-analytics";
import { terminal, type Artifact, type CaseResult, type Operation, type Session } from "@/lib/vibe";
import { buildRunSummary } from "@/lib/vibe-build-timeline";
import { CaseEvidence } from "./case-evidence";
import { comparableGrades } from "./scorecard";
import { VibeButton } from "./vibe-button";
import { suiteCases } from "./test-suite-cases";
import { resultHeadline } from "./result-headline";

export function BuildRunResult({ session, operation, artifact, current, first, busy, loadEvidence, onImprove, onDetails, onTry, headingLevel = 2 }: {
  headingLevel?: 1 | 2;
  session: Session;
  operation: Operation;
  artifact?: Artifact;
  current: boolean;
  first: boolean;
  busy: boolean;
  loadEvidence: (id: string, key: string) => Promise<CaseResult>;
  onImprove: () => void;
  onDetails: () => void;
  onTry?: () => void;
}) {
  const Heading = headingLevel === 1 ? "h1" : "h2";
  const hasOutput = session.document.messages.some(message => message.origin === "playground" && message.role === "assistant" && message.artifact_id === artifact?.id);
  const card = useRef<HTMLElement>(null);
  const visible = useRef(false);
  const loadedCase = useRef<string | undefined>(undefined);
  const score = operation.scorecard;
  const inputs = new Map(suiteCases(artifact?.blueprint).map(c => [c.key, c.input]));
  const done = terminal(operation.state);
  const leading = operation.results.find(c => c.verdict === "FAIL") || operation.results.find(c => c.verdict === "UNKNOWN") || operation.results[0];
  const leadingOrdinal = leading ? operation.results.indexOf(leading) + 1 : 0;
  const baseline = session.operations.find(o => o.id === operation.baseline_id);
  const comparable = comparableGrades(operation, baseline);
  const regraded = operation.source?.comparison === "regraded" || operation.source?.comparison === "rechecked";
  const oldVerdicts = new Map(baseline?.results.map(c => [c.case_key, c.verdict]));
  const improved = comparable ? operation.results.filter(c => c.verdict === "PASS" && oldVerdicts.get(c.case_key) === "FAIL").length : 0;
  const regressed = comparable ? operation.results.filter(c => c.verdict === "FAIL" && oldVerdicts.get(c.case_key) === "PASS").length : 0;
  const allPass = done && operation.state === "COMPLETED" && !operation.error && !!score && score.total > 0
    && operation.results.length === score.total && operation.results.every(c => c.verdict === "PASS")
    && score.passed === score.total && score.evaluated === score.total && !score.failed && !score.unknown && !score.incomplete_cases;
  const canImprove = done && current && !operation.error && !!score?.failed && !regraded;
  const complete = done && operation.state === "COMPLETED" && !operation.error && !!score && score.total > 0
    && score.evaluated === score.total && !score.unknown && !score.incomplete_cases && operation.results.length === score.total;
  const headline = complete && !regraded ? resultHeadline(score!.passed, score!.total) : buildRunSummary(operation);
  const eventBase = { session_id: session.id, operation_id: operation.id, artifact_id: artifact?.id, sample: !!artifact?.sample };
  function action(name: "checks" | "review_fix") {
    captureBuildEvent(WEB_EVENTS.VIBE_BUILD_ACTION_CLICKED, { ...eventBase, action: name });
  }
  useEffect(() => {
    if (!done || !card.current) return;
    const report = () => {
      visible.current = true;
      const base = { session_id: session.id, operation_id: operation.id, artifact_id: artifact?.id, sample: !!artifact?.sample };
      captureBuildEvent(WEB_EVENTS.VIBE_BUILD_RESULT_VIEWED, { ...base, outcome: !score || operation.state !== "COMPLETED" || operation.error ? "incomplete" : score.failed ? "failure" : allPass ? "all_pass" : "unassessed", passed: score?.passed || 0, failed: score?.failed || 0, unknown: score?.unknown || 0 }, `${session.id}:${operation.id}:${operation.state}`);
      if (loadedCase.current) captureBuildEvent(WEB_EVENTS.VIBE_BUILD_EXAMPLE_VIEWED, { ...base, case_ordinal: leadingOrdinal }, `${session.id}:${operation.id}:${loadedCase.current}`);
    };
    if (typeof IntersectionObserver === "undefined") { report(); return; }
    const observer = new IntersectionObserver(entries => { if (entries.some(entry => entry.isIntersecting)) report(); }, { threshold: 0.1 });
    observer.observe(card.current);
    return () => { visible.current = false; observer.disconnect(); };
  }, [session.id, operation.id, operation.state, operation.error, score, artifact?.id, artifact?.sample, allPass, done, leadingOrdinal]);
  return <article ref={card} className="vibe-build-result" aria-label="Agent test results" data-run-id={operation.id}>
    <div role="status"><Heading tabIndex={-1} className="vibe-results-outcome">{headline}</Heading></div>
    {artifact?.sample && <p className="vibe-build-note">Sample demonstration. These results test the sample, not your business policy.</p>}
    {operation.source?.kind === "provided_conversations" && <p className="vibe-build-note">Recorded replies only; your live app wasn’t called.</p>}
    {(allPass || hasOutput) && <p className="vibe-build-note">{hasOutput && "These sample tests don’t grade your separate trial output. "}{allPass && "Other inputs remain untested."}</p>}
    {baseline && done && <p className="vibe-build-note">{regraded
      ? "Only the grades changed. This does not show an improved agent."
      : comparable
        ? `Same tests and grading · ${improved} improved · ${regressed} regressed. ${baseline.models.target === operation.models.target ? "Earlier results are kept." : "The target model also changed, so the difference cannot be credited to the prompt alone."}`
        : "Separate baseline: the tests or grading differ. These scores do not establish an improvement."}</p>}
    {current && leading && <div className="vibe-build-examples">
      {first && <p className="vibe-test-explanation">We gave it an input and graded its reply against the rules. That’s a test.</p>}
      <CaseEvidence key={leading.case_key} summary={{ ...leading, title: leading.title || inputs.get(leading.case_key) }} concise compactPreview
        defaultOpen={current} prefetch={current}
        onReload={() => captureBuildEvent(WEB_EVENTS.VIBE_BUILD_RECOVERY_CLICKED, { ...eventBase, action: "reload_evidence", error_code: leading.error?.code })}
        evidenceVersion={`${operation.id}:${operation.state}`} load={key => loadEvidence(operation.id, key)}
        origin={operation.source?.kind}
        onEvidence={evidence => {
          loadedCase.current = evidence.case_key;
          if (visible.current) captureBuildEvent(WEB_EVENTS.VIBE_BUILD_EXAMPLE_VIEWED, { ...eventBase, case_ordinal: leadingOrdinal }, `${session.id}:${operation.id}:${evidence.case_key}`);
        }} />
    </div>}
    {current && canImprove && <div className="vibe-build-evidence-action"><VibeButton variant="primary" disabled={busy} onClick={() => { action("review_fix"); onImprove(); }}><Lightbulb aria-hidden="true" />Suggest a fix</VibeButton></div>}
    {current && operation.results.length > 1 && <details className="vibe-other-results"><summary>Other results · {operation.results.length - 1}</summary>
      {operation.results.filter(result => result !== leading).map(result => <CaseEvidence key={result.case_key} summary={{ ...result, title: result.title || inputs.get(result.case_key) }} concise evidenceVersion={`${operation.id}:${operation.state}`} load={key => loadEvidence(operation.id, key)} origin={operation.source?.kind} busy={busy} />)}
    </details>}
    {!hasOutput && <p className="vibe-build-note">Runs here using what you supplied; your business systems aren’t connected.</p>}

    {current && allPass && onTry && <VibeButton variant="primary" disabled={busy} onClick={onTry}>Try your agent</VibeButton>}
    {!current && done && <VibeButton variant="quiet" onClick={() => { action("checks"); onDetails(); }}>See saved results</VibeButton>}

  </article>;
}

export function BuildAdmissionError({ message, busy, onRun, notice }: { notice?: import("react").ReactNode; message: string; busy: boolean; onRun: () => void }) {
  return <section className="vibe-build-result" aria-label="Build needs a run"><h2>Your agent is saved</h2><p role="alert">{message}</p><VibeButton variant="primary" disabled={busy} onClick={onRun}>Run 3 tests</VibeButton>{notice}</section>;
}
