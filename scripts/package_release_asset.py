#!/usr/bin/env python3
from __future__ import annotations

import argparse
import gzip
import io
import stat
import tarfile
import tempfile
import zipfile
from pathlib import Path


def members(source: Path) -> list[Path]:
    return sorted((item for item in source.rglob("*") if item.is_file()), key=lambda item: item.relative_to(source).as_posix())


def mode_for(path: Path) -> int:
    return 0o755 if path.stat().st_mode & stat.S_IXUSR else 0o644


def atomic_output(output: Path, writer) -> None:
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(prefix=f".{output.name}.", dir=output.parent, delete=False) as handle:
        temporary = Path(handle.name)
    try:
        writer(temporary)
        temporary.replace(output)
    finally:
        if temporary.exists():
            temporary.unlink()


def write_tar(source: Path, root_name: str, output: Path) -> None:
    def writer(temporary: Path) -> None:
        buffer = io.BytesIO()
        with tarfile.open(fileobj=buffer, mode="w", format=tarfile.USTAR_FORMAT) as archive:
            for path in members(source):
                data = path.read_bytes()
                info = tarfile.TarInfo(f"{root_name}/{path.relative_to(source).as_posix()}")
                info.size, info.mode, info.mtime = len(data), mode_for(path), 0
                info.uid = info.gid = 0
                info.uname = info.gname = "root"
                archive.addfile(info, io.BytesIO(data))
        with temporary.open("wb") as handle:
            with gzip.GzipFile(filename="", mode="wb", fileobj=handle, mtime=0, compresslevel=9) as compressed:
                compressed.write(buffer.getvalue())
    atomic_output(output, writer)


def write_zip(source: Path, root_name: str, output: Path) -> None:
    def writer(temporary: Path) -> None:
        with zipfile.ZipFile(temporary, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
            for path in members(source):
                info = zipfile.ZipInfo(f"{root_name}/{path.relative_to(source).as_posix()}", (1980, 1, 1, 0, 0, 0))
                info.compress_type, info.create_system = zipfile.ZIP_DEFLATED, 3
                info.external_attr = (stat.S_IFREG | mode_for(path)) << 16
                archive.writestr(info, path.read_bytes())
    atomic_output(output, writer)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--format", required=True, choices=("tar.gz", "zip"))
    parser.add_argument("--source", required=True, type=Path)
    parser.add_argument("--root-name", required=True)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    if not args.source.is_dir() or not args.root_name or "/" in args.root_name or "\\" in args.root_name:
        raise SystemExit("invalid package input")
    (write_tar if args.format == "tar.gz" else write_zip)(args.source, args.root_name, args.output)


if __name__ == "__main__":
    main()
