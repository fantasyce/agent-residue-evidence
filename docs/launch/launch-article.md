# Tests finish. Residue remains.

An Agent runs a test suite. The tests pass. The terminal returns. A temporary
database, helper process, or listening port may still be there.

Agent Residue Evidence (ARE) adds a local evidence checkpoint to that workflow.
The Agent declares the exact workspace and task-owned temporary roots before
testing. ARE records what appears in that scope, groups related candidates,
and presents path aliases instead of exposing exact paths by default.

Then ARE stops.

It has no delete, execute, terminate, or close tool. The Agent explains the
evidence and the user decides whether cleanup is appropriate. If cleanup is
authorized, the Agent uses its ordinary host tools and ARE verifies the same
stable candidates in a new immutable revision.

## Reproduce the checkpoint

```bash
bash scripts/demo_task_residue.sh
```

The demo creates a harmless task-owned file and reports it without exposing the
owner capability or local path. A separate `--cleanup-and-verify` mode proves
the complete lifecycle inside the disposable fixture.

## What ARE does not claim

`NO_CANDIDATES_OBSERVED` means no candidate appeared inside the declared
observation scope. It is not a whole-machine cleanliness result. Retrospective
inspection without a baseline is always partial evidence.

ARE is local and offline. It does not upload task data, collect telemetry,
scan the full host, or silently remove reports during uninstall.

Start with the [quickstart](https://github.com/fantasyce/agent-residue-evidence/blob/main/docs/quickstart.md),
run the [demo](https://github.com/fantasyce/agent-residue-evidence/blob/main/docs/demo.md),
or download the [latest release](https://github.com/fantasyce/agent-residue-evidence/releases/latest).

We welcome sanitized integration cases from Agent hosts, MCP clients, build
systems, and test frameworks—especially cases where ownership or cleanup
authorization is difficult to represent precisely.
