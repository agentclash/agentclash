"use client";

import { useState } from "react";
import { ArrowRight } from "lucide-react";
import { terminal, type Artifact, type CaseResult, type Operation, type Session } from "@/lib/vibe";
import { buildRunSummary, buildVersion } from "@/lib/vibe-build-timeline";
import { CaseEvidence } from "./case-evidence";
import { verifiedFinding } from "./grounded-finding";
import { comparableGrades } from "./scorecard";
import { VibeButton } from "./vibe-button";
import { EvaluationOutcome } from "./evaluation-outcome";
import { suiteCases } from "./test-suite-cases";

export function BuildRunResult({ session, operation, artifact, current, first, primary, busy, loadEvidence, onTry, onImprove, onDetails, onAsk, onSave, onTougher }: {
  session: Session;
  operation: Operation;
  artifact?: Artifact;
  current: boolean;
  first: boolean;
  primary: boolean;
  busy: boolean;
  loadEvidence: (id: string, key: string) => Promise<CaseResult>;
  onTry: () => void;
  onImprove: () => void;
  onDetails: () => void;
  onAsk: () => void;
  onSave: () => void;
  onTougher?: (text: string, count: number, artifactID: string) => void;
}) {
  const [extend, setExtend] = useState(false);
  const [finding, setFinding] = useState<{ text: string; supported: boolean }>();
  const score = operation.scorecard;
  const inputs = new Map(suiteCases(artifact?.blueprint).map(c => [c.key, c.input]));
  const done = terminal(operation.state);
  const leading = operation.results.find(c => c.verdict === "FAIL") || operation.results[0];
  const baseline = session.operations.find(o => o.id === operation.baseline_id);
  const comparable = comparableGrades(operation, baseline);
  const regraded = operation.source?.comparison === "regraded" || operation.source?.comparison === "rechecked";
  const oldVerdicts = new Map(baseline?.results.map(c => [c.case_key, c.verdict]));
  const improved = comparable ? operation.results.filter(c => c.verdict === "PASS" && oldVerdicts.get(c.case_key) === "FAIL").length : 0;
  const regressed = comparable ? operation.results.filter(c => c.verdict === "FAIL" && oldVerdicts.get(c.case_key) === "PASS").length : 0;
  const allPass = done && operation.state === "COMPLETED" && !operation.error && !!score && score.total > 0
    && score.passed === score.total && score.evaluated >= score.total && !score.unknown && !score.incomplete_cases;
  const canImprove = done && current && !operation.error && !!score?.failed && finding?.supported && !regraded;
  const headline = done && operation.state === "COMPLETED" && !operation.error && finding?.text && !regraded
    ? finding.text : buildRunSummary(operation);
  return <article className="vibe-build-result" aria-label="Prototype example results" data-run-id={operation.id}>
    <p className="vibe-build-eyebrow">{artifact ? `Interactive prototype · v${buildVersion(session, artifact)}` : "Saved check"}{artifact?.sample ? " · Sample demonstration" : ""}</p>
    {artifact && <h2 className="vibe-build-task">{artifact.title}</h2>}
    <p className="vibe-build-finding" role="status">{headline}</p>
    {score && <p className="vibe-build-counts">{score.passed} passed{score.failed > 0 && ` · ${score.failed} need attention`}{score.unknown > 0 && ` · ${score.unknown} could not be assessed`}</p>}
    {artifact?.sample
      ? <p className="vibe-build-note">These results check the sample, not your business policy.</p>
      : <p className="vibe-build-note">{operation.source?.kind === "provided_conversations" ? "Recorded replies only; your live app wasn’t called." : "Your business systems aren’t connected. This only checks the version running here."}</p>}
    {allPass && <p className="vibe-build-note">Other situations remain untested.</p>}
    {baseline && done && <p className="vibe-build-note">{regraded
      ? "Only the grades changed. This does not show an improved prototype."
      : comparable
        ? `Same examples and grading · ${improved} improved · ${regressed} regressed. Earlier results are kept.`
        : "Separate baseline: the examples or grading differ. These scores do not establish an improvement."}</p>}
    {done && <div className="vibe-build-actions">
      {current && allPass && artifact?.agent_prompt && <VibeButton variant={primary ? "primary" : "secondary"} disabled={busy} onClick={onTry}>{/email|spam/i.test(artifact.title) ? "Try an email" : "Try it yourself"} <ArrowRight /></VibeButton>}
      {canImprove && <VibeButton variant={primary ? "primary" : "secondary"} disabled={busy} onClick={onImprove}>Review a fix <ArrowRight /></VibeButton>}
      {current && !allPass && !canImprove && <VibeButton variant={primary ? "primary" : "secondary"} onClick={onDetails}>See what happened <ArrowRight /></VibeButton>}
      {(!current || allPass || canImprove) && <VibeButton variant="quiet" onClick={onDetails}>View details</VibeButton>}
      <VibeButton variant="quiet" onClick={onAsk}>Ask about this result</VibeButton>
      {current && onTougher && <VibeButton variant="secondary" disabled={busy} onClick={() => setExtend(!extend)}>Try tougher situations</VibeButton>}
      {current && <VibeButton variant="quiet" onClick={() => document.getElementById(`keep-${operation.id}`)?.setAttribute("open", "")}>Save / export</VibeButton>}
    </div>}
    {first && operation.results.length > 0 && <p className="vibe-test-explanation">Here’s what we gave it and whether its reply followed the rules. Open a row to see exactly what happened.</p>}
    {operation.results.length > 0 && <div className="vibe-build-examples">
      {operation.results.map(c => <CaseEvidence key={c.case_key} summary={{ ...c, title: c.title || inputs.get(c.case_key) }} exampleFirst concise
        defaultOpen={false} prefetch={current && c.verdict === "FAIL" && c.case_key === leading?.case_key}
        evidenceVersion={`${operation.id}:${operation.state}`} load={key => loadEvidence(operation.id, key)}
        origin={operation.source?.kind}
        onEvidence={c.case_key === leading?.case_key ? evidence => {
          const failed = evidence.checks.find(check => check.verdict === "FAIL" && verifiedFinding(evidence, check));
          if (failed) setFinding({ text: failed.evidence.length > 220 ? failed.evidence.slice(0, 217) + "…" : failed.evidence, supported: true });
        } : undefined} />)}
    </div>}
    {current && extend && artifact && <EvaluationOutcome key="extend" showTougher session={session} artifact={artifact} operation={operation} busy={busy} loadEvidence={loadEvidence} onTougher={onTougher} />}
    {current && done && artifact && <details id={`keep-${operation.id}`} className="vibe-keep-menu">
      <summary>Save, export and next steps</summary>
      <p className="vibe-build-note">Keep the instructions, checks and results. This does not deploy or connect the agent.</p>
      <div className="vibe-keep-content">
        <VibeButton disabled={busy} onClick={onSave}>Keep these tests</VibeButton>
        <EvaluationOutcome session={session} artifact={artifact} operation={operation} busy={busy} loadEvidence={loadEvidence} />
      </div>
    </details>}
  </article>;
}
