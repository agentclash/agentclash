package repository

import "context"

// ConnectorDependencyChecks extends the API's existing dependency check map.
// It has no provider/network probe and does not alter inference availability.
func (r *Repository) ConnectorDependencyChecks(ctx context.Context) map[string]string {
	checks := map[string]string{"connector_schema": "unavailable"}
	rows, err := r.db.Query(ctx, `SELECT c.snapshot_id,c.workspace_id,c.organization_id,c.adapter,c.config,c.target_origin,c.secret_id,c.secret_revision,c.approved_by,c.approved_at,c.revoked_at,
 s.id,s.workspace_id,s.revision,s.encrypted_value,d.id,d.run_agent_id,d.case_key,d.source_case_id,d.challenge_key,d.snapshot_id,d.input_hash,d.state,d.deadline_at,d.response,d.updated_at,
 dl.run_agent_id,dl.deadline_at,h.run_id,h.run_agent_id,h.external_run_id,h.status,h.dispatch_claimed_at,h.deadline_at,
 e.source_event_id,e.payload_hash,e.original_event,e.sequence_number,n.event,n.run_id,n.run_agent_id,n.delivered_at,n.created_at
 FROM connector_snapshots c,workspace_secrets s,connector_dispatches d,connector_run_deadlines dl,hosted_run_executions h,hosted_event_receipts e,hosted_terminal_notifications n LIMIT 0`)
	if err == nil {
		rows.Close()
		if rows.Err() == nil {
			checks["connector_schema"] = "ok"
		}
	}
	rows, err = r.db.Query(ctx, `SELECT f.request_id,f.request_hash,f.eval_set_id,s.workspace_id,a.workspace_id,a.request_id,a.request_hash,a.eval_session_id,c.run_agent_id,c.snapshot_id,c.settings_hash,c.settings,c.admitted_at,c.dispatch_validated_at,c.dispatch_guard_version FROM eval_set_admissions f,eval_sessions s,eval_session_admissions a,run_agent_controlled_settings c LIMIT 0`)
	checks["experiment_schema"] = "unavailable"
	if err == nil {
		rows.Close()
		if rows.Err() == nil {
			checks["experiment_schema"] = "ok"
		}
	}
	return checks
}
