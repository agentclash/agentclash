"use client";

import { useState } from "react";
import type { Artifact, CaseResult, Operation, Session } from "@/lib/vibe";
import { VibeButton } from "./vibe-button";

import { coverageProposal } from "@/lib/vibe-coverage";
import { testCount } from "@/lib/vibe-presentation";
export { coverageProposal } from "@/lib/vibe-coverage";
import { ProjectEnquiry } from "./project-enquiry";

export function EvaluationOutcome({session, artifact, operation, busy, onExport, onTougher, showTougher = false, hideContact = false}: {
  onExport?: (artifact:Artifact,operation?:Operation)=>void;
  showTougher?: boolean; hideContact?: boolean;
  session: Session; artifact: Artifact; operation: Operation; busy: boolean;
  loadEvidence: (operation: string,key: string) => Promise<CaseResult>;
  onTougher?: (request: string, count: number, artifactID: string) => void;
}) {
  const [tougher,setTougher]=useState(showTougher);
  const proposals=coverageProposal(session.rule_coverage?.[artifact.id] || []);
  if (!proposals.length && artifact.sample) proposals.push("A differently worded test decided by the existing sample assumptions");
  const completed=operation.scorecard && operation.scorecard.passed+operation.scorecard.failed>0;
  return <section className="mt-6 space-y-3" aria-label="Keep and extend your results">
    {!showTougher && <div className="flex flex-wrap gap-2">
      <VibeButton disabled={busy} onClick={() => onExport?.(artifact,operation)}>{artifact.conversation_evaluation ? "Export recorded tests" : "Export / build it myself"}</VibeButton>
      {!showTougher && completed && artifact.kind === "test_suite" && proposals.length>0 && onTougher && <VibeButton disabled={busy} onClick={() => setTougher(!tougher)}>Add harder tests</VibeButton>}
      {completed && !hideContact && <ProjectEnquiry key={artifact.id} session={session} artifact={artifact} operation={operation} />}
    </div>}
    {tougher && <div className="vibe-panel p-4 text-sm space-y-3"><p>Prepare {testCount(proposals.length, "more")}. Your existing tests and results stay intact.</p><ul className="list-disc pl-5 space-y-2">{proposals.map(x => <li key={x}>{x}</li>)}</ul><p className="vibe-muted">Next, review one maximum cost for preparing these tests and running them with your original tests.</p><VibeButton disabled={busy} onClick={() => {onTougher?.(`Add exactly ${testCount(proposals.length)} covering the following gaps or input variations, using only our existing rules. Keep every existing test, expectation and grading setting; do not invent business rules beyond this batch. ${proposals.join("; ")}`,proposals.length,artifact.id)}}>See cost and continue</VibeButton></div>}

  </section>;
}
