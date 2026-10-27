# Reviewed tests through application CI

Vibe Evals keeps authored tests, published tests, executed results and imported
claims distinct. A downloaded report is evidence of its recorded source; importing
it does not execute an application or promote a baseline.

1. Review the authored cases and expectations. Keep the tests in the workspace,
   open the advanced builder, compile and explicitly publish the version. Its
   publication receipt retains the originating project/version and the exact
   input-set, challenge and authored case keys.
2. In Details, select that exact publication/input set. Configure or select a
   canonical deployment. For an HTTP connector, an administrator reviews the
   HTTPS target, mapped inputs and workspace secret before approval. Secrets stay
   in the workspace; exports contain references and provenance, not secret values.
3. Run published tests. The canonical worker invokes the connected application
   and stores actual observations and grades. A deterministic fixture proves the
   integration path, not a production model's accuracy. A connector snapshot
   identifies the declared integration settings; unknown external application or
   model code remains unknown.
4. Review a completed run in the workspace CI setup page. Explicitly choose it
   as the locked baseline, keep refresh manual, and save the CI profile. Select
   an existing active deployment to evaluate a connected application, or use the
   existing build/spec/runtime flow for a newly created native candidate.

For example, this manifest reuses an approved connected deployment:

```yaml
version: 1
trigger:
  paths: ["application/**"]
candidate:
  deployment:
    id: "<active-workspace-deployment-id>"
evaluation:
  challenge_pack_version_id: "<published-version-id>"
  input_set_id: "<published-input-set-id>"
baseline:
  run_id: "<reviewed-completed-run-id>"
  run_agent_id: "<reviewed-participant-id>"
  refresh: manual
gate:
  fail_on: regression
  policy_file: .agentclash/gate-policy.json
regressions:
  promote_failures: disabled
```

`candidate.deployment.id` is mutually exclusive with candidate build/spec/runtime
settings. The CLI verifies the deployment belongs to the selected workspace and
is active. The policy file is a JSON object (up to 1 MiB); it is sent to the real
release-gate evaluator. Commit the policy with the manifest and review it with
baseline changes. Run `agentclash ci validate --remote` before `agentclash ci run`.
Use the workspace CI setup page to generate the supported GitHub workflow;
creating a profile or downloading its YAML does not create a GitHub PR.

A failed CI gate retains the candidate's canonical run, scorecard, comparison and
gate artifacts. Open the failure, review the actual answer and explicitly propose
a regression case through the existing regression suite. Project result links use
`run=run:<run-id>` (or `eval_session:<id>`), with the originating `session` and
`agent` artifact IDs. Independently shared report IDs and SDK report IDs cannot
stand in for canonical run IDs or complete baselines.

In Settings, “Review an updated export” compares authored identities and values,
not array positions. Cancel is read-only. “Accept changes as a new version” binds
the reviewed file bytes and current project revision, retains the exact original
file and creates an unapproved child version. It preserves earlier versions and
historical claims without creating execution. A partial export cannot silently
remove omitted authored cases. Publish the new version separately, execute it,
then explicitly select and save the new completed CI baseline. Publication alone
never changes the profile's pinned baseline.

`agentclash evaltest run` is a local smoke command. It does not invoke the canonical
connected runtime described here. For Python SDK/pytest execution, follow the
installed SDK version's own application integration documentation; no uninspected
SDK callable or provider configuration is assumed by this guide.
