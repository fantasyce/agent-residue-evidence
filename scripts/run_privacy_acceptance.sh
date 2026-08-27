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

go_os="$(go env GOOS)"
task_base="${TMPDIR:-/tmp}"; task_base="${task_base%/}"
if [[ "$go_os" == windows ]]; then task_base="$(cygpath -u "$task_base")"; fi
native_path() { if [[ "$go_os" == windows ]]; then cygpath -m "$1"; else printf '%s\n' "$1"; fi; }
test_root="$(mktemp -d "$task_base/are-privacy-acceptance.XXXXXX")"
cleanup() { case "$test_root" in "$task_base"/are-privacy-acceptance.*) find "$test_root" -depth -delete 2>/dev/null || true ;; *) return 1 ;; esac; }
trap cleanup EXIT INT TERM
binary="$test_root/agent-residue-evidence"
GOPROXY=off GOSUMDB=off go build -C "$repo_dir" -trimpath -o "$binary" ./cmd/agent-residue-evidence
workspace="$test_root/private-workspace-marker"; mkdir -p "$workspace"
state_native="$(native_path "$test_root/state")"; workspace_native="$(native_path "$workspace")"
ARE_HOME="$state_native" "$binary" begin > "$test_root/begin.json" <<JSON
{"task_id":"private-task-marker","workspace":"$workspace_native"}
JSON
owner_handle="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["owner_handle"])' "$test_root/begin.json")"
printf 'private-content-marker' > "$workspace/private-candidate-marker.log"
ARE_HOME="$state_native" "$binary" end > "$test_root/end.json" <<JSON
{"owner_handle":"$owner_handle"}
JSON
python3 - "$test_root/state" "$owner_handle" <<'PY'
import pathlib, sys
root=pathlib.Path(sys.argv[1]); payload=b"\n".join(path.read_bytes() for path in root.rglob("*") if path.is_file())
for marker in (b"private-task-marker", b"private-workspace-marker", b"private-candidate-marker", b"private-content-marker", sys.argv[2].encode()):
    assert marker not in payload, f"plaintext state marker found: {marker[:32]!r}"
for path in root.rglob("*"):
    assert "private-" not in path.name
PY
trap - EXIT INT TERM; cleanup; test ! -e "$test_root"
echo 'PRIVACY_ACCEPTANCE=PASS state=ENCRYPTED identifiers=OPAQUE raw_commands=ABSENT environments=ABSENT file_contents=ABSENT secrets=ABSENT'
