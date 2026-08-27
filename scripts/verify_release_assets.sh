#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 3 ]]; then echo 'usage: verify_release_assets.sh DIST VERSION COMMIT' >&2; exit 64; fi
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
dist="$(cd "$1" && pwd -P)"
version="$2"
commit="$3"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ && "$commit" =~ ^[a-f0-9]{40}$ ]] || exit 65
for name in \
  "agent-residue-evidence_${version}_darwin_arm64.tar.gz" \
  "agent-residue-evidence_${version}_linux_amd64.tar.gz" \
  "agent-residue-evidence_${version}_windows_amd64.zip" \
  "agent-residue-evidence_${version}.mcpb" server.json sbom.spdx.json SHA256SUMS; do
  [[ -f "$dist/$name" ]] || { echo "missing release asset: $name" >&2; exit 1; }
done
(cd "$dist" && shasum -a 256 -c SHA256SUMS)
python3 - "$repo_dir" "$dist" "$version" "$commit" <<'PY'
import gzip, hashlib, io, json, pathlib, re, stat, sys, tarfile, zipfile
repo, dist, version, commit = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2]), sys.argv[3], sys.argv[4]
targets = {
  "darwin_arm64": ("tar.gz", "agent-residue-evidence"),
  "linux_amd64": ("tar.gz", "agent-residue-evidence"),
  "windows_amd64": ("zip", "agent-residue-evidence.exe"),
}
binary_bytes = {}
for target, (extension, binary_name) in targets.items():
    archive_path = dist / f"agent-residue-evidence_{version}_{target}.{extension}"
    root = f"agent-residue-evidence_{version}_{target}"
    if extension == "zip":
        with zipfile.ZipFile(archive_path) as archive:
            names = archive.namelist()
            assert names == sorted(names)
            assert all(info.date_time == (1980,1,1,0,0,0) for info in archive.infolist())
            binary_bytes[target] = archive.read(f"{root}/{binary_name}")
            provenance = json.loads(archive.read(f"{root}/provenance.json"))
    else:
        raw = archive_path.read_bytes()
        assert int.from_bytes(raw[4:8], "little") == 0
        with tarfile.open(archive_path, "r:gz") as archive:
            members = archive.getmembers()
            assert [m.name for m in members] == sorted(m.name for m in members)
            assert all(m.mtime == 0 and m.uid == 0 and m.gid == 0 for m in members)
            binary_bytes[target] = archive.extractfile(f"{root}/{binary_name}").read()
            provenance = json.load(archive.extractfile(f"{root}/provenance.json"))
    assert provenance["version"] == version and provenance["commit"] == commit and provenance["target"] == target
    assert provenance["binary_sha256"] == hashlib.sha256(binary_bytes[target]).hexdigest()

bundle = dist / f"agent-residue-evidence_{version}.mcpb"
with zipfile.ZipFile(bundle) as archive:
    names = archive.namelist()
    assert names == sorted(names)
    assert all(info.date_time == (1980,1,1,0,0,0) for info in archive.infolist())
    manifest = json.loads(archive.read("manifest.json"))
    assert manifest["manifest_version"] == "0.3" and manifest["version"] == version
    assert manifest["license"] == "Apache-2.0"
    assert manifest["compatibility"]["platforms"] == ["darwin", "linux", "win32"]
    assert len(manifest["tools"]) == 8
    assert "permissions" not in manifest and manifest["server"]["mcp_config"]["env"] == {}
    assert archive.read("server/agent-residue-evidence-darwin-arm64") == binary_bytes["darwin_arm64"]
    assert archive.read("server/agent-residue-evidence-linux-amd64") == binary_bytes["linux_amd64"]
    assert archive.read("server/agent-residue-evidence-windows-amd64.exe") == binary_bytes["windows_amd64"]
    payload = b"\n".join(archive.read(name) for name in names)
    forbidden = re.compile(rb"/Users/[^/]+/|[A-Za-z]:\\Users\\|BEGIN (?:RSA |OPENSSH |EC )?PRIVATE KEY|__pycache__|node_modules|/\.git/")
    assert not forbidden.search(payload)
registry = json.loads((dist / "server.json").read_text())
package = registry["packages"][0]
assert registry["version"] == version and package["version"] == version
assert package["fileSha256"] == hashlib.sha256(bundle.read_bytes()).hexdigest()
assert package["transport"] == {"type":"stdio"}
sbom = json.loads((dist / "sbom.spdx.json").read_text())
assert sbom["spdxVersion"] == "SPDX-2.3" and sbom["packages"]
PY
