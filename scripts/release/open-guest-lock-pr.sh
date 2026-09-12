#!/usr/bin/env bash
# Always leave a reviewable branch when repository policy blocks bot PRs.
set -euo pipefail
: "${GITHUB_REPOSITORY:?}"
: "${GITHUB_RUN_ID:?}"
: "${GITHUB_RUN_ATTEMPT:?}"
: "${GITHUB_STEP_SUMMARY:?}"
branch="guest-lock/${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}"
git config user.name 'github-actions[bot]'
git config user.email '41898282+github-actions[bot]@users.noreply.github.com'
git checkout -b "$branch"
git add guest-build
git commit -m 'Refresh the guest package lock'
git push origin "$branch"
# GITHUB_TOKEN does not trigger pull_request CI; an explicit dispatch does.
if gh workflow run ci.yml --ref "$branch"; then
  printf 'CI requested for `%s`. Check its result before merging.\n\n' "$branch" >>"$GITHUB_STEP_SUMMARY"
else
  printf '::warning::Could not dispatch CI for the guest-lock branch.\n'
  printf 'CI was not started for `%s`; run it manually before merging.\n\n' "$branch" >>"$GITHUB_STEP_SUMMARY"
fi
body=$(mktemp)
trap 'rm -f "$body" "$body.error"' EXIT
cat >"$body" <<'BODY'
Refreshes the locked guest packages for the next image build.

Validation:
- Guest contract tests passed with the updated lock.
- A full image build and runtime validation are still required.

Draft for review. The refresh workflow requests CI on this branch; verify its result before merging.
BODY
if url=$(gh pr create --draft --title 'Refresh the guest package lock' --body-file "$body" --head "$branch" --base master 2>"$body.error"); then
  printf 'Guest package refresh: %s\n' "$url" >>"$GITHUB_STEP_SUMMARY"
  gh pr view "$url" --json title,body,isDraft,headRefName,baseRefName
else
  # Do not discard the successful refresh because an organization disables
  # PR creation by GITHUB_TOKEN. The branch is the supported fallback.
  printf '::warning::Guest lock was refreshed, but automatic draft PR creation failed. See the run summary.\n'
  printf 'Guest lock is ready for review on `%s`. No PR was created.\n\n' "$branch" >>"$GITHUB_STEP_SUMMARY"
  printf '[Open a draft PR](https://github.com/%s/compare/master...%s?expand=1)\n\n' "$GITHUB_REPOSITORY" "$branch" >>"$GITHUB_STEP_SUMMARY"
  printf 'Run the normal CI checks and image validation before merging.\n' >>"$GITHUB_STEP_SUMMARY"
  cat "$body.error" >&2
fi
rm -f "$body.error"
