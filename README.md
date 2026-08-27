# Agent Residue Evidence

Agent Residue Evidence (ARE) adds an evidence checkpoint to an Agent's test and
build workflow:

```text
observe → Agent reviews evidence → user authorizes cleanup
        → Agent cleans with its own tools → ARE verifies
```

ARE shows task-scoped files, directories, attributed processes, and their
listening ports. It never deletes files, stops processes, closes ports, decides
that something is safe to delete, scans the full machine, uploads data, uses
the network, or collects telemetry.

## Standard task

Before the first test or build, the Agent calls `begin_task_observation` with
the exact workspace and any task-owned temporary roots. Events are optional;
safe generic events or empty heartbeats can be sent with
`append_task_events`. Before its final answer, the Agent calls
`end_task_observation`, explains the candidates, and asks the user what to do.
Only after explicit authorization does the Agent clean with its normal tools.
It then calls `verify_task_residue` to recheck the same stable candidates.

`NO_CANDIDATES_OBSERVED` means only that no candidate appeared inside the
declared observation scope. It is not a host-wide cleanliness claim.

## Completed task without a baseline

`inspect_completed_task` accepts explicit roots and a bounded time window.
The result is always `PARTIAL_EVIDENCE`; it can use Event, receipt, inferred,
or unattributed evidence, but never claims `BASELINE_OBSERVED`. Use
`get_residue_report` to read a saved report without observing again.

The MCP surface contains exactly six tools:

- `begin_task_observation`
- `append_task_events`
- `end_task_observation`
- `inspect_completed_task`
- `get_residue_report`
- `verify_task_residue`

There is deliberately no cleanup, delete, execute, terminate, or close tool.

## Local-first packaging

One release contains a native CLI/stdio MCP server, thin Agent Plugin, MCPB,
checksums, provenance, and SPDX SBOM. See [Quickstart](docs/quickstart.md) and
[Install](docs/install.md). Runtime operation is fully local and offline.
Reports default to seven-day retention and a 100 MB total cap; retained reports
and active baselines are protected. Uninstall never silently deletes reports.

Current local native acceptance is macOS 14+ arm64. Linux amd64 and Windows 11
amd64 are release targets with mandatory native CI jobs; they remain pending
until those jobs produce successful evidence. Cross-compilation alone is not
reported as native acceptance.

## Development

Go 1.26 or 1.27 is required.

```bash
bash scripts/check.sh
bash scripts/run_native_acceptance.sh
```

Apache-2.0 licensed. See [Security](SECURITY.md),
[Contributing](CONTRIBUTING.md), and [native acceptance](docs/native-acceptance.md).
