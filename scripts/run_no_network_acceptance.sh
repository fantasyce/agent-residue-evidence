#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"; repo_dir="$(cd "$script_dir/.." && pwd -P)"
set +e
git -C "$repo_dir" grep -n -E -I -e \
  '"net/http"|"net/rpc"|grpc\.Dial|net\.Dial|net\.Listen|http\.NewRequest|http\.Get' -- \
  cmd internal ':(exclude,glob)**/*_test.go'
scan_status=$?
set -e
case "$scan_status" in
  0) echo 'network-capable runtime source found' >&2; exit 1 ;;
  1) ;;
  *) echo "source scan failed with status $scan_status" >&2; exit "$scan_status" ;;
esac
task_base="${TMPDIR:-/tmp}"; task_base="${task_base%/}"; test_root="$(mktemp -d "$task_base/are-no-network.XXXXXX")"; pid=""; launcher_pid=""
cleanup(){
  if [[ -n "$launcher_pid" ]]; then : > "$test_root/stop"; wait "$launcher_pid" 2>/dev/null || true; launcher_pid=""; fi
  case "$test_root" in "$task_base"/are-no-network.*) find "$test_root" -depth -delete 2>/dev/null || true;; esac
}
trap cleanup EXIT INT TERM
GOPROXY=off GOSUMDB=off go build -C "$repo_dir" -trimpath -o "$test_root/are" ./cmd/agent-residue-evidence
python3 - "$test_root/are" "$test_root/state" "$test_root/mcp.pid" "$test_root/stop" "$test_root/stdout" "$test_root/stderr" <<'PY' &
import os, pathlib, subprocess, sys, time
binary, state, pid_path, stop_path, stdout_path, stderr_path = sys.argv[1:]
env = dict(os.environ, ARE_HOME=state, HTTP_PROXY="http://127.0.0.1:1", HTTPS_PROXY="http://127.0.0.1:1")
with open(stdout_path, "wb") as stdout, open(stderr_path, "wb") as stderr:
    process = subprocess.Popen([binary, "mcp"], stdin=subprocess.PIPE, stdout=stdout, stderr=stderr, env=env)
    pathlib.Path(pid_path).write_text(str(process.pid), encoding="ascii")
    while process.poll() is None and not pathlib.Path(stop_path).exists():
        time.sleep(0.05)
    if process.poll() is None:
        process.stdin.close()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()
    raise SystemExit(process.returncode)
PY
launcher_pid=$!
for _ in {1..100}; do [[ -s "$test_root/mcp.pid" ]] && break; sleep 0.05; done
[[ -s "$test_root/mcp.pid" ]] || { echo 'MCP process did not start' >&2; exit 1; }
pid="$(<"$test_root/mcp.pid")"
python3 - "$pid" <<'PY'
import os, sys
os.kill(int(sys.argv[1]), 0)
PY
case "$(go env GOOS)" in
  darwin|linux)
    command -v lsof >/dev/null 2>&1 || { echo 'lsof is required for native socket observation' >&2; exit 1; }
    if lsof -Pan -a -p "$pid" -i 2>/dev/null | grep -q .; then echo 'ARE opened a network socket' >&2; exit 1; fi
    ;;
  windows)
    command -v powershell.exe >/dev/null 2>&1 || { echo 'PowerShell is required for native socket observation' >&2; exit 1; }
    set +e
    powershell.exe -NoProfile -NonInteractive -Command \
      "\$connections = @(Get-NetTCPConnection -OwningProcess $pid -ErrorAction SilentlyContinue; Get-NetUDPEndpoint -OwningProcess $pid -ErrorAction SilentlyContinue); if (\$connections.Count -gt 0) { exit 42 }"
    socket_status=$?
    set -e
    case "$socket_status" in
      0) ;;
      42) echo 'ARE opened a network socket' >&2; exit 1 ;;
      *) echo "native socket observation failed with status $socket_status" >&2; exit "$socket_status" ;;
    esac
    ;;
  *) echo 'unsupported native socket observation target' >&2; exit 1 ;;
esac
: > "$test_root/stop"; wait "$launcher_pid"; launcher_pid=""; pid=""
test ! -s "$test_root/stderr"
trap - EXIT INT TERM; cleanup; test ! -e "$test_root"
echo 'NO_NETWORK_ACCEPTANCE=PASS clients=ABSENT listeners=ABSENT telemetry=ABSENT'
