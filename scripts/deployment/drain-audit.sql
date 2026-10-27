-- Read-only drain inventory. Redirect output to private operator evidence.
-- No row contents, messages, tokens, workflow IDs or connection details.
-- Counts alone do not prove a write fence: stop/restrict every source writer.
BEGIN READ ONLY;
SET LOCAL statement_timeout = '15s';

SELECT 'runs' AS component, status, count(*) AS records FROM runs GROUP BY status
UNION ALL SELECT 'run_agents', status, count(*) FROM run_agents GROUP BY status
UNION ALL SELECT 'eval_sessions', status, count(*) FROM eval_sessions GROUP BY status
UNION ALL SELECT 'eval_sets', status, count(*) FROM eval_sets GROUP BY status
UNION ALL SELECT 'dataset_generation_jobs', status, count(*) FROM dataset_generation_jobs GROUP BY status
UNION ALL SELECT 'agent_harness_executions', status, count(*) FROM agent_harness_executions GROUP BY status
UNION ALL SELECT 'agent_tryouts', status, count(*) FROM agent_tryouts GROUP BY status
UNION ALL SELECT 'multi_turn_human_turns', status, count(*) FROM multi_turn_human_turns GROUP BY status
ORDER BY component, status;

-- Includes idle app connections, because they can write later. The audit
-- connection is excluded. Query text, usernames and client addresses stay out.
SELECT coalesce(state, 'unknown') AS state, count(*) AS other_database_sessions
FROM pg_stat_activity
WHERE datname = current_database() AND pid <> pg_backend_pid()
  AND backend_type = 'client backend'
GROUP BY state ORDER BY state;

SELECT count(*) AS prepared_transactions
FROM pg_prepared_xacts WHERE database = current_database();

SELECT count(*) AS applied_migrations FROM schema_migrations;
COMMIT;
