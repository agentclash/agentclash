"use client";

import { downloadJSON, exportRuns } from "@/lib/vibe-export";
import { useState } from "react";
import type { Artifact, CaseResult, Operation, Session } from "@/lib/vibe";
import { VibeButton } from "./vibe-button";

import { coverageProposal } from "@/lib/vibe-coverage";
export { coverageProposal } from "@/lib/vibe-coverage";
import { ProjectEnquiry } from "./project-enquiry";

export function EvaluationOutcome({session, artifact, operation, busy, loadEvidence, onTougher, showTougher = false, hideContact = false}: {
  showTougher?: boolean; hideContact?: boolean;
  session: Session; artifact: Artifact; operation: Operation; busy: boolean;
  loadEvidence: (operation: string,key: string) => Promise<CaseResult>;
  onTougher?: (request: string, count: number, artifactID: string) => void;
}) {
  const [working,setWorking]=useState(false);
  const [error,setError]=useState("");
  const [tougher,setTougher]=useState(showTougher);
  const proposals=coverageProposal(session.rule_coverage?.[artifact.id] || []);
  if (!proposals.length && artifact.sample) proposals.push("A differently worded situation decided by the existing sample assumptions");
  const completed=operation.scorecard && operation.scorecard.passed+operation.scorecard.failed>0;
  async function exportWork() {
    setWorking(true); setError("");
    try {
      const runs=await exportRuns(session, loadEvidence);
      downloadJSON("vibe-evaluation.json",{format:"agentclash-evaluation-v1", artifact_id:artifact.id, scope:artifact.scope_note || "Runs here; business systems are not connected.", sample:artifact.sample || null, artifacts:session.document.artifacts, rules:session.document.policies, runs});
    } catch(e) {setError((e as Error).message)} finally {setWorking(false)}
  }
  return <section className="mt-6 space-y-3" aria-label="Keep and extend your results">
    {!showTougher && <div className="flex flex-wrap gap-2">
      <VibeButton disabled={busy || working} onClick={() => void exportWork()}>{working ? "Preparing…" : "Export / build it myself"}</VibeButton>
      {!showTougher && completed && artifact.kind === "test_suite" && proposals.length>0 && onTougher && <VibeButton disabled={busy || working} onClick={() => setTougher(!tougher)}>Try tougher situations</VibeButton>}
      {completed && !hideContact && <ProjectEnquiry key={artifact.id} session={session} artifact={artifact} operation={operation} />}
    </div>}
    {tougher && <div className="vibe-panel p-4 text-sm space-y-3"><p>Prepare {proposals.length} additional examples. Your existing examples and results stay intact.</p><ul className="list-disc pl-5 space-y-2">{proposals.map(x => <li key={x}>{x}</li>)}</ul><p className="vibe-muted">Next, review one maximum cost for preparing and running this batch, including your original examples.</p><VibeButton disabled={busy} onClick={() => {setTougher(false);onTougher?.(`Add exactly ${proposals.length} examples covering the following gaps or input variations, using only our existing rules. Preserve every existing case, expectation and grading setting; do not invent business rules beyond this batch. ${proposals.join("; ")}`,proposals.length,artifact.id)}}>See cost and continue</VibeButton></div>}

    {error && <p role="alert" className="text-sm text-builder-warn">{error}</p>}
  </section>;
}
