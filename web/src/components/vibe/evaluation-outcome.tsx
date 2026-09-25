"use client";

import { useState } from "react";
import type { Artifact, CaseResult, Operation, RuleCoverage, Session } from "@/lib/vibe";
import { VibeButton } from "./vibe-button";

function download(name: string, value: unknown) {
  const url = URL.createObjectURL(new Blob([JSON.stringify(value, null, 2)], {type:"application/json"}));
  const link = document.createElement("a"); link.href=url; link.download=name; link.click();
  setTimeout(() => URL.revokeObjectURL(url),1000);
}
export function coverageProposal(rows: RuleCoverage[]) {
  const gaps = rows.filter(row => !row.case_keys.length).slice(0,3);
  if (gaps.length) return gaps.map(row => `One situation covering: ${row.statement}`);
  // Different aspects, not a claim that every task needs a fixed suite size.
  return rows.slice(0,2).map(row => `A boundary or missing-information case for: ${row.statement}`);
}
export function EvaluationOutcome({session, artifact, operation, busy, loadEvidence, onTougher}: {
  session: Session; artifact: Artifact; operation: Operation; busy: boolean;
  loadEvidence: (operation: string,key: string) => Promise<CaseResult>;
  onTougher?: (request: string, count: number, artifactID: string) => void;
}) {
  const [handoff,setHandoff]=useState(false);
  const [summary,setSummary]=useState("");
  const [working,setWorking]=useState(false);
  const [error,setError]=useState("");
  const [tougher,setTougher]=useState(false);
  const proposals=coverageProposal(session.rule_coverage?.[artifact.id] || []);
  const completed=operation.scorecard && operation.scorecard.passed+operation.scorecard.failed>0;
  async function exportWork() {
    setWorking(true); setError("");
    try {
      const runs=await Promise.all(session.operations.filter(o => o.results.length>0).map(async o => ({...o, results:await Promise.all(o.results.map(c => loadEvidence(o.id,c.case_key)))})));
      download("vibe-evaluation.json",{format:"agentclash-evaluation-v1", artifact_id:artifact.id, scope:artifact.scope_note || "Runs here; business systems are not connected.", sample:artifact.sample || null, artifacts:session.document.artifacts, rules:session.document.policies, runs});
    } catch(e) {setError((e as Error).message)} finally {setWorking(false)}
  }
  async function prepareHandoff() {
    setWorking(true);setError("");
    try {
      const cases=await Promise.all(operation.results.map(c => loadEvidence(operation.id,c.case_key)));
      setSummary([artifact.title, artifact.sample ? "SAMPLE demonstration; actual business policy is unspecified." : "Prototype / supplied evidence only; live application has not been verified.",
        `Results: ${operation.scorecard?.passed} passed, ${operation.scorecard?.failed} failed, ${operation.scorecard?.unknown} unassessed.`,
        ...cases.map(c => `${c.verdict}: ${JSON.stringify(c.input)}\nActual reply: ${c.output || "Unavailable"}`),
        `Coverage gaps: ${proposals.join("; ") || "These examples do not establish reliability outside the tested inputs."}`,
        "Production work: connect and validate business systems, permissions, deployment and ongoing monitoring. No business integrations ran here.",
      ].join("\n\n"));setHandoff(true);
    } catch(e) {setError((e as Error).message)} finally {setWorking(false)}
  }
  return <section className="mt-6 space-y-3" aria-label="Keep and extend your results">
    <div className="flex flex-wrap gap-2">
      <VibeButton disabled={busy || working} onClick={() => void exportWork()}>{working ? "Preparing…" : "Export / build it myself"}</VibeButton>
      {completed && artifact.kind === "test_suite" && proposals.length>0 && onTougher && <VibeButton disabled={busy || working} onClick={() => setTougher(!tougher)}>Try tougher situations</VibeButton>}
      {completed && <VibeButton variant="quiet" disabled={busy || working} onClick={() => void prepareHandoff()}>Make this production-ready</VibeButton>}
    </div>
    {tougher && <div className="vibe-panel p-4 text-sm space-y-3"><p>Prepare {proposals.length} additional examples. Your existing examples and results stay intact.</p><ul className="list-disc pl-5 space-y-2">{proposals.map(x => <li key={x}>{x}</li>)}</ul><p className="vibe-muted">You’ll see the complete batch, maximum run cost and estimated time before running. Preparation uses the assistant model.</p><VibeButton disabled={busy} onClick={() => {setTougher(false);onTougher?.(`Add exactly ${proposals.length} examples covering the following gaps or input variations, using only our existing rules. Preserve every existing case, expectation and grading setting; do not invent business rules or run anything. ${proposals.join("; ")}`,proposals.length,artifact.id)}}>Prepare these examples</VibeButton></div>}
    {handoff && <div className="vibe-panel p-4 space-y-3"><label className="block text-sm" htmlFor="production-summary">Review your handoff</label><textarea id="production-summary" className="vibe-textarea w-full min-h-60 text-sm" value={summary} onChange={e => setSummary(e.target.value)}/><p className="text-xs vibe-muted">Copy this summary to reuse when booking. Nothing is sent automatically. Findings, fixes and exports remain available here.</p><div className="flex flex-wrap gap-2"><VibeButton onClick={() => void navigator.clipboard.writeText(summary).catch(() => setError("Select and copy the summary above."))}>Copy summary</VibeButton><a className="vibe-button inline-flex items-center rounded-lg border border-[var(--vibe-border)] px-4 py-2 text-sm" href="https://cal.com/atharva-kanherkar-epgztu/agentclash-demo" target="_blank" rel="noreferrer">Talk to our team</a><VibeButton variant="quiet" onClick={() => void exportWork()}>Export / build it myself</VibeButton></div></div>}
    {error && <p role="alert" className="text-sm text-builder-warn">{error}</p>}
  </section>;
}
