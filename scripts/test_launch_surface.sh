#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
for path in \
  docs/demo.md docs/launch/launch-article.md docs/launch/faq.md \
  docs/launch/community-posts.md docs/launch/maintainer-outreach.md \
  docs/launch/showcase-submission.md docs/launch/launch-manifest.json \
  .github/workflows/pages.yml .github/workflows/release.yml \
  .github/workflows/publish-mcp.yml scripts/render_registry_metadata.py \
  scripts/verify_registry_metadata.py; do
  [[ -f "$repo_dir/$path" ]] || { echo "missing launch surface: $path" >&2; exit 1; }
done

python3 - "$repo_dir" <<'PY'
import json, pathlib, sys
repo = pathlib.Path(sys.argv[1])
manifest = json.loads((repo / "docs/launch/launch-manifest.json").read_text())
assert manifest["schema_version"] == 1
assert manifest["release"] == "v0.3.0"
channels = {item["id"]: item for item in manifest["channels"]}
assert {"github-release","mcp-registry","github-pages","github-discussion","design-partners"} <= channels.keys()
for channel in ("reddit","x","linkedin","show-hn","openai-developer-forum","chinese-community"):
    assert channels[channel]["status"] == "blocked"
    assert "owner" in channels[channel]["reason"].lower()

release = (repo / ".github/workflows/release.yml").read_text()
registry = (repo / ".github/workflows/publish-mcp.yml").read_text()
assert "git rev-parse origin/main" in release
assert "ref: v0.3.0" in registry
assert "login github-oidc" in registry
assert "io.github.fantasyce%2Fagent-residue-evidence/versions/0.3.0" in registry
for workflow in (release, registry):
    assert "actions/checkout@" in workflow
    assert "actions/checkout@v" not in workflow
PY

echo 'launch surface tests passed'
