#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
plugin_dir="$repo_dir/plugin/agent-residue-evidence"

python3 - "$plugin_dir" <<'PY'
import json
import pathlib
import sys

plugin = pathlib.Path(sys.argv[1])
manifest = json.loads((plugin / ".codex-plugin" / "plugin.json").read_text())
mcp = json.loads((plugin / ".mcp.json").read_text())
skill = (plugin / "skills" / "agent-residue-evidence" / "SKILL.md").read_text()

assert manifest["name"] == "agent-residue-evidence"
assert manifest["mcpServers"] == "./.mcp.json"
assert manifest["skills"] == "./skills/"
assert "[TODO:" not in json.dumps(manifest) + skill
assert mcp == {"mcpServers": {"agent-residue-evidence": {"command": "agent-residue-evidence", "args": ["mcp"]}}}

required = {
    "begin_task_observation",
    "append_task_events",
    "end_task_observation",
    "inspect_completed_task",
    "get_residue_report",
    "verify_task_residue",
    "delegate_task_executor",
    "resolve_residue_candidate",
}
missing = sorted(name for name in required if name not in skill)
assert not missing, f"skill missing tools: {missing}"
for forbidden in ("automatic deletion", "full-disk scope", "environment values", "conversation transcripts"):
    assert forbidden in skill, f"skill missing boundary: {forbidden}"
for required_instruction in ("agent-task-event/2.0", "executor_handle", "owner_handle"):
    assert required_instruction in skill, f"skill missing capability instruction: {required_instruction}"
PY
