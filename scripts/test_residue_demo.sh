#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
output="$(mktemp "${TMPDIR:-/tmp}/are-demo-result.XXXXXX")"
cleanup() {
  case "$output" in "${TMPDIR:-/tmp}"/are-demo-result.*) rm -f "$output" ;; *) return 1 ;; esac
}
trap cleanup EXIT INT TERM

bash "$repo_dir/scripts/demo_task_residue.sh" --json --cleanup-and-verify > "$output"
python3 - "$output" <<'PY'
import json, pathlib, sys
result = json.loads(pathlib.Path(sys.argv[1]).read_text())
assert result["schema_version"] == "are-residue-demo/1.0"
assert result["observation"] == "BASELINE_OBSERVED"
assert result["candidate_count"] >= 1
assert "file" in result["candidate_kinds"]
assert result["cleanup_authorization_required"] is True
assert result["are_cleanup_tool_present"] is False
assert result["verification"] == "ABSENT"
serialized = json.dumps(result, sort_keys=True)
for prohibited in ("/Users/", "/private/tmp/", "owner_handle", "exact_path"):
    assert prohibited not in serialized, prohibited
PY

if pgrep -f 'are-residue-demo\..*/demo-helper' >/dev/null 2>&1; then
  echo 'demo helper process leaked' >&2
  exit 1
fi

echo 'residue demo tests passed'
