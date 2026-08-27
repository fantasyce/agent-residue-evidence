#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
tmp_root="${TMPDIR:-/tmp}"; tmp_root="${tmp_root%/}"; tmp_root="$(cd "$tmp_root" && pwd -P)"
run_dir="$(mktemp -d "$tmp_root/are-residue-demo.XXXXXX")"

cleanup() {
  case "$run_dir" in
    "$tmp_root"/are-residue-demo.*) find "$run_dir" -depth -delete 2>/dev/null || true ;;
    *) echo 'refusing unexpected demo cleanup path' >&2; return 1 ;;
  esac
}
trap cleanup EXIT INT TERM

json_only=false
cleanup_and_verify=false
for argument in "$@"; do
  case "$argument" in
    --json) json_only=true ;;
    --cleanup-and-verify) cleanup_and_verify=true ;;
    *) echo 'usage: demo_task_residue.sh [--json] [--cleanup-and-verify]' >&2; exit 64 ;;
  esac
done

mkdir -p "$run_dir/bin" "$run_dir/workspace"
GOPROXY=off GOSUMDB=off go build -C "$repo_dir" -trimpath \
  -o "$run_dir/bin/agent-residue-evidence" ./cmd/agent-residue-evidence
export ARE_HOME="$run_dir/state"

begin="$(
  printf '{"task_id":"are-residue-demo","workspace":"%s"}\n' "$run_dir/workspace" |
    "$run_dir/bin/agent-residue-evidence" begin
)"
owner_handle="$(printf '%s' "$begin" | python3 -c 'import json,sys; print(json.load(sys.stdin)["owner_handle"])')"
candidate="$run_dir/workspace/test-output.tmp"
printf 'harmless task-owned demo artifact\n' > "$candidate"
report="$(
  printf '{"owner_handle":"%s"}\n' "$owner_handle" |
    "$run_dir/bin/agent-residue-evidence" end
)"

verification=NOT_RUN
if $cleanup_and_verify; then
  rm "$candidate"
  verified="$(
    printf '{"owner_handle":"%s"}\n' "$owner_handle" |
      "$run_dir/bin/agent-residue-evidence" verify
  )"
  verification="$(
    printf '%s' "$verified" | python3 -c '
import json, sys
data=json.load(sys.stdin)
print("ABSENT" if data["candidates"] and all(item["current_status"] == "NO_LONGER_PRESENT" for item in data["candidates"]) else "PRESENT")
'
  )"
  printf '{"owner_handle":"%s"}\n' "$owner_handle" |
    "$run_dir/bin/agent-residue-evidence" report forget >/dev/null
fi

summary="$(
  printf '%s' "$report" | python3 -c '
import json, sys
data=json.load(sys.stdin)
result={
  "schema_version":"are-residue-demo/1.0",
  "observation":"BASELINE_OBSERVED" if any(item["evidence_level"] == "BASELINE_OBSERVED" for item in data["candidates"]) else "PARTIAL_EVIDENCE",
  "candidate_count":data["candidate_total"],
  "candidate_kinds":sorted({item["kind"] for item in data["candidates"]}),
  "cleanup_authorization_required":True,
  "are_cleanup_tool_present":False,
}
print(json.dumps(result, sort_keys=True, separators=(",", ":")))
'
)"
summary="$(
  printf '%s' "$summary" | DEMO_VERIFICATION="$verification" python3 -c '
import json, os, sys
data=json.load(sys.stdin)
data["verification"]=os.environ["DEMO_VERIFICATION"]
print(json.dumps(data, sort_keys=True, separators=(",", ":")))
'
)"

if $json_only; then
  printf '%s\n' "$summary"
else
  printf '%s' "$summary" | python3 -c '
import json, sys
data=json.load(sys.stdin)
print("Observation: " + data["observation"])
print("Candidates: " + str(data["candidate_count"]))
print("Kinds: " + ", ".join(data["candidate_kinds"]))
print("Cleanup authorization required: true")
print("ARE cleanup tool present: false")
if data["verification"] != "NOT_RUN":
    print("Verification: " + data["verification"])
print()
print("ARE reported scoped evidence and did not clean or authorize the candidate.")
'
fi
