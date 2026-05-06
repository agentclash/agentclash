# Issue #581 Expectations

## Scope

Add a backend foundation for Agent Harness suites and private task banks without requiring challenge packs and without duplicating scoring, validator, replay, or judge systems.

## Expected behavior

- Workspaces can create and list versioned Agent Harness suites.
- Suites contain versioned tasks with public task prompts plus private evaluation config, execution config, repository/base ref, and source snapshots.
- Task sources support manual, GitHub issue, uploaded file, and prior harness run metadata. GitHub issue tasks persist title/body/comments/labels/repository/base ref in the source snapshot.
- Starting a suite creates normal Agent Harness executions for a single task, a subset, or the full suite against one or more harnesses. Each execution snapshots the task prompt/configs and records suite/task/version metadata.
- Results remain comparable through existing harness execution/run-agent scorecards by suite version, harness, model/template/repo, and budget metadata.

## Validation

- Repository tests cover suite/task creation and listing.
- API tests cover creating/listing suites and starting suite runs.
- Existing harness execution tests remain green.
