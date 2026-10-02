-- Read-only inventory after ingress/starts are fenced; never settles or replays work.
SELECT json_build_object(
 'operations', (SELECT count(*) FROM vibe_operations WHERE state IN ('CREATED','VALIDATING','RESERVED','QUEUED','RUNNING','FINALIZING','CANCELLING')),
 'outbox', (SELECT count(*) FROM vibe_outbox b JOIN vibe_operations o ON o.id=b.operation_id WHERE b.delivered_at IS NULL AND o.state='QUEUED'),
 'continuations', (SELECT count(*) FROM vibe_cycle_steps c JOIN vibe_operations o ON o.id=c.operation_id JOIN vibe_sessions s ON s.id=o.session_id JOIN vibe_cycle_quotes q ON q.id=c.cycle_id WHERE (c.step IN ('prepare','answer','initial_trial') OR c.step LIKE 'retry:%' OR c.step LIKE 'message:%') AND o.state='COMPLETED' AND s.document->>'format_version'='1' AND s.document#>>'{build,phase}'='ready' AND s.document#>>'{build,cycle_id}'=c.cycle_id::text AND q.stopped_at IS NULL AND s.deleted_at IS NULL),
 'attempts', (SELECT count(*) FROM vibe_attempts WHERE state IN ('DISPATCHING','UNCERTAIN') OR actual_cost IS NULL OR reconciliation_evidence->>'conflict'='true'),
 'holds', (SELECT count(*) FROM vibe_reservations WHERE settled_amount IS NULL) + (SELECT count(*) FROM vibe_operations WHERE billing='RECONCILING'),
 'input_work', (SELECT count(*) FROM vibe_inputs WHERE status IN ('uploaded','extracting') OR lease_until>now()),
 'cleanup', (SELECT count(*) FROM vibe_sessions WHERE deleted_at IS NOT NULL AND cleanup_finished_at IS NULL) + (SELECT count(*) FROM vibe_input_staging),
 'enquiries', (SELECT count(*) FROM vibe_enquiries WHERE status IN ('received','sending') OR lease_until>now())
);
