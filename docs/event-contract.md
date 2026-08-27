# Agent Task Event contract

The optional `agent-task-event/1.0` contract strengthens attribution without
making ARE depend on AAA, Codex, Claude, or any private transcript format.

Supported types are `command_started`, `command_completed`, `process_started`,
`process_exited`, `artifact_declared`, `test_phase_started`,
`test_phase_completed`, and `cleanup_attempted`.

Every event has `schema_version`, `task_id`, `event_id`, `type`, and a UTC
`timestamp`. Optional safe fields are a scoped working directory, a
`sha256:` command fingerprint, exit code, stable PID plus process creation
time, scoped declared outputs, and an opaque receipt ID. Unknown fields are
rejected. Raw commands, environment values, secrets, file contents, and full
conversations are outside the contract and must not be submitted.

Events can improve evidence to `EVENT_BOUND` or `RECEIPT_BOUND`; they cannot
override contradictory current observation. An empty batch is a valid
heartbeat and refreshes the active task's interruption deadline.
