#!/usr/bin/env bash
#
# select-review-model.sh — picks the best opencode-go model for a PR review
# based on what files changed and how large the diff is.
#
# Usage: ./scripts/pr-review/select-review-model.sh [--base <base-ref>] [--head <head-ref>]
#
# Outputs a single line: opencode-go/<model-id>
#
# Model selection logic (all opencode-go models via $OPENCODE_GO_API_KEY):
#   - Large or cross-cutting diffs (>500 lines or touches 2+ major areas) → glm-5.1 (strongest reasoning)
#   - Go backend/CLI changes → glm-5.1 (strong at systems/code)
#   - Frontend/TypeScript changes → qwen3.6-plus (good all-rounder)
#   - Config/CI only → deepseek-v4-flash (fast, cheap)
#   - Default fallback → qwen3.6-plus

set -euo pipefail

BASE_REF="${BASE_REF:-}"
HEAD_REF="${HEAD_REF:-}"

# Parse flags
while [[ $# -gt 0 ]]; do
  case "$1" in
    --base) BASE_REF="$2"; shift 2 ;;
    --head) HEAD_REF="$2"; shift 2 ;;
    *) echo "Unknown flag: $1" >&2; exit 1 ;;
  esac
done

# If refs not provided, use GitHub event context or default to diff against main
if [[ -z "$BASE_REF" ]]; then
  if [[ -n "${GITHUB_BASE_REF:-}" ]]; then
    BASE_REF="origin/${GITHUB_BASE_REF}"
  else
    BASE_REF="origin/main"
  fi
fi

if [[ -z "$HEAD_REF" ]]; then
  if [[ -n "${GITHUB_HEAD_REF:-}" ]]; then
    HEAD_REF="origin/${GITHUB_HEAD_REF}"
  else
    HEAD_REF="HEAD"
  fi
fi

# Get the diff stat
DIFF_STAT=$(git diff --stat "${BASE_REF}"..."${HEAD_REF}" 2>/dev/null || git diff --stat "${BASE_REF}" "${HEAD_REF}" 2>/dev/null || echo "")

if [[ -z "$DIFF_STAT" ]]; then
  echo "opencode-go/qwen3.6-plus"
  exit 0
fi

# Count total changed lines
TOTAL_LINES=$(git diff --shortstat "${BASE_REF}"..."${HEAD_REF}" 2>/dev/null | grep -oP '\d+(?= insertion)' || echo "0")
TOTAL_DEL=$(git diff --shortstat "${BASE_REF}"..."${HEAD_REF}" 2>/dev/null | grep -oP '\d+(?= deletion)' || echo "0")
TOTAL_CHANGED=$((TOTAL_LINES + TOTAL_DEL))

# Get list of changed files
CHANGED_FILES=$(git diff --name-only "${BASE_REF}"..."${HEAD_REF}" 2>/dev/null || git diff --name-only "${BASE_REF}" "${HEAD_REF}" 2>/dev/null || echo "")

# Count files touching each major area
GO_BACKEND=0
GO_CLI=0
FRONTEND=0
CI_WORKFLOWS=0
CONFIG_DOCS=0
TOTAL_FILES=0

while IFS= read -r file; do
  [[ -z "$file" ]] && continue
  TOTAL_FILES=$((TOTAL_FILES + 1))

  case "$file" in
    backend/**/*.go) GO_BACKEND=1 ;;
    cli/**/*.go) GO_CLI=1 ;;
    web/**) FRONTEND=1 ;;
    .github/workflows/**) CI_WORKFLOWS=1 ;;
    *.md|*.json|*.yaml|*.yml|*.toml|Dockerfile*|.gitignore|LICENSE) CONFIG_DOCS=1 ;;
    *) CONFIG_DOCS=1 ;;
  esac
done <<< "$CHANGED_FILES"

# Count distinct major areas touched
AREAS_TOUCHED=$((GO_BACKEND + GO_CLI + FRONTEND + CI_WORKFLOWS))

# Decision logic
# 1. Large diff or cross-cutting → glm-5.1 for deep analysis
if [[ "$TOTAL_CHANGED" -gt 500 ]] || [[ "$AREAS_TOUCHED" -ge 2 ]]; then
  echo "opencode-go/glm-5.1"
  exit 0
fi

# 2. Go code (backend or CLI) → glm-5.1
if [[ "$GO_BACKEND" -eq 1 ]] || [[ "$GO_CLI" -eq 1 ]]; then
  echo "opencode-go/glm-5.1"
  exit 0
fi

# 3. Frontend → qwen3.6-plus
if [[ "$FRONTEND" -eq 1 ]]; then
  echo "opencode-go/qwen3.6-plus"
  exit 0
fi

# 4. CI/config/docs only → deepseek-v4-flash (fast, cheap)
echo "opencode-go/deepseek-v4-flash"
