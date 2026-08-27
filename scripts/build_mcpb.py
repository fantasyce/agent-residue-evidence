#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import io
import json
import os
import re
import stat
import tarfile
import tempfile
import zipfile
from pathlib import Path
from typing import Any

VERSION = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+$")
COMMIT = re.compile(r"^[a-f0-9]{40}$")
EPOCH = (1980, 1, 1, 0, 0, 0)


def render(value: Any, replacements: dict[str, str]) -> Any:
    if isinstance(value, str):
        for token, replacement in replacements.items():
            value = value.replace(token, replacement)
        return value
    if isinstance(value, list):
        return [render(item, replacements) for item in value]
    if isinstance(value, dict):
        return {key: render(item, replacements) for key, item in value.items()}
    return value


def archive_member(path: Path, name: str) -> bytes:
    if path.suffix == ".zip":
        with zipfile.ZipFile(path) as archive:
            return archive.read(name)
    with tarfile.open(path, "r:gz") as archive:
        handle = archive.extractfile(name)
        if handle is None:
            raise SystemExit(f"missing archive member: {name}")
        return handle.read()


def json_bytes(value: Any) -> bytes:
    return (json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False) + "\n").encode()


def zip_info(name: str, mode: int) -> zipfile.ZipInfo:
    info = zipfile.ZipInfo(name, EPOCH)
    info.compress_type, info.create_system = zipfile.ZIP_DEFLATED, 3
    info.external_attr = (stat.S_IFREG | mode) << 16
    return info


def atomic_bytes(path: Path, data: bytes) -> None:
    with tempfile.NamedTemporaryFile(prefix=f".{path.name}.", dir=path.parent, delete=False) as handle:
        temporary = Path(handle.name)
        handle.write(data)
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(temporary, path)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dist", required=True, type=Path)
    parser.add_argument("--version", required=True)
    parser.add_argument("--commit", required=True)
    args = parser.parse_args()
    if not args.dist.is_dir() or not VERSION.fullmatch(args.version) or not COMMIT.fullmatch(args.commit):
        raise SystemExit("invalid MCPB build input")
    repo, dist, version = Path(__file__).resolve().parent.parent, args.dist.resolve(), args.version
    targets = {
        "darwin_arm64": ("tar.gz", "agent-residue-evidence"),
        "linux_amd64": ("tar.gz", "agent-residue-evidence"),
        "windows_amd64": ("zip", "agent-residue-evidence.exe"),
    }
    files: dict[str, tuple[bytes, int]] = {
        "LICENSE": ((repo / "LICENSE").read_bytes(), 0o644),
        "assets/icon.svg": ((repo / "assets/icon.svg").read_bytes(), 0o644),
    }
    for target, (extension, binary_name) in targets.items():
        archive = dist / f"agent-residue-evidence_{version}_{target}.{extension}"
        root = f"agent-residue-evidence_{version}_{target}"
        output = f"server/agent-residue-evidence-{target.replace('_', '-')}"
        if target.startswith("windows"):
            output += ".exe"
        if not archive.is_file():
            raise SystemExit(f"missing MCPB source archive: {archive.name}")
        files[output] = (archive_member(archive, f"{root}/{binary_name}"), 0o755)
    plugin_files = {
        "plugin/.codex-plugin/plugin.json": repo / "plugin/agent-residue-evidence/.codex-plugin/plugin.json",
        "plugin/.mcp.json": repo / "plugin/agent-residue-evidence/.mcp.json",
        "plugin/skills/agent-residue-evidence/SKILL.md": repo / "plugin/agent-residue-evidence/skills/agent-residue-evidence/SKILL.md",
        "plugin/skills/agent-residue-evidence/agents/openai.yaml": repo / "plugin/agent-residue-evidence/skills/agent-residue-evidence/agents/openai.yaml",
    }
    for name, path in plugin_files.items():
        files[name] = (path.read_bytes(), 0o644)
    template = json.loads((repo / "packaging/mcpb/manifest.json.in").read_text())
    files["manifest.json"] = (json_bytes(render(template, {"@VERSION@": version, "@COMMIT@": args.commit})), 0o644)
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for name in sorted(files):
            archive.writestr(zip_info(name, files[name][1]), files[name][0])
    bundle = dist / f"agent-residue-evidence_{version}.mcpb"
    atomic_bytes(bundle, buffer.getvalue())
    registry = json.loads((repo / "packaging/mcp-registry/server.json.in").read_text())
    registry = render(registry, {"@VERSION@": version, "@MCPB_SHA256@": hashlib.sha256(bundle.read_bytes()).hexdigest()})
    atomic_bytes(dist / "server.json", json_bytes(registry))


if __name__ == "__main__":
    main()
