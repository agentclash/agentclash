import {createApiClient} from "./api/client";
export type DeploymentAdapter = "native" | "http_json" | "hosted";
export type ConnectorConfig = {adapter:Exclude<DeploymentAdapter,"native">;endpoint_url:string;secret_key:string;auth:"bearer"|"api_key";input_fields?:Record<string,string>;literal_json?:Record<string,unknown>;response_pointer:string;timeout_seconds:number};
export type ConnectorBinding = {snapshot_id:string;deployment_id:string;workspace_id:string;config:ConnectorConfig;target_origin:string;secret_id:string;secret_revision:number;approved:boolean};
export type ConnectorSelection = {session_id:string;workspace_id:string;artifact_id:string;deployment_id:string;snapshot_id:string};
// The publication picker must supply an exact retained Task 5 publication and
// input-set identity. A mutable draft is never used to select a run's tests.
export type ConnectorPublication = {draft_id:string;artifact_id:string;challenge_pack_version_id:string;challenge_input_set_id:string;input_set_key:string;case_count:number};
export function connectorClient(token:string,workspace:string){
 const api=createApiClient(token),base=`/v1/workspaces/${encodeURIComponent(workspace)}/connectors`;
 return {
  list:(signal?:AbortSignal)=>api.get<{connectors:ConnectorBinding[]}>(base,{signal,cache:"no-store"}),
  save:(name:string,config:ConnectorConfig)=>api.post<ConnectorBinding>(base,{name,config}),
  approve:(binding:ConnectorBinding)=>api.post<ConnectorBinding>(`${base}/${binding.snapshot_id}/approve`,{target_origin:binding.target_origin,secret_id:binding.secret_id,secret_revision:binding.secret_revision}),
  revoke:(binding:ConnectorBinding)=>api.post(`${base}/${binding.snapshot_id}/revoke`,{}),
  run:(binding:ConnectorBinding,publication:ConnectorPublication)=>api.post<{id:string}>("/v1/runs",{workspace_id:workspace,challenge_pack_version_id:publication.challenge_pack_version_id,challenge_input_set_id:publication.challenge_input_set_id,agent_deployment_ids:[binding.deployment_id],deployment_snapshot_ids:{[binding.deployment_id]:binding.snapshot_id}}),
 };
}
