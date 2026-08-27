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
- Keep the returned owner capability inside this task. Never expose it in
  Events, logs, reports, or messages to another task. Task, observation,
  report, and candidate IDs are references only and do not authorize access.
- During a long task, optionally call `append_task_events` with safe generic
  `agent-task-event/2.0` events or an empty event batch as a heartbeat. Use
  `owner_handle` for the owner and `executor_handle` for a delegated producer.
  Send only fingerprints and
  scoped paths. Never send command text, environment values, secrets, file
  contents, or conversation transcripts.
- Before the task's final answer, call `end_task_observation`. Explain each
  candidate's evidence level, current status, conflicts, and limitations. A
  candidate is not a safe-to-delete conclusion.
- Ask the user for explicit authorization before any cleanup. If authorized,
  use the Agent's normal file or process tools; ARE has no cleanup tool. Keep
  the cleanup targets exactly within what the user approved.
- After cleanup, call `verify_task_residue` with the owner capability. Report changed,
  absent, still-active, and unknown candidates without widening the scan.
- For a task that already ended without a baseline, use
  `inspect_completed_task` only after the local user-facing grant flow issued
  a single-use retrospective grant for explicit roots and a bounded time window.
  Treat its `PARTIAL_EVIDENCE` result as retrospective, not comprehensive.
- Use `get_residue_report` when only the stored report is needed. Use
  `delegate_task_executor` only for short-lived append-only event producers,
  and `resolve_residue_candidate` only when the user needs an exact authorized
  cleanup target. Do not use
  ARE for host-wide storage cleanup, automatic deletion, process termination,
  remote inspection, or credential discovery.
