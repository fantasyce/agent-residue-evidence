#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo 'usage: build_release_assets.sh OUTPUT_DIRECTORY [darwin_arm64|linux_amd64|windows_amd64]' >&2
  exit 64
fi
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
output_dir="$1"
target="${2:-$(go env GOOS)_$(go env GOARCH)}"
version="${ARE_RELEASE_VERSION:-$(tr -d '[:space:]' < "$repo_dir/VERSION")}"
commit="${ARE_RELEASE_COMMIT:-$(git -C "$repo_dir" rev-parse HEAD)}"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'release version must be stable semver' >&2; exit 65; }
[[ "$commit" =~ ^[a-f0-9]{40}$ ]] || { echo 'release commit must be a full Git SHA' >&2; exit 65; }
case "$target" in
  darwin_arm64) expected_os=darwin; expected_arch=arm64; extension=tar.gz; binary_name=agent-residue-evidence ;;
  linux_amd64) expected_os=linux; expected_arch=amd64; extension=tar.gz; binary_name=agent-residue-evidence ;;
  windows_amd64) expected_os=windows; expected_arch=amd64; extension=zip; binary_name=agent-residue-evidence.exe ;;
  *) echo "unsupported release target: $target" >&2; exit 65 ;;
esac
[[ "$(go env GOOS)" == "$expected_os" && "$(go env GOARCH)" == "$expected_arch" ]] || {
  echo "target $target must be built natively on ${expected_os}/${expected_arch}" >&2
  exit 65
}
mkdir -p "$output_dir"
output_dir="$(cd "$output_dir" && pwd -P)"
staging="$(mktemp -d "$output_dir/.are-release-${target}.XXXXXX")"
cleanup() {
  case "$staging" in "$output_dir"/.are-release-*) find "$staging" -depth -delete 2>/dev/null || true ;; esac
}
trap cleanup EXIT INT TERM
root_name="agent-residue-evidence_${version}_${target}"
root="$staging/$root_name"
mkdir -p "$root/plugin"
GOPROXY=off GOSUMDB=off go build -C "$repo_dir" -trimpath \
  -ldflags "-s -w -X github.com/fantasyce/agent-residue-evidence/internal/versioninfo.Version=$version -X github.com/fantasyce/agent-residue-evidence/internal/versioninfo.Commit=$commit" \
  -o "$root/$binary_name" ./cmd/agent-residue-evidence
cp "$repo_dir/LICENSE" "$root/LICENSE"
cp -R "$repo_dir/plugin/agent-residue-evidence" "$root/plugin/"
python3 - "$root" "$binary_name" "$version" "$commit" "$target" <<'PY'
import hashlib, json, pathlib, sys
root, binary_name, version, commit, target = pathlib.Path(sys.argv[1]), sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5]
plugin = root / "plugin/agent-residue-evidence/.codex-plugin/plugin.json"
data = {"schema_version":"are-install-provenance/1.0","version":version,"commit":commit,"target":target,
        "binary_sha256":hashlib.sha256((root / binary_name).read_bytes()).hexdigest(),
        "plugin_manifest_sha256":hashlib.sha256(plugin.read_bytes()).hexdigest()}
(root / "provenance.json").write_text(json.dumps(data, sort_keys=True, separators=(",", ":")) + "\n")
PY
python3 "$script_dir/package_release_asset.py" --format "$extension" --source "$root" --root-name "$root_name" \
  --output "$output_dir/${root_name}.${extension}"
if [[ -f "$output_dir/agent-residue-evidence_${version}_darwin_arm64.tar.gz" &&
      -f "$output_dir/agent-residue-evidence_${version}_linux_amd64.tar.gz" &&
      -f "$output_dir/agent-residue-evidence_${version}_windows_amd64.zip" ]]; then
  python3 "$script_dir/build_mcpb.py" --dist "$output_dir" --version "$version" --commit "$commit"
fi
python3 - "$repo_dir" "$output_dir" "$version" "$commit" <<'PY'
import hashlib, json, pathlib, re, sys
repo, dist, version, commit = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2]), sys.argv[3], sys.argv[4]
modules = [{"name":"github.com/fantasyce/agent-residue-evidence","versionInfo":version}]
for path, module_version in re.findall(r"^\s*([^\s()]+)\s+(v[^\s]+)(?:\s+// indirect)?$", (repo / "go.mod").read_text(), re.M):
    modules.append({"name":path,"versionInfo":module_version})
sbom = {"spdxVersion":"SPDX-2.3","dataLicense":"CC0-1.0","SPDXID":"SPDXRef-DOCUMENT","name":f"agent-residue-evidence-{version}","documentNamespace":f"https://github.com/fantasyce/agent-residue-evidence/releases/tag/v{version}#{commit}","packages":modules}
(dist / "sbom.spdx.json").write_text(json.dumps(sbom, sort_keys=True, separators=(",", ":")) + "\n")
assets = sorted(path for path in dist.iterdir() if path.is_file() and path.name != "SHA256SUMS")
(dist / "SHA256SUMS").write_text("".join(f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n" for path in assets))
PY
