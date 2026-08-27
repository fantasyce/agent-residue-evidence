# Launch FAQ

## Why is a passing test suite not enough?

Pass or fail describes assertions. It does not prove that task-owned files,
processes, sockets, or ports were removed after the command returned.

## Does ARE clean residue?

No. ARE provides evidence and candidate verification. It has no delete,
execute, terminate, or close tool. The Agent uses normal host tools only after
the user authorizes cleanup.

## Does ARE scan the whole machine?

No. Prospective observation is limited to the exact workspace and task-owned
temporary roots declared at the start.

## Is NO_CANDIDATES_OBSERVED a clean-host verdict?

No. It is a scoped observation result. Resources outside the declared roots,
short-lived resources between samples, and evidence unavailable to the local
account remain outside that claim.

## Does ARE upload paths or process data?

No. Runtime operation is local and offline with no telemetry. Reports use path
aliases; exact paths require the task's opaque owner capability.

## Can another task read a report ID?

No. Task, observation, report, and candidate IDs are display references only.
Possession of the opaque owner capability is required.

## Which platforms are supported?

Published native assets target macOS 14+ arm64, Linux amd64, and Windows 11
amd64. All three run mandatory native acceptance.

## Is ARE tied to Across Agents Assistant?

No. ARE is an independent Apache-2.0 CLI, stdio MCP server, Agent Plugin, and
cross-platform MCPB.
