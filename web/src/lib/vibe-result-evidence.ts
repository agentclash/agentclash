import type { CaseResult, Operation } from "./vibe";
export type ResultRef = { kind: "vibe_operation" | "run" | "eval_session" | "sdk_report"; id: string };
export type ResultOrigin = "prototype" | "recorded" | "imported" | "live" | "regraded";
export type Fingerprints = { model: string | null; build: string | null; test: string | null; judge: string | null; config: string | null };
export type DimensionCapture = { scope: "unknown" | "partial" | "complete"; instrumentation: "unknown" | "not_instrumented" | "instrumented" };
export type CaptureScope = { messages: DimensionCapture; tools: DimensionCapture; retrieval: DimensionCapture };
export type EvidenceMetadata = { result_ref: ResultRef; origin: ResultOrigin; fingerprints: Fingerprints; capture_scope: CaptureScope; application_revision: string | null; source_result_ref?: ResultRef };
export type ProjectResultBinding = { session_id: string; artifact_id: string; result_ref: ResultRef; challenge_pack_version_id?: string };
export type CaseEvidenceBinding = { case_key: string; source_case_id: string; run_agent_id?: string };
export type ResultScope = { artifact_id: string; case_keys: string[]; retry_source_result_ref?: ResultRef };
export type GradeReviewInput = { source_case_id?: string; source_evidence_hash: string; client_id: string; result_ref: ResultRef; artifact_id: string; case_key: string; check_key?: string; reason: string; quotes?: { message_id: string; text: string }[] };
export type GradeReview = GradeReviewInput & { id: string; created_at: string; evidence_hash: string; evidence: CaseResult; evidence_changed: boolean };
export type RecordedIntakeRequest = { client_id: string; revision: number; label: string; content: string; expectations: { id: string; statement: string }[] };
export type RecordedIntakeOutcome = "saved" | "rejected" | "uncertain";
export type SuiteFinding = { status: string; rule_ids?: string[]; source_block_ids?: string[]; reason: string };
export type SuiteValidation = { status: "supported" | "contradicted" | "unclear" | "unavailable"; blueprint_hash: string; policy_hash: string; validator_version: string; model?: string; profile_hash?: string; cases?: (SuiteFinding & {case_key:string})[]; rules?: (SuiteFinding & {rule_id:string})[]; shared_criteria?: SuiteFinding; policy_reconciliation?: SuiteFinding; problems?: {subject:string;status:string;reason:string}[]; assertions?: unknown[]; consistency?: unknown };
export type ValidationReview = { id:string;operation_id:string;candidate_artifact_id:string;prior_artifact_id?:string;stage:string;validation:SuiteValidation;cases:{case_key:string;input:unknown;expected:string}[];rules:{id:string;statement:string}[];error?:{code:string;message:string};created_at:string };

export function compareResultEvidence(current: Operation, baseline: Operation) {
 const a=current.evidence_metadata, b=baseline.evidence_metadata;
 const changed:string[]=[], unknown:string[]=[];
 for(const key of ["model","build","test","judge","config"] as const){
  const next=a?.fingerprints[key],before=b?.fingerprints[key];
  if(!next||!before)unknown.push(key);else if(next!==before)changed.push(key);
 }
 if(!a||!b)unknown.push("origin");else if(a.origin!==b.origin)changed.push("origin");
 const scope=current.result_scope, oldScope=baseline.result_scope;
 if(!scope||!oldScope)unknown.push("test scope");else if(JSON.stringify(scope.case_keys)!==JSON.stringify(oldScope.case_keys))changed.push("test scope");
 const regraded=a?.origin==="imported"||b?.origin==="imported"||a?.origin==="regraded"||b?.origin==="regraded";
 return {changed,unknown,controlled:!regraded&&!unknown.length&&changed.every(key=>key==="build")};
}
export const unresolvedCase = (result: CaseResult) => result.verdict==="UNKNOWN" || !!result.error || result.checks.some(check => !!check.error || !["PASS","FAIL"].includes(check.verdict)) || (result.expected_checks ?? result.checks.length)>result.checks.length;
