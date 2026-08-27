# Native acceptance

ARE supports macOS 14+ arm64, Linux amd64, and Windows 11 amd64. A successful
cross-compile is only a build gate; it is not native runtime evidence.

`scripts/run_native_acceptance.sh` must run on each target architecture. It
builds an installed-style binary with version and commit metadata, exercises
prospective and retrospective task journeys, verifies Agent-owned cleanup,
restarts stdio MCP, runs stable process and listening-port attribution, checks
scope and link defenses, tests interruption and retention, and removes its
task-owned roots. The output records only OS, architecture, Go version, commit,
binary digest, opaque report IDs, result, and residue status. It never uploads
task paths or report contents.

`scripts/run_privacy_acceptance.sh` proves the safe Event projection and scans
runtime source for raw command, environment, transcript, file-content, and
credential paths. `scripts/run_no_network_acceptance.sh` rejects networking
clients/listeners in product source and observes the installed stdio MCP for
network sockets. `scripts/verify_zero_residue.sh` rejects known task temporary
roots, generated repository directories, and surviving acceptance processes.

The GitHub Actions matrix uses the official hosted labels `macos-15` (arm64),
`ubuntu-24.04` (amd64), and `windows-2025` (amd64). Go 1.26.x and 1.27.x run
source gates separately. Failure artifacts contain only the sanitized summary;
raw reports and private task roots are never uploaded.

Local evidence on one host must be reported only for that host. Release support
requires successful native matrix jobs on macOS arm64, Linux amd64, and Windows
amd64; evidence from one host never stands in for another.
