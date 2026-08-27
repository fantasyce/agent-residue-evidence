---
name: agent-residue-evidence
description: Use local task-scoped evidence when tests, builds, packaging, or validation may leave files, directories, processes, or attributed listening ports that the Agent should review with the user.
---

# Agent Residue Evidence

Use ARE as an evidence step in a testing or build workflow. ARE observes and
reports; it never cleans resources.

- Before the first test, build, package, or validation command, call
  `begin_task_observation` with one task ID, the exact workspace, and only
  task-owned temporary roots. Never use a filesystem root, home directory,
  project collection, or full-disk scope.
- During a long task, optionally call `append_task_events` with safe generic
  events or an empty event batch as a heartbeat. Send only fingerprints and
  scoped paths. Never send command text, environment values, secrets, file
  contents, or conversation transcripts.
- Before the task's final answer, call `end_task_observation`. Explain each
  candidate's evidence level, current status, conflicts, and limitations. A
  candidate is not a safe-to-delete conclusion.
- Ask the user for explicit authorization before any cleanup. If authorized,
  use the Agent's normal file or process tools; ARE has no cleanup tool. Keep
  the cleanup targets exactly within what the user approved.
- After cleanup, call `verify_task_residue` with the report ID. Report changed,
  absent, still-active, and unknown candidates without widening the scan.
- For a task that already ended without a baseline, use
  `inspect_completed_task` only with explicit roots and a bounded time window.
  Treat its `PARTIAL_EVIDENCE` result as retrospective, not comprehensive.
- Use `get_residue_report` when only the stored report is needed. Do not use
  ARE for host-wide storage cleanup, automatic deletion, process termination,
  remote inspection, or credential discovery.
