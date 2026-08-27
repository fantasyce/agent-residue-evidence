#!/usr/bin/env bash
set -euo pipefail
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"; task_base="${TMPDIR:-/tmp}"; task_base="${task_base%/}"
found="$(find "$task_base" -maxdepth 1 \( -name 'are-native-acceptance.*' -o -name 'are-no-network.*' -o -name 'are-packaging-test.*' -o -name 'are-install-test.*' -o -name 'are-cli-build.*' \) -print 2>/dev/null)"
[[ -z "$found" ]] || { echo "task-owned temporary residue remains: $found" >&2; exit 1; }
for path in "$repo_dir/build" "$repo_dir/dist"; do [[ ! -e "$path" ]] || { echo "generated residue remains: $path" >&2; exit 1; }; done
if pgrep -f '/are-(native-acceptance|no-network|install-test)\.[^/]*/.*agent-residue-evidence' >/dev/null 2>&1; then echo 'task-owned ARE process remains' >&2; exit 1; fi
echo 'ZERO_RESIDUE=PASS'
