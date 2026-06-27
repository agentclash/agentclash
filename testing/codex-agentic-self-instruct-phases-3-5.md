# Codex Agentic Self-Instruct Phases 3-5 — Test Contract

## Functional Behavior
- Phase 3 adds an optional direct solver loop for `agentic_self_instruct` generation:
  - API/CLI/UI inputs can specify weak and strong provider accounts, models, and rollout counts.
  - When `solver_mode` is `direct_provider`, each valid challenger candidate is solved by the configured weak and strong models before judging.
  - The judge prompt includes the seed set, candidate, weak attempts, and strong attempts.
  - Accepted examples persist solver metadata including model IDs, rollout counts, solver attempts, judge scores, and score gap.
  - Rejected examples preserve structured metadata for provider failures, solver failures, judge parse failures, invalid candidates, duplicates, and quality gate failures.
- Phase 4 adds AgentClash deployment-loop configuration at the product surface:
  - API/CLI/UI inputs can capture weak/strong deployment IDs, challenge pack version ID, challenge key, and field mapping.
  - The generation job config persists this deployment context for history and follow-up evaluation workflows.
  - The workflow keeps per-candidate generation execution on the direct provider path; deployment-backed competitive runs remain handled by the existing dataset evaluation/run creation flow.
- Phase 5 improves generation history and explainability:
  - Job summaries include accepted counts, rejection counts by reason, average weak score, average strong score, average gap, and solver mode.
  - A generation history endpoint returns structured rejection/attempt records for a job.
  - The dataset generation UI surfaces recent job summary stats and lets the user inspect recent rejection reasons.

## Unit Tests
- `datasetgeneration.DecodeJobConfigForStrategy` accepts valid direct-provider solver config and rejects invalid solver modes or rollout counts.
- `BuildAgenticSolverPrompt` omits the expected answer and contains only the candidate task plus role guidance.
- `BuildAgenticJudgePrompt` includes weak/strong solver attempt sections when attempts are present.
- `DatasetGenerationActivities.ExecuteSyntheticDatasetGeneration` invokes challenger, weak solver, strong solver, and judge in order for direct-provider jobs.
- `DatasetGenerationActivities.ExecuteSyntheticDatasetGeneration` records solver metadata on accepted examples.
- `DatasetGenerationActivities.ExecuteSyntheticDatasetGeneration` records a solver failure rejection when a weak/strong provider call fails.
- API tests cover validation and persistence of new solver/deployment generation fields.
- CLI command tests cover new generation flags and schema output.

## Integration / Functional Tests
- Backend package tests pass for generation, API, repository, and workflow packages.
- CLI command tests pass and the schema golden snapshot is updated.
- Web lint passes for the dataset generation dialog and shared API types.

## Smoke Tests
- Start generation with default judge-only settings still works for existing clients.
- Start generation with direct weak/strong solver settings accepts the payload and stores the full config.
- The generation history endpoint returns rejections for an existing generation job.
- The dataset generation dialog renders without type or lint errors after adding solver and deployment fields.

## E2E Tests
- N/A — this PR does not introduce a browser-driven E2E suite. Manual UI verification is covered by the smoke checks and lint/type checks.

## Manual / cURL Tests
- Start a direct-provider agentic generation job:

  ```bash
  curl -X POST "$API/v1/workspaces/$WORKSPACE_ID/datasets/$DATASET_ID/generate" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
      "strategy": "agentic_self_instruct",
      "targetCount": 5,
      "providerAccountId": "'"$CHALLENGER_PROVIDER_ID"'",
      "model": "gpt-4.1-mini",
      "judgeProviderAccountId": "'"$JUDGE_PROVIDER_ID"'",
      "judgeModel": "gpt-4.1",
      "solverMode": "direct_provider",
      "weakProviderAccountId": "'"$WEAK_PROVIDER_ID"'",
      "weakModel": "gpt-4.1-nano",
      "strongProviderAccountId": "'"$STRONG_PROVIDER_ID"'",
      "strongModel": "gpt-4.1",
      "weakRollouts": 1,
      "strongRollouts": 1
    }'
  ```

- Inspect generation history for a job:

  ```bash
  curl "$API/v1/workspaces/$WORKSPACE_ID/datasets/$DATASET_ID/generations/$JOB_ID/rejections" \
    -H "Authorization: Bearer $TOKEN"
  ```

- Start a generation job with deployment context:

  ```bash
  agentclash dataset generate "$DATASET_ID" \
    --strategy agentic_self_instruct \
    --provider-account "$CHALLENGER_PROVIDER_ID" \
    --model gpt-4.1-mini \
    --judge-provider-account "$JUDGE_PROVIDER_ID" \
    --judge-model gpt-4.1 \
    --solver-mode direct_provider \
    --weak-provider-account "$WEAK_PROVIDER_ID" \
    --weak-model gpt-4.1-nano \
    --strong-provider-account "$STRONG_PROVIDER_ID" \
    --strong-model gpt-4.1 \
    --weak-deployment "$WEAK_DEPLOYMENT_ID" \
    --strong-deployment "$STRONG_DEPLOYMENT_ID" \
    --challenge-pack-version "$CHALLENGE_PACK_VERSION_ID" \
    --challenge-key "$CHALLENGE_KEY" \
    --field-mapping '{"input":"prompt","expected":"answer"}'
  ```
