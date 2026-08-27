# Showcase submission

**Name:** Agent Residue Evidence (ARE)

**One line:** Local task-scoped evidence for files, processes, and listening
ports left by Agent tests and builds—without a cleanup tool.

**Problem:** A test command finishing does not prove that everything it created
has stopped or disappeared.

**Approach:** ARE observes explicit task roots, returns grouped and path-aliased
evidence, stops for human review, and verifies the same stable candidates after
user-authorized Agent cleanup.

**Safety boundary:** Local and offline; no full-host scan, telemetry, delete,
execute, terminate, close-port, or automatic safety decision.

**Demo:** https://github.com/fantasyce/agent-residue-evidence/blob/main/docs/demo.md

**Source:** https://github.com/fantasyce/agent-residue-evidence

**License:** Apache-2.0
