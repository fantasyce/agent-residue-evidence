#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
task_base="${TMPDIR:-/tmp}"; task_base="${task_base%/}"
test_root="$(mktemp -d "$task_base/are-native-acceptance.XXXXXX")"
cleanup() { case "$test_root" in "$task_base"/are-native-acceptance.*) find "$test_root" -depth -delete 2>/dev/null || true ;; *) return 1 ;; esac; }
trap cleanup EXIT INT TERM

go_os="$(go env GOOS)"; go_arch="$(go env GOARCH)"
case "${go_os}_${go_arch}" in darwin_arm64|linux_amd64|windows_amd64) ;; *) echo "unsupported native host: ${go_os}/${go_arch}" >&2; exit 1 ;; esac
commit="$(git -C "$repo_dir" rev-parse HEAD)"; version=0.1.0
binary="$test_root/agent-residue-evidence"; [[ "$go_os" == windows ]] && binary="$binary.exe"
GOPROXY=off GOSUMDB=off go build -C "$repo_dir" -trimpath \
  -ldflags "-s -w -X github.com/fantasyce/agent-residue-evidence/internal/versioninfo.Version=$version -X github.com/fantasyce/agent-residue-evidence/internal/versioninfo.Commit=$commit" \
  -o "$binary" ./cmd/agent-residue-evidence
GOPROXY=off GOSUMDB=off go test -C "$repo_dir" -count=1 -race \
  ./internal/app ./internal/correlate ./internal/event ./internal/fsobserve ./internal/process ./internal/scope ./internal/store

export ARE_HOME="$test_root/state"
workspace="$test_root/workspace"; task_temp="$test_root/task-temp"; mkdir -p "$workspace" "$task_temp"
printf '{"task_id":"native-standard","workspace":"%s","temp_roots":["%s"]}\n' "$workspace" "$task_temp" | "$binary" begin > "$test_root/begin.json"
mkdir -p "$workspace/generated"; printf 'ARE_PRIVATE_FIXTURE_DO_NOT_COPY' > "$workspace/generated/result.log"
fingerprint="sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
printf '{"task_id":"native-standard","events":[{"schema_version":"agent-task-event/1.0","task_id":"native-standard","event_id":"artifact-1","type":"artifact_declared","timestamp":"2026-08-27T10:00:00Z","working_directory":"%s","command_fingerprint":"%s","declared_outputs":["%s"]}]}\n' "$workspace" "$fingerprint" "$workspace/generated/result.log" | "$binary" event append > "$test_root/event.json"
printf '{"task_id":"native-standard"}\n' | "$binary" end > "$test_root/end.json"

# No-event prospective baseline remains a valid standard journey.
empty="$test_root/empty"; mkdir -p "$empty"
printf '{"task_id":"native-empty","workspace":"%s"}\n' "$empty" | "$binary" begin >/dev/null
printf '{"task_id":"native-empty"}\n' | "$binary" end > "$test_root/empty-report.json"

# Agent-owned cleanup occurs outside ARE, followed by candidate-only verify.
report_id="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["report_id"])' "$test_root/end.json")"
find "$workspace/generated" -depth -delete
printf '{"report_id":"%s"}\n' "$report_id" | "$binary" verify > "$test_root/verified.json"

# Retrospective inspection must remain downgraded and partial.
historical="$test_root/historical"; mkdir -p "$historical"; touch "$historical/old.log"
printf '{"scope":{"task_id":"native-retrospective","workspace":"%s"},"started_at":"2020-01-01T00:00:00Z","ended_at":"2030-01-01T00:00:00Z","events":[]}\n' "$historical" | "$binary" inspect-completed > "$test_root/retrospective.json"

python3 - "$test_root" <<'PY'
import json, pathlib, sys
root=pathlib.Path(sys.argv[1]); standard=json.loads((root/'end.json').read_text()); verified=json.loads((root/'verified.json').read_text())
empty=json.loads((root/'empty-report.json').read_text()); retro=json.loads((root/'retrospective.json').read_text())
expected_status = 'PARTIAL_EVIDENCE' if standard.get('limitations') else 'REVIEW_REQUIRED'
assert standard['status'] == expected_status and standard['candidates']
assert 'ARE_PRIVATE_FIXTURE_DO_NOT_COPY' not in (root/'end.json').read_text()
assert any(c['evidence_level']=='BASELINE_OBSERVED' for c in standard['candidates'])
assert all(c['current_status']=='NO_LONGER_PRESENT' for c in verified['candidates'] if c['kind'] in ('file','directory'))
assert empty['status'] == 'NO_CANDIDATES_OBSERVED'
assert retro['status'] == 'PARTIAL_EVIDENCE' and all(c['evidence_level']!='BASELINE_OBSERVED' for c in retro['candidates'])
PY

# Installed binary protocol surface and restart recovery.
python3 - "$binary" "$ARE_HOME" <<'PY'
import json, os, subprocess, sys
for _ in range(2):
    p=subprocess.Popen([sys.argv[1],'mcp'],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,env=dict(os.environ,ARE_HOME=sys.argv[2]))
    for request in ({"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"native","version":"1"}}},{"jsonrpc":"2.0","method":"notifications/initialized","params":{}},{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}):
        p.stdin.write(json.dumps(request)+'\n'); p.stdin.flush()
    while True:
        response=json.loads(p.stdout.readline())
        if response.get('id') == 2: assert len(response['result']['tools']) == 6; break
    p.stdin.close(); assert p.wait(timeout=5)==0 and p.stderr.read()==''
PY

# Broad-root rejection must be a hard error.
if printf '{"task_id":"native-broad","workspace":"/"}\n' | "$binary" begin >/dev/null 2>&1; then echo 'broad root accepted' >&2; exit 1; fi

binary_sha="$(shasum -a 256 "$binary" | awk '{print $1}')"
summary="${ARE_ACCEPTANCE_SUMMARY:-}"
if [[ -n "$summary" ]]; then
  python3 - "$summary" "$go_os" "$go_arch" "$(go version)" "$commit" "$binary_sha" "$report_id" <<'PY'
import json, pathlib, sys
pathlib.Path(sys.argv[1]).write_text(json.dumps({"schema_version":"are-native-acceptance/1.0","os":sys.argv[2],"arch":sys.argv[3],"go":sys.argv[4],"commit":sys.argv[5],"binary_sha256":sys.argv[6],"report_ids":[sys.argv[7]],"result":"PASS","residue":"NONE"},sort_keys=True)+"\n")
PY
fi
trap - EXIT INT TERM; cleanup; test ! -e "$test_root"
echo "NATIVE_ACCEPTANCE=PASS os=$go_os arch=$go_arch commit=$commit binary_sha256=$binary_sha residue=NONE"
