#!/usr/bin/env bash
set -euo pipefail
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
version="$(tr -d '[:space:]' < "$repo_dir/VERSION")"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'VERSION is not stable semver' >&2; exit 1; }
python3 - "$repo_dir" "$version" <<'PY'
import json, pathlib, re, sys
repo, version = pathlib.Path(sys.argv[1]), sys.argv[2]
source=(repo/'internal/versioninfo/version.go').read_text()
assert re.search(r'Version\s*=\s*"'+re.escape(version)+r'"', source)
plugin=json.loads((repo/'plugin/agent-residue-evidence/.codex-plugin/plugin.json').read_text())
assert plugin['version'] == version
assert f'## {version} - 2026-08-27' in (repo/'CHANGELOG.md').read_text()
assert version in (repo/'docs/install.md').read_text()
manifest=(repo/'packaging/mcpb/manifest.json.in').read_text(); registry=(repo/'packaging/mcp-registry/server.json.in').read_text()
assert '@VERSION@' in manifest and '@COMMIT@' in manifest
assert '@VERSION@' in registry and '@MCPB_SHA256@' in registry
assert 'https://fantasyce.github.io/agent-residue-evidence/' in registry
workflow=(repo/'.github/workflows/quality.yml').read_text()
assert f'ARE_RELEASE_VERSION: {version}' in workflow
assert f'agent-residue-evidence_{version}_' in workflow
release=(repo/'.github/workflows/release.yml').read_text()
publish=(repo/'.github/workflows/publish-mcp.yml').read_text()
assert f'ARE_RELEASE_VERSION: {version}' in release
assert f'ref: v{version}' in publish
assert f'/versions/{version}' in publish
assert f'"release": "v{version}"' in (repo/'docs/launch/launch-manifest.json').read_text()
PY
output="$(cd "$repo_dir" && GOPROXY=off GOSUMDB=off go run ./cmd/agent-residue-evidence --version)"
[[ "$output" == "agent-residue-evidence $version (unknown)" ]] || { echo "binary version mismatch: $output" >&2; exit 1; }
echo "RELEASE_METADATA=PASS version=$version"
