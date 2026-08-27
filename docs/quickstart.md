# Quickstart

Install the verified native package and Agent Plugin as described in
[install.md](install.md). The Plugin guides compatible Agents automatically;
the CLI below exposes the same local contract.

```bash
agent-residue-evidence begin <<'JSON'
{"task_id":"example-test","workspace":"/absolute/path/to/repository","temp_roots":["/absolute/path/to/task-temp"]}
JSON
```

Keep the returned `owner_handle` in the active Agent task only. Public task,
observation, report, and candidate IDs cannot replace it.

Run the task's tests or build. Events are optional. End observation before the
Agent's final response:

```bash
agent-residue-evidence end <<'JSON'
{"owner_handle":"owner-handle-from-begin"}
JSON
```

Review the report with the user. ARE only recommends `review`; it never says a
candidate is safe to delete. If the user authorizes cleanup, the Agent uses its
own tools and then verifies:

```bash
agent-residue-evidence verify <<'JSON'
{"owner_handle":"owner-handle-from-begin"}
JSON
```

For a completed historical task, first use the local CLI `grant retrospective`
with explicit roots and a bounded time window, then pass its single-use,
ten-minute grant to `inspect-completed`. The resulting partial evidence cannot
establish what existed before the task.
