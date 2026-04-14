-- name: GetRunAgentExecutionContextByID :one
SELECT
    ra.id AS run_agent_id,
    ra.organization_id AS run_agent_organization_id,
    ra.workspace_id AS run_agent_workspace_id,
    ra.run_id AS run_agent_run_id,
    ra.agent_deployment_id AS run_agent_agent_deployment_id,
    ra.agent_deployment_snapshot_id AS run_agent_agent_deployment_snapshot_id,
    ra.lane_index AS run_agent_lane_index,
    ra.label AS run_agent_label,
    ra.status AS run_agent_status,
    ra.queued_at AS run_agent_queued_at,
    ra.started_at AS run_agent_started_at,
    ra.finished_at AS run_agent_finished_at,
    ra.failure_reason AS run_agent_failure_reason,
    ra.created_at AS run_agent_created_at,
    ra.updated_at AS run_agent_updated_at,

    r.id AS run_id,
    r.organization_id AS run_organization_id,
    r.workspace_id AS run_workspace_id,
    r.challenge_pack_version_id AS run_challenge_pack_version_id,
    r.challenge_input_set_id AS run_challenge_input_set_id,
    r.created_by_user_id AS run_created_by_user_id,
    r.name AS run_name,
    r.status AS run_status,
    r.execution_mode AS run_execution_mode,
    r.temporal_workflow_id AS run_temporal_workflow_id,
    r.temporal_run_id AS run_temporal_run_id,
    r.execution_plan AS run_execution_plan,
    r.queued_at AS run_queued_at,
    r.started_at AS run_started_at,
    r.finished_at AS run_finished_at,
    r.cancelled_at AS run_cancelled_at,
    r.failed_at AS run_failed_at,
    r.created_at AS run_created_at,
    r.updated_at AS run_updated_at,

    cpv.id AS challenge_pack_version_id,
    cpv.challenge_pack_id AS challenge_pack_id,
    cpv.version_number AS challenge_pack_version_number,
    cpv.manifest_checksum AS challenge_pack_manifest_checksum,
    cpv.manifest AS challenge_pack_manifest,
    challenge_definitions.challenge_definitions::jsonb AS challenge_pack_challenges,

    cis.id AS challenge_input_set_id,
    cis.challenge_pack_version_id AS challenge_input_set_challenge_pack_version_id,
    cis.input_key AS challenge_input_set_input_key,
    cis.name AS challenge_input_set_name,
    cis.description AS challenge_input_set_description,
    cis.input_checksum AS challenge_input_set_input_checksum,
    challenge_input_items.challenge_input_items::jsonb AS challenge_input_set_items,

    ads.id AS snapshot_id,
    ads.agent_deployment_id AS snapshot_agent_deployment_id,
    ads.agent_build_id AS snapshot_agent_build_id,
    ads.source_agent_build_version_id AS snapshot_source_agent_build_version_id,
    ads.source_runtime_profile_id AS snapshot_source_runtime_profile_id,
    ads.source_provider_account_id AS snapshot_source_provider_account_id,
    ads.source_model_alias_id AS snapshot_source_model_alias_id,
    ads.deployment_type AS snapshot_deployment_type,
    ads.endpoint_url AS snapshot_endpoint_url,
    ads.snapshot_hash AS snapshot_hash,
    ads.snapshot_config AS snapshot_config,
    ads.source_agent_spec AS snapshot_source_agent_spec,

    rp.id AS runtime_profile_id,
    rp.name AS runtime_profile_name,
    rp.slug AS runtime_profile_slug,
    rp.execution_target AS runtime_profile_execution_target,
    rp.trace_mode AS runtime_profile_trace_mode,
    rp.max_iterations AS runtime_profile_max_iterations,
    rp.max_tool_calls AS runtime_profile_max_tool_calls,
    rp.step_timeout_seconds AS runtime_profile_step_timeout_seconds,
    rp.run_timeout_seconds AS runtime_profile_run_timeout_seconds,
    rp.profile_config AS runtime_profile_profile_config,

    pa.id AS provider_account_id,
    pa.workspace_id AS provider_account_workspace_id,
    pa.provider_key AS provider_account_provider_key,
    pa.name AS provider_account_name,
    pa.credential_reference AS provider_account_credential_reference,
    pa.limits_config AS provider_account_limits_config,

    ma.id AS model_alias_id,
    ma.workspace_id AS model_alias_workspace_id,
    ma.provider_account_id AS model_alias_provider_account_id,
    ma.model_catalog_entry_id AS model_alias_model_catalog_entry_id,
    ma.alias_key AS model_alias_alias_key,
    ma.display_name AS model_alias_display_name,

    mce.id AS model_catalog_entry_id,
    mce.provider_key AS model_catalog_provider_key,
    mce.provider_model_id AS model_catalog_provider_model_id,
    mce.display_name AS model_catalog_display_name,
    mce.model_family AS model_catalog_model_family,
    mce.modality AS model_catalog_modality,
    mce.metadata AS model_catalog_metadata,
    mce.input_cost_per_million_tokens AS model_catalog_input_cost_per_million_tokens,
    mce.output_cost_per_million_tokens AS model_catalog_output_cost_per_million_tokens
FROM run_agents AS ra
JOIN runs AS r
  ON r.id = ra.run_id
 AND r.organization_id = ra.organization_id
 AND r.workspace_id = ra.workspace_id
JOIN challenge_pack_versions AS cpv
  ON cpv.id = r.challenge_pack_version_id
LEFT JOIN LATERAL (
  SELECT COALESCE(
    jsonb_agg(
      jsonb_build_object(
        'id', cpvc.id,
        'challenge_identity_id', cpvc.challenge_identity_id,
        'challenge_key', ci.challenge_key,
        'execution_order', cpvc.execution_order,
        'title', cpvc.title_snapshot,
        'category', cpvc.category_snapshot,
        'difficulty', cpvc.difficulty_snapshot,
        'definition', cpvc.challenge_definition
      )
      ORDER BY cpvc.execution_order
    ),
    '[]'::jsonb
  ) AS challenge_definitions
  FROM challenge_pack_version_challenges AS cpvc
  JOIN challenge_identities AS ci
    ON ci.id = cpvc.challenge_identity_id
   AND ci.challenge_pack_id = cpv.challenge_pack_id
  WHERE cpvc.challenge_pack_version_id = cpv.id
) AS challenge_definitions
  ON TRUE
LEFT JOIN challenge_input_sets AS cis
  ON cis.id = r.challenge_input_set_id
 AND cis.challenge_pack_version_id = r.challenge_pack_version_id
LEFT JOIN LATERAL (
  SELECT COALESCE(
    jsonb_agg(
      jsonb_build_object(
        'id', cii.id,
        'challenge_identity_id', cii.challenge_identity_id,
        'challenge_key', ci.challenge_key,
        'item_key', cii.item_key,
        'payload', cii.payload
      )
      ORDER BY cpvc.execution_order, cii.item_key
    ),
    '[]'::jsonb
  ) AS challenge_input_items
  FROM challenge_input_items AS cii
  JOIN challenge_pack_version_challenges AS cpvc
    ON cpvc.challenge_pack_version_id = cii.challenge_pack_version_id
   AND cpvc.challenge_identity_id = cii.challenge_identity_id
  JOIN challenge_identities AS ci
    ON ci.id = cii.challenge_identity_id
   AND ci.challenge_pack_id = cpv.challenge_pack_id
  WHERE cii.challenge_input_set_id = cis.id
) AS challenge_input_items
  ON TRUE
JOIN agent_deployment_snapshots AS ads
  ON ads.id = ra.agent_deployment_snapshot_id
 AND ads.agent_deployment_id = ra.agent_deployment_id
 AND ads.organization_id = ra.organization_id
 AND ads.workspace_id = ra.workspace_id
JOIN runtime_profiles AS rp
  ON rp.id = ads.source_runtime_profile_id
 AND rp.organization_id = ads.organization_id
LEFT JOIN provider_accounts AS pa
  ON pa.id = ads.source_provider_account_id
 AND pa.organization_id = ads.organization_id
LEFT JOIN model_aliases AS ma
  ON ma.id = ads.source_model_alias_id
 AND ma.organization_id = ads.organization_id
LEFT JOIN model_catalog_entries AS mce
  ON mce.id = ma.model_catalog_entry_id
WHERE ra.id = @id
LIMIT 1;
