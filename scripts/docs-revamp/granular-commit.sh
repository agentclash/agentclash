#!/usr/bin/env bash
# Granular commits for feat/docs-revamp — one logical change per commit.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

commit() {
  local msg="$1"
  shift
  git add "$@"
  if git diff --cached --quiet; then
    echo "skip (empty): $msg"
    return 0
  fi
  git commit -m "$msg"
  echo "committed: $msg"
}

commit_if_changed() {
  local msg="$1"
  shift
  local has=0
  for f in "$@"; do
    if [[ -e "$f" ]] && ! git diff --quiet -- "$f" 2>/dev/null; then
      has=1
      break
    fi
    if [[ -e "$f" ]] && ! git ls-files --error-unmatch "$f" >/dev/null 2>&1; then
      has=1
      break
    fi
  done
  if [[ "$has" -eq 1 ]]; then
    commit "$msg" "$@"
  else
    echo "skip (unchanged): $msg"
  fi
}

echo "=== Phase 0: contract + epic ==="
commit_if_changed "test(docs): add feat/docs-revamp test contract" testing/feat-docs-revamp.md
commit_if_changed "docs(epic): add docs revamp migration tracker" docs/epics/docs-revamp.md

echo "=== Phase 0: MDX pipeline ==="
commit_if_changed "feat(web): add shared remark-gfm mdx options" web/src/lib/mdx-options.ts
commit_if_changed "chore(web): add remark-gfm dependency" web/package.json web/pnpm-lock.yaml
commit_if_changed "fix(web): wire remark-gfm into docs MDXRemote" "web/src/app/docs/[[...slug]]/page.tsx"
commit_if_changed "fix(web): wire remark-gfm into blog MDXRemote" "web/src/app/blog/[slug]/page.tsx"

echo "=== Phase 0: typography + components ==="
commit_if_changed "style(web): align docs prose typography with changelog" web/src/app/globals.css
commit_if_changed "feat(web): syntax-highlight docs fenced code blocks" web/src/components/docs/copyable-code-block.tsx
commit_if_changed "feat(web): extend docs MDX table and code mappings" web/src/components/docs/mdx-components.tsx
commit_if_changed "style(web): neutral changelog-style docs callouts" web/src/components/docs/callout.tsx
commit_if_changed "style(web): refresh docs diagram preset chrome" web/src/components/docs/docs-diagram-presets.tsx

echo "=== Phase 1: shell + navigation ==="
commit_if_changed "feat(web): revamp docs shell layout" web/src/components/docs/docs-shell.tsx
commit_if_changed "style(web): align docs sidebar tokens" web/src/components/docs/docs-sidebar.tsx
commit_if_changed "style(web): align docs TOC tokens" web/src/components/docs/docs-toc.tsx
commit_if_changed "style(web): align docs search tokens" web/src/components/docs/docs-search.tsx
commit_if_changed "feat(web): add mobile docs navigation drawer" web/src/components/docs/docs-mobile-nav.tsx

echo "=== Phase 3: new pages ==="
commit_if_changed "docs(guides): add datasets overview" web/content/docs/guides/datasets-overview.mdx
commit_if_changed "docs(challenge-packs): add multi-turn reference" web/content/docs/challenge-packs/multi-turn.mdx
commit_if_changed "docs(guides): add security evaluation guide" web/content/docs/guides/security-evaluation.mdx
commit_if_changed "docs(nav): register new doc pages in sidebar" web/src/lib/docs.ts
commit_if_changed "docs(home): refresh index for datasets multi-turn security" web/content/docs/index.mdx

echo "=== Phase 2: per-file migration ==="
chmod +x scripts/docs-revamp/migrate-frontmatter.py

MDX_FILES=()
while IFS= read -r file; do
  MDX_FILES+=("$file")
done < <(find web/content/docs -name '*.mdx' | sort)

link_keys_for() {
  local rel="$1"
  case "$rel" in
    getting-started/*) echo "datasets multi_turn" ;;
    concepts/*) echo "datasets multi_turn security" ;;
    guides/dataset-ci-gates.mdx|guides/ci-cd-workload-recipes.mdx) echo "datasets security" ;;
    guides/*) echo "datasets multi_turn security" ;;
    challenge-packs/*) echo "multi_turn security" ;;
    architecture/*) echo "datasets security" ;;
    contributing/*) echo "datasets" ;;
    reference/*) echo "datasets security" ;;
    *) echo "" ;;
  esac
}

for file in "${MDX_FILES[@]}"; do
  rel="${file#web/content/docs/}"
  case "$rel" in
    guides/datasets-overview.mdx|challenge-packs/multi-turn.mdx|guides/security-evaluation.mdx|index.mdx)
      continue
      ;;
  esac

  slug="${rel%.mdx}"
  python3 scripts/docs-revamp/migrate-frontmatter.py --mode date --file "$rel"
  commit_if_changed "docs($slug): add dateModified frontmatter" "$file"

  keys=$(link_keys_for "$rel")
  for key in $keys; do
    python3 scripts/docs-revamp/migrate-frontmatter.py --mode link --link-key "$key" --file "$rel"
    commit_if_changed "docs($slug): link $key in see-also" "$file"
  done
done

echo "=== Phase 4: epic completion + tooling ==="
commit_if_changed "chore(docs): add docs revamp migration scripts" \
  scripts/docs-revamp/migrate-frontmatter.py \
  scripts/docs-revamp/granular-commit.sh \
  scripts/docs-revamp/update-epic-checkboxes.py

python3 scripts/docs-revamp/update-epic-checkboxes.py
commit_if_changed "docs(epic): mark revamp phases complete" docs/epics/docs-revamp.md

echo "=== Done ==="
git rev-list --count main..HEAD | xargs -I{} echo "Commits on branch: {}"
