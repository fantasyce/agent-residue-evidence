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
- AES-256-GCM encrypted state with opaque filenames and metadata-bound authentication;
- capability-only task/report access: public identifiers never authorize an operation;
- append-only, short-lived executor capabilities that contain no owner record key;
- immutable, digest-chained verification revisions rather than report mutation;
- current observation wins when Event history conflicts.

ARE does not inspect containers or virtual machines, system users/services,
browser profiles, Keychain/credential stores, shared caches, or remote hosts.
It cannot prove that all task effects are absent. A malicious privileged local
process, compromised kernel, or attacker able to rewrite the ARE binary/state
is outside the runtime trust boundary. Anyone who receives an owner capability
can access that one task, so Agents must treat it as a local secret and never
put it in logs, transcripts, Events, or published reports. Release checksums, packaged provenance,
and the SBOM address distribution integrity, not a compromised host.
