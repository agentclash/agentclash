"use client";

import { useState } from "react";
import { WEB_EVENTS } from "@/lib/analytics/events";
import { captureBuildEvent } from "@/lib/vibe-build-analytics";
import { terminal, type Artifact, type CaseResult, type Operation, type Session } from "@/lib/vibe";
import { CaseEvidence } from "./case-evidence";
import { EvaluationOutcome } from "./evaluation-outcome";
import { ProjectEnquiry } from "./project-enquiry";
import { VibeButton } from "./vibe-button";

export function BuildNextActions({ session, artifact, operation, hasOutput, busy, primary, onTry, onDetails, onGuide, onSave, onTougher, loadEvidence }: {
  session: Session;
  artifact: Artifact;
  operation?: Operation;
  hasOutput: boolean;
  busy: boolean;
  primary: boolean;
  onTry: () => void;
  onDetails: (operation: Operation) => void;
  onGuide: () => void;
  onSave: (operation: Operation) => void;
  onTougher?: (text: string, count: number, artifactID: string) => void;
  loadEvidence: (id: string, key: string) => Promise<CaseResult>;
}) {
  const [keepOpen, setKeepOpen] = useState(false);
  const [sampleOpen, setSampleOpen] = useState(false);
  const [tougherOpen, setTougherOpen] = useState(false);
  const score = operation?.scorecard;
  const done = !!operation && terminal(operation.state);
  const allPass = done && operation.state === "COMPLETED" && !operation.error && !!score && score.total > 0
    && operation.results.length === score.total && operation.results.every(item => item.verdict === "PASS")
    && score.passed === score.total && score.evaluated === score.total && !score.failed && !score.unknown && !score.incomplete_cases;
  const brief = artifact.kind === "task_brief";
  const stopped = operation?.state === "CANCELLED";
  const contactEligible = brief || hasOutput || allPass || !!score?.failed || !!score?.unknown;
  const firstSample = operation?.results[0];
  const rules = artifact.sample_basis?.rules || session.document.policies?.find(policy => policy.id === artifact.policy_id)?.rules || [];
  const eventBase = { session_id: session.id, artifact_id: artifact.id, operation_id: operation?.id, sample: !!artifact.sample };
  function action(name: "try" | "checks" | "rules" | "change" | "tougher" | "save" | "review_problem") {
    captureBuildEvent(WEB_EVENTS.VIBE_BUILD_ACTION_CLICKED, { ...eventBase, action: name });
  }
  // Keep the contact owner mounted while checks run so reviewed drafts and
  // receipts survive a new result. Stop/recovery stays with the operation UI.
  const available = !busy && !(operation && !done) && !stopped;
  return <section className="vibe-build-next-actions" aria-label="Next actions for this version" data-artifact-id={artifact.id} hidden={!available}>
    <div className="vibe-build-next-primary">
      {brief ? <VibeButton variant={primary ? "primary" : "secondary"} onClick={() => { action("change"); onGuide(); }}>Add missing information</VibeButton>
        : operation && !allPass ? <VibeButton variant={primary ? "primary" : "secondary"} onClick={() => { action(score?.failed ? "review_problem" : "checks"); onDetails(operation); }}>{score?.failed ? "Review the problem" : "See what happened"}</VibeButton>
          : !hasOutput ? <VibeButton variant={primary ? "primary" : "secondary"} onClick={() => { action("try"); onTry(); }}>Try it yourself</VibeButton>
            : null}
      <div hidden={!contactEligible}><ProjectEnquiry key={`${session.id}:${artifact.id}`} session={session} artifact={artifact} operation={operation} primary={hasOutput && (allPass || !operation)} /></div>
    </div>
    {!brief && <div className="vibe-build-next-secondary">
      {!hasOutput && operation && !allPass && <VibeButton variant="quiet" onClick={() => { action("try"); onTry(); }}>Try it yourself</VibeButton>}
      {operation && allPass && <VibeButton variant="quiet" onClick={() => { action("checks"); onDetails(operation); }}>See {operation.results.length} {operation.results.length === 1 ? "check" : "checks"}</VibeButton>}
      {hasOutput && <VibeButton variant="quiet" onClick={() => { action("change"); onGuide(); }}>Change how it works</VibeButton>}
      {operation && <VibeButton variant="quiet" aria-expanded={keepOpen} onClick={() => { action("save"); setKeepOpen(value => !value); }}>Download / save</VibeButton>}
    </div>}
    {!brief && <details className="vibe-build-more" onToggle={event => { if (event.currentTarget.open) action("rules"); }}><summary>More ways to check</summary>
      <div className="vibe-build-more-content">
        {hasOutput && <VibeButton variant="quiet" onClick={() => { action("try"); onTry(); }}>Try another input</VibeButton>}
        <details className="vibe-prototype-details"><summary>Rules used</summary><p>{artifact.scope_note || "Only supplied text was available."}</p>{artifact.sample && <p>These are sample rules, not your business policy.</p>}{rules.length > 0 && <ul className="vibe-scope-rules">{rules.map(rule => <li key={rule.id}>{rule.statement}</li>)}</ul>}</details>
        {operation && !hasOutput && allPass && firstSample && <><VibeButton variant="quiet" aria-expanded={sampleOpen} onClick={() => setSampleOpen(value => !value)}>Try a sample</VibeButton>{sampleOpen && <section aria-label="Saved sample demonstration"><p className="vibe-build-note">A made-up input and the saved reply from this check. Opening it does not run the model.</p><CaseEvidence summary={firstSample} exampleFirst concise compactPreview defaultOpen load={key => loadEvidence(operation.id, key)} /></section>}</>}
        {operation && onTougher && <><VibeButton variant="quiet" aria-expanded={tougherOpen} onClick={() => { action("tougher"); setTougherOpen(value => !value); }}>Try tougher situations</VibeButton>{tougherOpen && <EvaluationOutcome key="extend" showTougher hideContact session={session} artifact={artifact} operation={operation} busy={busy} loadEvidence={loadEvidence} onTougher={onTougher} />}</>}
      </div>
    </details>}
    {keepOpen && operation && <section aria-label="Download or save your work"><VibeButton disabled={busy} onClick={() => onSave(operation)}>Save to my account</VibeButton><EvaluationOutcome hideContact session={session} artifact={artifact} operation={operation} busy={busy} loadEvidence={loadEvidence} /></section>}
  </section>;
}
