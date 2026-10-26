"use client";
import type { SuiteValidation, ValidationReview } from "@/lib/vibe-result-evidence";
import { VibeButton } from "./vibe-button";

export function ValidationDetails({validation,onFix}:{validation:SuiteValidation;onFix?:(text:string)=>void}){
 return <section aria-label="Test validation" className="space-y-2"><h3>Validation: {validation.status}</h3><p className="text-sm vibe-muted">A bounded check against your supplied rules.</p>
  {validation.cases?.map(item=><div key={item.case_key}><strong>{item.case_key} · {item.status}</strong><p>{item.reason}</p>{item.status!=="supported"&&onFix&&<VibeButton onClick={()=>onFix(`Review the expected behavior for proposed case ${item.case_key}: ${item.reason}`)}>Review {item.case_key}</VibeButton>}</div>)}
  {validation.rules?.map(item=><div key={item.rule_id}><strong>Rule {item.rule_id} · {item.status}</strong><p>{item.reason}</p>{item.status!=="supported"&&onFix&&<VibeButton onClick={()=>onFix(`Clarify rule ${item.rule_id}: ${item.reason}`)}>Clarify {item.rule_id}</VibeButton>}</div>)}
  {validation.problems?.map((problem,i)=><p key={i}>{problem.subject} · {problem.status}: {problem.reason}</p>)}
  {validation.shared_criteria?.reason&&<p>Shared criteria: {validation.shared_criteria.reason}</p>}{validation.policy_reconciliation?.reason&&<p>Policy: {validation.policy_reconciliation.reason}</p>}
 </section>;
}
export function ValidationHistory({reviews,onFix,onRetry}:{reviews:ValidationReview[];onFix:(text:string)=>void;onRetry?:(operation:string)=>void}){
 return <section aria-label="Validation history" className="space-y-3"><h2>Test review history</h2><p>Your working tests and earlier results remain separate from rejected proposals. Showing the latest {reviews.length} reviews.</p>{reviews.map(review=><details key={review.id} open={review.validation.status!=="supported"}><summary>{review.validation.status} · proposed version {review.candidate_artifact_id.slice(0,8)}</summary>
  {review.error&&<p>{review.error.message}</p>}
  {review.cases?.map(c=><div key={c.case_key}><strong>{c.case_key}</strong><pre className="whitespace-pre-wrap break-words">{JSON.stringify(c.input,null,2)}</pre><p>Expected: {c.expected}</p></div>)}
  <ValidationDetails validation={review.validation} onFix={onFix}/>
  {review.validation.status==="unavailable"&&(onRetry?<VibeButton onClick={()=>onRetry(review.operation_id)}>Retry test validation</VibeButton>:<VibeButton onClick={()=>onFix("Review these tests against the supplied rules again. Keep the working version until validation succeeds.")}>Request validation again</VibeButton>)}
 </details>)}</section>;
}
