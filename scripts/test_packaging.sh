#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
task_base="${TMPDIR:-/tmp}"; task_base="${task_base%/}"
test_root="$(mktemp -d "$task_base/are-packaging-test.XXXXXX")"
cleanup() { case "$test_root" in "$task_base"/are-packaging-test.*) find "$test_root" -depth -delete 2>/dev/null || true ;; *) return 1 ;; esac; }
trap cleanup EXIT INT TERM
version=0.3.0
commit="$(git -C "$repo_dir" rev-parse HEAD)"
mkdir -p "$test_root/dist-one" "$test_root/dist-two"
ARE_RELEASE_VERSION="$version" ARE_RELEASE_COMMIT="$commit" bash "$script_dir/build_release_assets.sh" "$test_root/dist-one" darwin_arm64
ARE_RELEASE_VERSION="$version" ARE_RELEASE_COMMIT="$commit" bash "$script_dir/build_release_assets.sh" "$test_root/dist-two" darwin_arm64
cmp "$test_root/dist-one/agent-residue-evidence_${version}_darwin_arm64.tar.gz" "$test_root/dist-two/agent-residue-evidence_${version}_darwin_arm64.tar.gz"
python3 - "$repo_dir" "$test_root" "$version" "$commit" <<'PY'
import json, pathlib, shutil, stat, subprocess, sys, tarfile
repo, root, version, commit = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2]), sys.argv[3], sys.argv[4]
for dist_name in ("dist-one", "dist-two"):
    dist = root / dist_name
    darwin = dist / f"agent-residue-evidence_{version}_darwin_arm64.tar.gz"
    with tarfile.open(darwin, "r:gz") as archive:
        binary = archive.extractfile(f"agent-residue-evidence_{version}_darwin_arm64/agent-residue-evidence").read()
    for target, extension, binary_name in (("linux_amd64","tar.gz","agent-residue-evidence"),("windows_amd64","zip","agent-residue-evidence.exe")):
        source = root / f"fixture-{dist_name}-{target}"
        source.mkdir()
        path = source / binary_name
        path.write_bytes(binary); path.chmod(0o755)
        shutil.copy2(repo / "LICENSE", source / "LICENSE")
        shutil.copytree(repo / "plugin/agent-residue-evidence", source / "plugin/agent-residue-evidence")
        provenance = {"schema_version":"are-install-provenance/1.0","version":version,"commit":commit,"target":target,
                      "binary_sha256":__import__("hashlib").sha256(binary).hexdigest(),
                      "plugin_manifest_sha256":__import__("hashlib").sha256((repo/"plugin/agent-residue-evidence/.codex-plugin/plugin.json").read_bytes()).hexdigest()}
        (source/"provenance.json").write_text(json.dumps(provenance, sort_keys=True, separators=(",", ":"))+"\n")
        output = dist / f"agent-residue-evidence_{version}_{target}.{extension}"
        subprocess.run([sys.executable, str(repo/"scripts/package_release_asset.py"), "--format", extension, "--source", str(source), "--root-name", f"agent-residue-evidence_{version}_{target}", "--output", str(output)], check=True)
    subprocess.run([sys.executable, str(repo/"scripts/build_mcpb.py"), "--dist", str(dist), "--version", version, "--commit", commit], check=True)
PY
cmp "$test_root/dist-one/agent-residue-evidence_${version}.mcpb" "$test_root/dist-two/agent-residue-evidence_${version}.mcpb"
ARE_RELEASE_VERSION="$version" ARE_RELEASE_COMMIT="$commit" bash "$script_dir/build_release_assets.sh" "$test_root/dist-one" darwin_arm64
bash "$script_dir/verify_release_assets.sh" "$test_root/dist-one" "$version" "$commit"
trap - EXIT INT TERM; cleanup
test ! -e "$test_root"
echo 'packaging tests passed'
