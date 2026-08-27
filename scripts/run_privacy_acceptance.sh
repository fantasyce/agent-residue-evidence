#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"; repo_dir="$(cd "$script_dir/.." && pwd -P)"
reject_git_grep() {
  local message="$1" pattern="$2" status
  shift 2
  set +e
  git -C "$repo_dir" grep -n -E -I -e "$pattern" -- "$@"
  status=$?
  set -e
  case "$status" in
    0) echo "$message" >&2; exit 1 ;;
    1) return 0 ;;
    *) echo "source scan failed with status $status" >&2; exit "$status" ;;
  esac
}
GOPROXY=off GOSUMDB=off go test -C "$repo_dir" -count=1 ./internal/contract ./internal/event ./internal/app
reject_git_grep 'privacy-forbidden runtime source found' \
  'os\.Environ|CommandLine|Cmdline|ReadFile\([^)]*candidate|raw_command|environment_values' \
  internal ':(exclude,glob)**/*_test.go'
reject_git_grep 'credential-like source content found' \
  'BEGIN (RSA |OPENSSH |EC )?PRIVATE KEY|AKIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9]{20,}' \
  . ':(exclude,glob)docs/superpowers/**' ':(exclude)scripts/run_privacy_acceptance.sh'
echo 'PRIVACY_ACCEPTANCE=PASS raw_commands=ABSENT environments=ABSENT file_contents=ABSENT secrets=ABSENT'
