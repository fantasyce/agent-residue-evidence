#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import json
import re
from pathlib import Path
from urllib.parse import urlparse


VERSION = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+$")
SHA256 = re.compile(r"^[a-f0-9]{64}$")


def https(value: str) -> bool:
    parsed = urlparse(value)
    return parsed.scheme == "https" and bool(parsed.netloc)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--server", required=True, type=Path)
    parser.add_argument("--mcpb", required=True, type=Path)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()
    if not VERSION.fullmatch(args.version) or not args.server.is_file() or not args.mcpb.is_file():
        raise SystemExit("invalid Registry verification input")
    document = json.loads(args.server.read_text(encoding="utf-8"))
    expected_keys = {"$schema", "name", "title", "description", "version", "websiteUrl", "repository", "packages"}
    assert set(document) == expected_keys
    assert document["$schema"] == "https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json"
    assert document["name"] == "io.github.fantasyce/agent-residue-evidence"
    assert document["title"] == "Agent Residue Evidence"
    assert document["version"] == args.version
    assert 1 <= len(document["description"]) <= 100
    assert document["websiteUrl"] == "https://fantasyce.github.io/agent-residue-evidence/"
    assert https(document["websiteUrl"])
    assert document["repository"] == {
        "id": "1348411114",
        "source": "github",
        "url": "https://github.com/fantasyce/agent-residue-evidence",
    }
    package = document["packages"][0]
    assert package["registryType"] == "mcpb"
    assert package["version"] == args.version
    assert package["transport"] == {"type": "stdio"}
    expected_url = f"https://github.com/fantasyce/agent-residue-evidence/releases/download/v{args.version}/agent-residue-evidence_{args.version}.mcpb"
    assert package["identifier"] == expected_url and https(package["identifier"])
    expected_digest = hashlib.sha256(args.mcpb.read_bytes()).hexdigest()
    assert SHA256.fullmatch(package["fileSha256"])
    assert package["fileSha256"] == expected_digest


if __name__ == "__main__":
    main()
