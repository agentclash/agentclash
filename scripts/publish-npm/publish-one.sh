#!/usr/bin/env bash
# Publish one or more npm package directories, tolerating rerun races and
# surfacing actionable Trusted Publishing/bootstrap guidance on failure.
#
# Usage:  publish-one.sh <label> <pkg_dir>...
# Label is only used for log grouping ("platform" | "root").
#
# Why this exists:
#   A single pre-publish `npm view ... version` probe is insufficient for
#   idempotent reruns — npm's registry metadata is eventually consistent.
#   After a partial publish, the earlier `npm view` can still read as
#   missing in one edge while the registry itself already knows the
#   version exists, so `npm publish` returns E409 ("You cannot publish
#   over the previously published versions").
#
#   The safe sequence is:
#     1. Try `npm publish --access=public --provenance`.
#     2. On success → done.
#     3. On failure → probe the registry (with retries, because the
#        positive read may still be propagating). If it reports the
#        expected version, treat the whole step as success.
#     4. Otherwise → surface the original error, plus a clearer hint when
#        the failure looks like missing npm bootstrap or Trusted Publishing
#        setup for the package/workflow pair.

set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "usage: $0 <label> <pkg_dir>..." >&2
  exit 64
fi

label="$1"
shift

conflict_pattern='E409|403 Forbidden.*cannot publish over|cannot publish over the previously'
trusted_publishing_pattern='E404|404 Not Found|E403|403 Forbidden|do not have permission to access|you do not have permission'

# Retry loop for the post-failure visibility probe.
check_published() {
  local name="$1"
  local version="$2"
  local attempts="${NPM_VERIFY_ATTEMPTS:-12}"
  local sleep_s="${NPM_VERIFY_SLEEP:-5}"
  local got
  for _ in $(seq 1 "${attempts}"); do
    if got="$(npm view "${name}@${version}" version --silent 2>/dev/null)" && [ "${got}" = "${version}" ]; then
      return 0
    fi
    sleep "${sleep_s}"
  done
  return 1
}

package_exists() {
  local name="$1"
  local got
  if got="$(npm view "${name}" version --silent 2>/dev/null)" && [ -n "${got}" ]; then
    return 0
  fi
  return 1
}

explain_trusted_publishing_failure() {
  local name="$1"
  local version="$2"

  if ! package_exists "${name}"; then
    cat >&2 <<EOF
${name} does not appear to exist on npm yet.
npm Trusted Publishing cannot create first-time packages. Complete the one-time bootstrap in docs/cli-distribution.md ("npm setup (one-time)"), then rerun this release.
EOF
    return
  fi

  cat >&2 <<EOF
${name} exists on npm, but ${name}@${version} never became visible after the failed publish attempt.
Check Trusted Publishing for ${name} in the npm UI and confirm it is configured for repo agentclash/agentclash and workflow .github/workflows/release-cli.yml before rerunning the release.
EOF
}

for pkg in "$@"; do
  name="$(jq -r .name "${pkg}/package.json")"
  version="$(jq -r .version "${pkg}/package.json")"
  echo "::group::npm publish (${label}) ${name}@${version}"

  # Capture both the exit code and stderr so we can discriminate the
  # rerun-conflict case from bootstrap / Trusted Publishing failures.
  tmp_stderr="$(mktemp)"
  set +e
  npm publish "${pkg}" --access=public --provenance 2>"${tmp_stderr}"
  publish_rc=$?
  set -e
  cat "${tmp_stderr}" >&2

  if [ "${publish_rc}" -eq 0 ]; then
    rm -f "${tmp_stderr}"
    echo "published ${name}@${version}"
    echo "::endgroup::"
    continue
  fi

  if grep -qE "${conflict_pattern}" "${tmp_stderr}"; then
    echo "publish returned a registry-side error for ${name}@${version}; verifying registry state..."
    if check_published "${name}" "${version}"; then
      rm -f "${tmp_stderr}"
      echo "${name}@${version} already on registry at the expected version; treating as success"
      echo "::endgroup::"
      continue
    fi

    rm -f "${tmp_stderr}"
    echo "registry still does not report ${name}@${version} after ${NPM_VERIFY_ATTEMPTS:-12} attempts" >&2
    exit 1
  fi

  if grep -qE "${trusted_publishing_pattern}" "${tmp_stderr}"; then
    echo "publish returned a registry-side error for ${name}@${version}; verifying registry state..."
    if check_published "${name}" "${version}"; then
      rm -f "${tmp_stderr}"
      echo "${name}@${version} already on registry at the expected version; treating as success"
      echo "::endgroup::"
      continue
    fi

    rm -f "${tmp_stderr}"
    explain_trusted_publishing_failure "${name}" "${version}"
    exit 1
  fi

  rm -f "${tmp_stderr}"
  echo "npm publish failed for ${name}@${version} (rc=${publish_rc})" >&2
  exit "${publish_rc}"
done
