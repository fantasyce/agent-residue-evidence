# Security policy

Report suspected vulnerabilities privately through GitHub Security Advisories
for `fantasyce/agent-residue-evidence`. Do not include real credentials,
private file contents, full command lines, environment values, transcripts, or
private task paths in a public issue.

ARE's security boundary is intentionally narrow: no administrator privileges,
no runtime network access or telemetry, reads only explicit task roots and
task-related process metadata, writes only its own state, and has no cleanup or
execution tool. Reports can contain exact local task paths and should be
treated as private local evidence.

Supported releases receive fixes on the latest published minor line. A report
should include the ARE version, platform, architecture, sanitized reproduction
steps, and whether the issue affects scope enforcement, identity binding,
local persistence, packaging provenance, or MCP input validation.
