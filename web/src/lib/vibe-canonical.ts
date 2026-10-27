import {vibeFetch} from "./vibe";
import type {CaseEvidenceBinding,EvidenceMetadata,Fingerprints,ProjectResultBinding,ResultRef} from "./vibe-result-evidence";
import type {ExportItem} from "./vibe-export";
export type CanonicalParticipant={dispatch_validated_at?:string;dispatch_guard_version?:number;run_id:string;run_agent_id:string;snapshot_id:string;deployment_id:string;adapter:string;status:string;label:string;challenge_input_set_id:string;cases:(CaseEvidenceBinding&{challenge_key:string;input_set_key:string;challenge_identity_id:string})[];fingerprints:Fingerprints;events:{id:string;source_event_id?:string;sequence:number;type:string;normalized_payload_sha256:string}[]};
export type CanonicalResult={fleet?:{id:string;status:string;eval_session_ids:string[]};experiment?:{requested:number;completed:number;failed:number;pending:number;scored:number;aggregate?:{overall?:{n:number;mean:number;std_dev:number};participants?:{label:string;overall?:{n:number;mean:number;std_dev:number}}[]};warnings:string[]};binding:ProjectResultBinding;evidence_metadata:EvidenceMetadata;name:string;status:string;created_at:string;participants:CanonicalParticipant[];evidence_hash:string;omissions:string[]};
export const canonicalBase=(session:string)=>`/sessions/${encodeURIComponent(session)}/canonical-results`;
export async function loadCanonicalResult(session:string,ref:ResultRef,token?:string|null,artifact?:string,hash?:string){
 if(ref.kind!=="run"&&ref.kind!=="eval_session")throw Error("Choose a canonical result.");
 const params=new URLSearchParams();if(artifact)params.set("artifact_id",artifact);if(hash)params.set("hash",hash);
 const row=await vibeFetch<CanonicalResult>(`${canonicalBase(session)}/${ref.kind}/${encodeURIComponent(ref.id)}?${params}`,token);
 if(row.binding.session_id!==session||row.binding.result_ref.kind!==ref.kind||row.binding.result_ref.id!==ref.id||(artifact&&row.binding.artifact_id!==artifact)||!row.evidence_hash)throw Error("Canonical source identity changed.");return row;
}
export function canonicalExportValue(item:ExportItem,results:CanonicalResult[],session:string){
 const source=results.find(r=>r.binding.result_ref.kind===item.result_ref?.kind&&r.binding.result_ref.id===item.result_ref?.id&&r.binding.artifact_id===item.artifact_id);
 if(!source||source.binding.session_id!==session||source.evidence_hash!==item.hash||item.id!==`${source.binding.result_ref.kind}:${source.binding.result_ref.id}:${source.binding.artifact_id}`)throw Error("Canonical export differs from the frozen preparation.");
 return structuredClone(source);
}
