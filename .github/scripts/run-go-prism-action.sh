#!/usr/bin/env bash
set -euo pipefail

bool_true() {
  case "${1:-}" in
    1 | true | TRUE | True | yes | YES | Yes | on | ON | On) return 0 ;;
    *) return 1 ;;
  esac
}

resolve_path() {
  local base="$1"
  local value="$2"

  if [[ -z "$value" ]]; then
    return 1
  fi

  if [[ "$value" = /* ]]; then
    printf '%s\n' "$value"
  else
    printf '%s/%s\n' "$base" "$value"
  fi
}

event_value() {
  local query="$1"

  if [[ -z "${GITHUB_EVENT_PATH:-}" || ! -f "$GITHUB_EVENT_PATH" ]]; then
    return 1
  fi

  if ! command -v jq >/dev/null 2>&1; then
    return 1
  fi

  jq -r "$query // empty" "$GITHUB_EVENT_PATH"
}

action_path="${GITHUB_ACTION_PATH:-$(pwd)}"
workspace="${GITHUB_WORKSPACE:-$(pwd)}"

workdir="${GO_PRISM_WORKDIR:-}"
if [[ -z "$workdir" ]]; then
  workdir="$workspace"
else
  workdir="$(resolve_path "$workspace" "$workdir")"
fi

base="${GO_PRISM_BASE:-origin/main}"
head="${GO_PRISM_HEAD:-HEAD}"
format="${GO_PRISM_FORMAT:-markdown}"
timeout="${GO_PRISM_TIMEOUT:-30s}"
output="${GO_PRISM_OUTPUT:-go-prism-report.md}"
report_path="$(resolve_path "$workspace" "$output")"

mkdir -p "$(dirname "$report_path")"

config="${GO_PRISM_CONFIG:-.go-prism.yml}"
config_arg=""
if [[ -n "$config" ]]; then
  config_path="$(resolve_path "$workspace" "$config")"
  if [[ -f "$config_path" ]]; then
    config_arg="$config_path"
  elif [[ "$config" == ".go-prism.yml" ]]; then
    config_arg=""
  else
    echo "go-prism config file not found: $config_path" >&2
    exit 2
  fi
fi

go_prism_args=(
  pr
  --workdir "$workdir"
  --base "$base"
  --head "$head"
  --config "$config_arg"
  --format "$format"
  --output "$report_path"
  --timeout "$timeout"
)

if [[ -n "${GO_PRISM_MODULE:-}" ]]; then
  go_prism_args+=(--module "$GO_PRISM_MODULE")
fi

if bool_true "${GO_PRISM_PREFLIGHT_DOCTOR:-true}"; then
  doctor_args=(
    doctor
    --workdir "$workdir"
    --base "$base"
    --head "$head"
    --config "$config_arg"
    --format text
    --timeout "$timeout"
  )

  if [[ -n "${GO_PRISM_MODULE:-}" ]]; then
    doctor_args+=(--module "$GO_PRISM_MODULE")
  fi

  echo "::group::Go Prism doctor"
  set +e
  (
    cd "$action_path"
    go run ./cmd/go-prism "${doctor_args[@]}"
  )
  doctor_status=$?
  set -e
  echo "::endgroup::"
  if [[ "$doctor_status" -ne 0 ]]; then
    exit "$doctor_status"
  fi
fi

(
  cd "$action_path"
  go run ./cmd/go-prism "${go_prism_args[@]}"
)

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  echo "report-path=$report_path" >> "$GITHUB_OUTPUT"
fi

if bool_true "${GO_PRISM_WRITE_SUMMARY:-true}"; then
  if [[ -z "${GITHUB_STEP_SUMMARY:-}" ]]; then
    echo "GITHUB_STEP_SUMMARY is not set; skipping summary output"
  else
    cat "$report_path" >> "$GITHUB_STEP_SUMMARY"
  fi
fi

if ! bool_true "${GO_PRISM_STICKY_COMMENT:-false}"; then
  exit 0
fi

if [[ "$format" != "markdown" ]]; then
  echo "sticky-comment requires format=markdown" >&2
  exit 2
fi

repo="${GITHUB_REPOSITORY:-}"
if [[ -z "$repo" ]]; then
  echo "GITHUB_REPOSITORY is not set; skipping sticky comment"
  exit 0
fi

event_name="${GITHUB_EVENT_NAME:-}"
if [[ "$event_name" != "pull_request" && "$event_name" != "pull_request_target" ]]; then
  echo "Event is not a pull request; skipping sticky comment"
  exit 0
fi

head_repo="$(event_value '.pull_request.head.repo.full_name' || true)"
if [[ -n "$head_repo" && "$head_repo" != "$repo" ]]; then
  echo "Pull request is from $head_repo, not $repo; skipping sticky comment"
  exit 0
fi

pr_number="${GO_PRISM_PR_NUMBER:-}"
if [[ -z "$pr_number" ]]; then
  pr_number="$(event_value '.pull_request.number' || true)"
fi

if [[ -z "$pr_number" ]]; then
  echo "Pull request number is required for sticky comments" >&2
  exit 2
fi

token="${GO_PRISM_GITHUB_TOKEN:-${GH_TOKEN:-${GITHUB_TOKEN:-}}}"
if [[ -z "$token" ]]; then
  echo "github-token, GH_TOKEN, or GITHUB_TOKEN is required for sticky comments" >&2
  exit 2
fi

GO_PRISM_COMMENT_MARKER="${GO_PRISM_COMMENT_MARKER:-<!-- go-prism:report -->}" \
GITHUB_REPOSITORY="$repo" \
PR_NUMBER="$pr_number" \
REPORT_FILE="$report_path" \
GH_TOKEN="$token" \
bash "$action_path/.github/scripts/upsert-go-prism-comment.sh"
