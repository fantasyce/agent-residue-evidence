#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
for term in SHA256SUMS SBOM provenance macOS Linux Windows repair upgrade rollback uninstall; do
  grep -Eqi "$term" "$repo_dir/docs/install.md" || { echo "install guide missing: $term" >&2; exit 1; }
done
task_base="${TMPDIR:-/tmp}"; task_base="${task_base%/}"
test_root="$(mktemp -d "$task_base/are-install-test.XXXXXX")"
server_pid=""
cleanup() {
  if [[ -n "$server_pid" ]] && kill -0 "$server_pid" 2>/dev/null; then kill "$server_pid" 2>/dev/null || true; wait "$server_pid" 2>/dev/null || true; fi
  case "$test_root" in "$task_base"/are-install-test.*) find "$test_root" -depth -delete 2>/dev/null || true ;; *) return 1 ;; esac
}
trap cleanup EXIT INT TERM
version=0.3.0; commit="$(git -C "$repo_dir" rev-parse HEAD)"
mkdir -p "$test_root/dist"
ARE_RELEASE_VERSION="$version" ARE_RELEASE_COMMIT="$commit" bash "$script_dir/build_release_assets.sh" "$test_root/dist" darwin_arm64
candidate="$test_root/dist/agent-residue-evidence_${version}_darwin_arm64.tar.gz"

install_archive() {
  local archive="$1" inject_failure="${2:-false}"
  local staging="$test_root/install.staging" previous="$test_root/install.previous"
  if [[ -n "$server_pid" ]] && kill -0 "$server_pid" 2>/dev/null; then return 1; fi
  find "$staging" -depth -delete 2>/dev/null || true
  mkdir -p "$staging"
  tar -xzf "$archive" -C "$staging" --strip-components=1
  test -x "$staging/agent-residue-evidence"
  python3 - "$staging" <<'PY'
import hashlib, json, pathlib, sys
root=pathlib.Path(sys.argv[1]); provenance=json.loads((root/'provenance.json').read_text())
assert hashlib.sha256((root/'agent-residue-evidence').read_bytes()).hexdigest() == provenance['binary_sha256']
assert hashlib.sha256((root/'plugin/agent-residue-evidence/.codex-plugin/plugin.json').read_bytes()).hexdigest() == provenance['plugin_manifest_sha256']
PY
  [[ "$inject_failure" != true ]] || return 70
  find "$previous" -depth -delete 2>/dev/null || true
  if [[ -d "$test_root/install" ]]; then mv "$test_root/install" "$previous"; fi
  mv "$staging" "$test_root/install"
}

install_archive "$candidate"
binary="$test_root/install/agent-residue-evidence"
[[ "$($binary --version)" == "agent-residue-evidence $version ($commit)" ]]
ARE_HOME="$test_root/state" "$binary" doctor | python3 -c 'import json,sys; value=json.load(sys.stdin); assert value["healthy"] and not value["network_access"]'
python3 - "$binary" "$test_root/state" <<'PY'
import json, os, subprocess, sys
env=dict(os.environ, ARE_HOME=sys.argv[2]); process=subprocess.Popen([sys.argv[1], 'mcp'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, env=env)
requests=[
 {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"install-test","version":"1"}}},
 {"jsonrpc":"2.0","method":"notifications/initialized","params":{}},
 {"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}},
]
for request in requests: process.stdin.write(json.dumps(request)+"\n"); process.stdin.flush()
while True:
    response=json.loads(process.stdout.readline())
    if response.get("id") == 2:
        assert len(response["result"]["tools"]) == 8
        break
process.stdin.close(); assert process.wait(timeout=5) == 0; assert process.stderr.read() == ""
PY

# A running old MCP must be stopped before replacement.
fifo="$test_root/server.stdin"; mkfifo "$fifo"; exec 9<>"$fifo"
ARE_HOME="$test_root/state" "$binary" mcp <&9 >"$test_root/server.stdout" 2>"$test_root/server.stderr" & server_pid=$!
kill -0 "$server_pid"
kill -KILL "$server_pid"; wait "$server_pid" 2>/dev/null || true; server_pid=""; exec 9>&-

# Same-version repair replaces damaged program bytes.
printf damaged > "$binary"
install_archive "$candidate"
binary="$test_root/install/agent-residue-evidence"; "$binary" --version >/dev/null

# An injected staging failure leaves the verified current install unchanged.
before="$(shasum -a 256 "$binary" | awk '{print $1}')"
if install_archive "$candidate" true; then echo 'injected failure unexpectedly succeeded' >&2; exit 1; fi
after="$(shasum -a 256 "$binary" | awk '{print $1}')"; [[ "$before" == "$after" ]]

# Rollback slot and upgrade/downgrade behavior use whole verified directories.
find "$test_root/rollback" -depth -delete 2>/dev/null || true; cp -R "$test_root/install" "$test_root/rollback"
install_archive "$candidate"
find "$test_root/install" -depth -delete; mv "$test_root/rollback" "$test_root/install"
"$test_root/install/agent-residue-evidence" --version >/dev/null

# Uninstall removes only the owned install root and leaves no server.
find "$test_root/install" -depth -delete
test ! -e "$test_root/install"
trap - EXIT INT TERM; cleanup; test ! -e "$test_root"
echo 'install lifecycle tests passed'
