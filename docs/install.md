# Install Agent Residue Evidence

ARE is distributed as native archives for macOS arm64, Linux amd64, and
Windows 11 amd64, plus a cross-platform MCPB. It runs fully locally and does
not download code at first launch.

The first release is `0.1.0`; asset names start with
`agent-residue-evidence_0.1.0_`, and the bundle is
`agent-residue-evidence_0.1.0.mcpb`.

## Verify before installing

Download the native archive, `SHA256SUMS`, and `sbom.spdx.json` from the same
GitHub Release. Verify the archive against `SHA256SUMS`, inspect the SPDX SBOM
when required by policy, and verify the packaged `provenance.json` binds the
version, commit, target, binary digest, and Plugin manifest digest. Do not
install an archive whose target, version, checksum, or provenance differs.

## Atomic installation

Extract into a new staging directory owned by the user. Verify
`provenance.json`, the binary SHA-256, the Plugin manifest SHA-256, executable
mode, `agent-residue-evidence --version`, `doctor`, MCP `initialize`, and the
six-tool `tools/list` response. Stop or drain an older ARE MCP process before
atomically replacing the current program, Plugin configuration, and
provenance together. Keep the previous verified directory as a rollback slot
until the new MCP reconnect succeeds.

Place the executable on the Agent's local PATH and install the bundled
`plugin/agent-residue-evidence` directory through that Agent's normal local
Plugin flow. Hosts without Agent Plugin support can register the executable as
a stdio MCP server with arguments `["mcp"]`; CI can use the same binary's JSON
CLI.

## Repair, upgrade, and rollback

A same-version repair repeats the complete staged verification and atomically
replaces damaged bytes. An upgrade never overlays individual files. If any
verification, commit, restart, or reconnect step fails, discard staging and
restore the previous verified directory. A deliberate downgrade follows the
same rollback procedure and must use a separately verified older release.

## Uninstall

Stop the ARE MCP process, remove its Agent Plugin registration, then remove
only the installed ARE program directory. Reports live in the documented ARE
state directory; keep or forget them according to user intent rather than
silently deleting them during uninstall. Never remove arbitrary task files,
directories, processes, or ports as part of uninstall.
