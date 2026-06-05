#!/usr/bin/env bash
set -euo pipefail

marker="${GO_PRISM_COMMENT_MARKER:-<!-- go-prism:report -->}"
report_file="${REPORT_FILE:-go-prism-report.md}"
repo="${GITHUB_REPOSITORY:-}"
pr_number="${PR_NUMBER:-}"
dry_run="${GO_PRISM_COMMENT_DRY_RUN:-}"

if [[ -z "$repo" ]]; then
  echo "GITHUB_REPOSITORY is required" >&2
  exit 2
fi

if [[ -z "$pr_number" ]]; then
  echo "PR_NUMBER is required" >&2
  exit 2
fi

if [[ ! -s "$report_file" ]]; then
  echo "Report file is missing or empty: $report_file" >&2
  exit 2
fi

body_file="$(mktemp)"
comments_file="$(mktemp)"
trap 'rm -f "$body_file" "$comments_file"' EXIT

{
  echo "$marker"
  echo
  echo "_Updated by [go-prism](https://github.com/Athena900/go-prism). This comment is replaced on each workflow run._"
  echo
  cat "$report_file"
} > "$body_file"

if [[ "$dry_run" == "1" || "$dry_run" == "true" ]]; then
  echo "Dry run: would upsert Go Prism comment on ${repo}#${pr_number}"
  echo "Body bytes: $(wc -c < "$body_file" | tr -d ' ')"
  exit 0
fi

if [[ -z "${GH_TOKEN:-${GITHUB_TOKEN:-}}" ]]; then
  echo "GH_TOKEN or GITHUB_TOKEN is required" >&2
  exit 2
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "gh is required" >&2
  exit 2
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 2
fi

gh api "repos/${repo}/issues/${pr_number}/comments" --paginate --slurp > "$comments_file"

comment_id="$(
  jq -r --arg marker "$marker" \
    '[.[][] | select((.body // "") | contains($marker)) | .id] | last // ""' \
    "$comments_file"
)"

if [[ -n "$comment_id" ]]; then
  gh api \
    --method PATCH \
    "repos/${repo}/issues/comments/${comment_id}" \
    -F "body=@${body_file}" \
    --silent
  echo "Updated Go Prism PR comment: ${comment_id}"
else
  gh api \
    --method POST \
    "repos/${repo}/issues/${pr_number}/comments" \
    -F "body=@${body_file}" \
    --silent
  echo "Created Go Prism PR comment"
fi
