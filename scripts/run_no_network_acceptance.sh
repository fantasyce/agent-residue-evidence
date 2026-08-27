#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"; repo_dir="$(cd "$script_dir/.." && pwd -P)"
if rg -n --glob '!**/*_test.go' --glob '!scripts/run_no_network_acceptance.sh' \
  '"net/http"|"net/rpc"|grpc\.Dial|net\.Dial|net\.Listen|http\.NewRequest|http\.Get' "$repo_dir/cmd" "$repo_dir/internal"; then
  echo 'network-capable runtime source found' >&2; exit 1
fi
task_base="${TMPDIR:-/tmp}"; task_base="${task_base%/}"; test_root="$(mktemp -d "$task_base/are-no-network.XXXXXX")"; pid=""
cleanup(){ if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then kill -KILL "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; fi; case "$test_root" in "$task_base"/are-no-network.*) find "$test_root" -depth -delete 2>/dev/null || true;; esac; }
trap cleanup EXIT INT TERM
GOPROXY=off GOSUMDB=off go build -C "$repo_dir" -trimpath -o "$test_root/are" ./cmd/agent-residue-evidence
fifo="$test_root/stdin"; mkfifo "$fifo"; exec 9<>"$fifo"; ARE_HOME="$test_root/state" HTTP_PROXY=http://127.0.0.1:1 HTTPS_PROXY=http://127.0.0.1:1 "$test_root/are" mcp <&9 >"$test_root/stdout" 2>"$test_root/stderr" & pid=$!
kill -0 "$pid"
if command -v lsof >/dev/null 2>&1 && lsof -Pan -a -p "$pid" -i 2>/dev/null | grep -q .; then echo 'ARE opened a network socket' >&2; exit 1; fi
kill -KILL "$pid"; wait "$pid" 2>/dev/null || true; pid=""; exec 9>&-
test ! -s "$test_root/stderr"
trap - EXIT INT TERM; cleanup; test ! -e "$test_root"
echo 'NO_NETWORK_ACCEPTANCE=PASS clients=ABSENT listeners=ABSENT telemetry=ABSENT'
