"use client";

import { useEffect, useRef, useState } from "react";
import { WEB_EVENTS } from "@/lib/analytics/events";
import { captureBuildEvent } from "@/lib/vibe-build-analytics";
import { terminal, type Artifact, type CaseResult, type Operation, type Session } from "@/lib/vibe";
import { buildRunSummary, buildVersion } from "@/lib/vibe-build-timeline";
import { CaseEvidence } from "./case-evidence";
import { verifiedFinding } from "./grounded-finding";
import { comparableGrades } from "./scorecard";
import { VibeButton } from "./vibe-button";
import { suiteCases } from "./test-suite-cases";

export function BuildRunResult({ session, operation, artifact, current, first, busy, loadEvidence, onImprove, onDetails }: {
  session: Session;
  operation: Operation;
  artifact?: Artifact;
  current: boolean;
  first: boolean;
  busy: boolean;
  loadEvidence: (id: string, key: string) => Promise<CaseResult>;
  onImprove: () => void;
  onDetails: () => void;
}) {
  const hasOutput = session.document.messages.some(message => message.origin === "playground" && message.role === "assistant" && message.artifact_id === artifact?.id);
  const [finding, setFinding] = useState<{ text: string; supported: boolean }>();
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
  const canImprove = done && current && !operation.error && !!score?.failed && finding?.supported && !regraded;
  const headline = done && operation.state === "COMPLETED" && !operation.error && finding?.text && !regraded
    ? finding.text : allPass && !regraded ? score!.total === 1
      ? `This ${artifact?.sample ? "sample check matched the sample rules" : "check matched your rules"}.`
      : `All ${score!.total} ${artifact?.sample ? "sample checks matched the sample rules" : "checks matched your rules"}.` : buildRunSummary(operation);
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
  return <article ref={card} className="vibe-build-result" aria-label="Prototype example results" data-run-id={operation.id}>
    {!hasOutput && <><p className="vibe-build-eyebrow">{artifact ? `Interactive prototype · v${buildVersion(session, artifact)}` : "Saved check"}{artifact?.sample ? " · Sample demonstration" : ""}</p>
    {artifact && <h2 className="vibe-build-task">{artifact.title}</h2>}
    <p className="vibe-build-note">Runs here using what you supplied; your business systems aren’t connected.</p>
    </>}
    <p className="vibe-build-finding" role="status">{headline}</p>
    {artifact?.sample && <p className="vibe-build-note">These results check the sample, not your business policy.</p>}
    {operation.source?.kind === "provided_conversations" && <p className="vibe-build-note">Recorded replies only; your live app wasn’t called.</p>}
    {(allPass || hasOutput) && <p className="vibe-build-note">{hasOutput && "These sample checks don’t grade your separate trial output. "}{allPass && "Other situations remain untested."}</p>}
    {baseline && done && <p className="vibe-build-note">{regraded
      ? "Only the grades changed. This does not show an improved prototype."
      : comparable
        ? `Same examples and grading · ${improved} improved · ${regressed} regressed. ${baseline.models.target === operation.models.target ? "Earlier results are kept." : "The target model also changed, so the difference cannot be credited to the prompt alone."}`
        : "Separate baseline: the examples or grading differ. These scores do not establish an improvement."}</p>}
    {current && leading && !allPass && <div className="vibe-build-examples">
      {first && <p className="vibe-test-explanation">We gave it a situation and checked its reply against the rules. That’s a check.</p>}
      <CaseEvidence key={leading.case_key} summary={{ ...leading, title: leading.title || inputs.get(leading.case_key) }} exampleFirst concise compactPreview
        prefetch={false}
        onReload={() => captureBuildEvent(WEB_EVENTS.VIBE_BUILD_RECOVERY_CLICKED, { ...eventBase, action: "reload_evidence", error_code: leading.error?.code })}
        evidenceVersion={`${operation.id}:${operation.state}`} load={key => loadEvidence(operation.id, key)}
        origin={operation.source?.kind}
        onEvidence={evidence => {
          const failed = evidence.checks.find(check => check.verdict === "FAIL" && verifiedFinding(evidence, check));
          if (failed) setFinding({ text: failed.evidence.length > 220 ? failed.evidence.slice(0, 217) + "…" : failed.evidence, supported: true });
          loadedCase.current = evidence.case_key;
          if (visible.current) captureBuildEvent(WEB_EVENTS.VIBE_BUILD_EXAMPLE_VIEWED, { ...eventBase, case_ordinal: leadingOrdinal }, `${session.id}:${operation.id}:${evidence.case_key}`);
        }} />
    </div>}
    {current && canImprove && <div className="vibe-build-evidence-action"><VibeButton variant="quiet" disabled={busy} onClick={() => { action("review_fix"); onImprove(); }}>Review a fix</VibeButton></div>}
    {!current && done && <VibeButton variant="quiet" onClick={() => { action("checks"); onDetails(); }}>See saved checks</VibeButton>}

  </article>;
}
