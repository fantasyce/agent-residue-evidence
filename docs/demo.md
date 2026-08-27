# Reproduce task residue safely

This demonstration creates all files and evidence state under one task-owned
temporary directory. It begins an ARE observation, creates a harmless test
artifact, ends observation, and prints a sanitized summary.

From a source release or clean checkout:

```bash
bash scripts/demo_task_residue.sh
```

Expected summary:

```text
Observation: BASELINE_OBSERVED
Candidates: at least 1
Kinds: file
Cleanup authorization required: true
ARE cleanup tool present: false
```

The demonstration stops at the same product boundary as ARE: the evidence is
available, but ARE never deletes the candidate or decides it is safe. The demo
harness removes its own temporary fixture when it exits.

Maintainers can exercise the complete, explicitly task-owned fixture lifecycle:

```bash
bash scripts/demo_task_residue.sh --cleanup-and-verify
```

That mode removes only the exact file created by the demo, using ordinary host
tools, then asks ARE to verify the stable candidate is absent. It also forgets
the isolated report. This is test-harness housekeeping, not an ARE cleanup
capability.

The script never uses the formal ARE state directory, edits Agent
configuration, requests privileges, scans outside the declared scope, opens a
network connection, or prints the owner capability or exact local path.
