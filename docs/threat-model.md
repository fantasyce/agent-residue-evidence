# Threat model

ARE addresses accidental residue from one Agent task: new or changed files and
directories under explicit roots, stable task-related processes, and local
listening ports owned by those processes. It also protects the evidence chain
against PID reuse, root replacement, symlink/reparse escape, malformed Events,
partial writes, stale tasks, and report tampering.

Security invariants:

- no full-disk, home-directory, drive-root, or project-collection scan;
- no administrator privileges, cleanup commands, process control, or port closure;
- no runtime network clients, listeners, telemetry, or first-run downloads;
- no storage of file contents, raw commands, environments, secrets, or transcripts;
- atomic private state writes, stable file/process identities, and bounded retention;
- current observation wins when Event history conflicts.

ARE does not inspect containers or virtual machines, system users/services,
browser profiles, Keychain/credential stores, shared caches, or remote hosts.
It cannot prove that all task effects are absent. A malicious privileged local
process, compromised kernel, or attacker able to rewrite the ARE binary/state
is outside the runtime trust boundary; release checksums, provenance, SBOM, and
attestation address distribution integrity, not a compromised host.
