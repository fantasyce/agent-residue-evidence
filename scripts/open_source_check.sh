#!/usr/bin/env bash
set -euo pipefail
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
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
required=(README.md LICENSE NOTICE SECURITY.md CONTRIBUTING.md CHANGELOG.md VERSION docs/quickstart.md docs/install.md docs/event-contract.md docs/report-contract.md docs/threat-model.md docs/native-acceptance.md)
for path in "${required[@]}"; do [[ -s "$repo_dir/$path" ]] || { echo "missing public file: $path" >&2; exit 1; }; done
for tool in begin_task_observation append_task_events end_task_observation inspect_completed_task get_residue_report verify_task_residue; do
  grep -Fq "$tool" "$repo_dir/README.md" || { echo "README missing tool: $tool" >&2; exit 1; }
done
for phrase in 'user authorizes cleanup' 'never deletes files' 'the network' 'NO_CANDIDATES_OBSERVED' 'PARTIAL_EVIDENCE' 'macOS 14+ arm64' 'Linux amd64' 'Windows 11'; do
  grep -Fq "$phrase" "$repo_dir/README.md" || { echo "README missing boundary: $phrase" >&2; exit 1; }
done
grep -Fq 'APPENDIX: How to apply the Apache License' "$repo_dir/LICENSE"
grep -Fq 'Copyright 2026 Agent Residue Evidence contributors' "$repo_dir/NOTICE"
reject_git_grep 'pipe-to-shell install instruction found' \
  'curl.*\|.*(sh|bash)|wget.*\|.*(sh|bash)' README.md docs
reject_git_grep 'private path or credential-like content found' \
  '/Users/[^/]+/|[A-Za-z]:\\Users\\|BEGIN (RSA |OPENSSH |EC )?PRIVATE KEY|AKIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9]{20,}' \
  . ':(exclude,glob)docs/superpowers/**' ':(exclude)scripts/open_source_check.sh' \
  ':(exclude)scripts/run_privacy_acceptance.sh' ':(exclude)scripts/verify_release_assets.sh'
bash "$repo_dir/scripts/test_plugin_surface.sh"
echo 'OPEN_SOURCE_CHECK=PASS'
