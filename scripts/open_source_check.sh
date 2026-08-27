#!/usr/bin/env bash
set -euo pipefail
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
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
if rg -n 'curl[^\n]*\|[^\n]*(sh|bash)|wget[^\n]*\|[^\n]*(sh|bash)' "$repo_dir/README.md" "$repo_dir/docs"; then
  echo 'pipe-to-shell install instruction found' >&2; exit 1
fi
if rg -n '/Users/[^/]+/|[A-Za-z]:\\Users\\|BEGIN (RSA |OPENSSH |EC )?PRIVATE KEY|AKIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9]{20,}' "$repo_dir" \
  --glob '!docs/superpowers/**' --glob '!scripts/open_source_check.sh' --glob '!scripts/run_privacy_acceptance.sh' --glob '!scripts/verify_release_assets.sh'; then
  echo 'private path or credential-like content found' >&2; exit 1
fi
bash "$repo_dir/scripts/test_plugin_surface.sh"
echo 'OPEN_SOURCE_CHECK=PASS'
