# Review Checkpoint Contract

Task: create a fresh isolated branch from `main`, avoid the dirty primary checkout, and open a draft PR for this workflow setup.

## Functional Expectations

1. Work must happen in a fresh git worktree based on local `main`, not in `/home/atharva/agentclash`.
2. The branch must be newly created for this task and must start from the current local `main` commit.
3. A draft PR must be opened from that branch to `main`.
4. The workflow must remain review-checkpoint compliant:
   - contract created before further changes
   - checkpoint scratch updated after each implementation step
   - self-review and cumulative-review recorded before commit
5. The request to persist "do not write on main unless explicitly asked" must be attempted via agentic-memory; if that path is unavailable, the blocker must be reported explicitly instead of pretending it succeeded.

## Tests And Verification

- Confirm the fresh worktree path with `pwd`.
- Confirm branch ancestry with `git branch --show-current` and `git rev-parse HEAD main`.
- Confirm repo diff for this branch with `git status --short`.
- Confirm PR creation by capturing the resulting PR URL.

## Manual Verification

- Verify the worktree is separate from the dirty main checkout.
- Verify the branch name is `codex/review-checkpoint-main-default`.
- Verify the final PR is a draft PR targeting `main`.
