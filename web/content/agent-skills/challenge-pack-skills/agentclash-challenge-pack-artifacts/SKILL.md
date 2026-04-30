---
name: agentclash-challenge-pack-artifacts
description: Use when specifying AgentClash challenge pack artifacts, file outputs, evidence capture, upload/download expectations, and scorecard artifact references.
metadata:
  agentclash.role: challenge-pack-artifacts
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Challenge Pack Artifacts

## Purpose
Make files and evidence outputs first-class parts of a challenge pack.

## Use When
- The agent must produce files, reports, patches, images, logs, or structured artifacts.
- Scoring depends on artifact contents.
- Reviewers need downloadable evidence after the run.

## Inputs Needed
- Required output files and formats.
- Artifact names and paths.
- Which artifacts are scored versus review-only.
- Privacy or retention constraints.

## Procedure
1. Name required artifacts clearly.
2. Define where the agent should write them.
3. Connect scored artifacts to validators or judges.
4. Separate review evidence from scoring evidence.
5. Include artifact inspection commands in the report.

## Commands
```bash
agentclash artifact list --run <run-id>
agentclash artifact download <artifact-id>
```

## Related Skills
- `agentclash-scorecard-reader`
- `agentclash-challenge-pack-scoring-validators`
