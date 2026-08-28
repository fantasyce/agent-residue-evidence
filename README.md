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

Visit the [project site](https://fantasyce.github.io/agent-residue-evidence/),
run the [reproducible task-residue demo](docs/demo.md), or download the
[latest verified release](https://github.com/fantasyce/agent-residue-evidence/releases/latest).

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

Every observation is isolated by an opaque owner capability. Task IDs, report
IDs, observation IDs, and candidate IDs are display references only and never
authorize access. State is encrypted at rest with opaque filenames; exact
paths are revealed only through an authorized candidate-resolution call.
Short-lived executor capabilities can append only explicitly allowed event
types and root aliases and cannot read, end, verify, or resolve a task.

The MCP surface contains exactly eight tools:

- `begin_task_observation`
- `append_task_events`
- `end_task_observation`
- `inspect_completed_task`
- `get_residue_report`
- `verify_task_residue`
- `delegate_task_executor`
- `resolve_residue_candidate`

There is deliberately no cleanup, delete, execute, terminate, or close tool.

## Local-first packaging

One release contains a native CLI/stdio MCP server, thin Agent Plugin, MCPB,
checksums, provenance, and SPDX SBOM. See [Quickstart](docs/quickstart.md) and
[Install](docs/install.md). Runtime operation is fully local and offline.
Reports default to seven-day retention and a 100 MB total cap; retained reports
and active baselines are protected. Uninstall never silently deletes reports.

The current release is published in the
[official MCP Registry](https://registry.modelcontextprotocol.io/v0.1/servers/io.github.fantasyce%2Fagent-residue-evidence/versions/0.3.0)
as `io.github.fantasyce/agent-residue-evidence`.

Native acceptance covers macOS 14+ arm64, Linux amd64, and Windows 11 amd64
through mandatory native CI jobs. Cross-compilation alone is not reported as
native acceptance.

## Agent Reliability Toolkit

ARE is one independent part of a small, local-first reliability toolkit:

- [Agent Runtime Proof](https://github.com/fantasyce/agent-runtime-proof) verifies that a live Agent or MCP runtime matches the artifact you approved.
- [Agent Residue Evidence](https://github.com/fantasyce/agent-residue-evidence) records task-scoped files, processes, and listening ports left by tests and builds.
- [DSH TypeLens](https://github.com/fantasyce/dsh-typelens) adds bounded type context and edit diagnostics to DeepSeek Harness.

Each project remains separately installable and keeps its own trust boundary.

## Development

Go 1.26 or 1.27 is required.

```bash
bash scripts/check.sh
bash scripts/run_native_acceptance.sh
```

Apache-2.0 licensed. See [Security](SECURITY.md),
[Contributing](CONTRIBUTING.md), and [native acceptance](docs/native-acceptance.md).
