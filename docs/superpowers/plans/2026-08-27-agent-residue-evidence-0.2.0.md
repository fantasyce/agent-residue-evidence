# Agent Residue Evidence 0.2.0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship ARE 0.2.0 with possession-based task isolation, encrypted local state, private path aliases, immutable verification revisions, bounded grouped evidence, and real two-task Codex acceptance.

**Architecture:** Random owner capabilities derive per-record authenticated-encryption keys while opaque record names prevent ID enumeration. The application service enforces owner and append-only executor authority; aliases, grouping, pagination, and immutable revision chains sit above the existing deterministic observation engine. MCP and CLI remain thin evidence-only adapters.

**Tech Stack:** Go 1.26 standard-library cryptography and JSON, official MCP Go SDK, existing native process adapters, shell/Python release validators, GitHub Actions native macOS/Linux/Windows runners.

**Spec:** `docs/superpowers/specs/2026-08-27-agent-residue-evidence-0.2.0-design.md`

## Global Constraints

- No global recovery key and no automatic 0.1.0 migration.
- Runtime remains local and deterministic with no model or network call.
- User resources remain read-only; ARE has no cleanup, execution, termination, or port-closing operation.
- Task/report identifiers are display or lookup metadata, never authorization.
- Active observations expire at 24 hours and completed chains at seven days.
- Agent-visible paths are aliases; exact paths require owner authority and an existing candidate ID.
- Reports and verification revisions are immutable.
- Generic MCP honestly exposes the recoverable-handle boundary; ephemeral mode loses access at restart.
- Tests and release work use task-owned roots and finish with zero task-created residue.

---

### Task 1: Capability and encrypted store foundation

**Files:**
- Create: `internal/capability/capability.go`
- Create: `internal/capability/capability_test.go`
- Create: `internal/store/crypto.go`
- Create: `internal/store/crypto_test.go`
- Modify: `internal/store/types.go`
- Modify: `internal/store/store.go`
- Modify: `internal/store/retention.go`
- Modify: `internal/store/store_test.go`
- Modify: `internal/store/retention_test.go`

**Interfaces:**
- Produces `capability.OwnerHandle`, `capability.ExecutorHandle`, `capability.Parse`, and opaque authenticated envelopes.
- Store operations consume a handle and return generic access denial for wrong, malformed, expired, or unrelated handles.

- [ ] Write failing tests proving two owners cannot read or mutate each other's records, filenames are opaque, plaintext scans reveal no task/path values, ciphertext tampering fails, and lost handles have no recovery path.
- [ ] Run focused tests and verify they fail for missing capability/encryption behavior.
- [ ] Implement random handles, contextual key derivation, AES-256-GCM envelopes, opaque names, and atomic private writes.
- [ ] Run focused store/capability tests and the existing store suite to green.
- [ ] Refactor only after green and commit the independently working encrypted store.

### Task 2: Contract v2, modes, aliases, and grouping

**Files:**
- Create: `internal/pathalias/alias.go`
- Create: `internal/pathalias/alias_test.go`
- Create: `internal/group/group.go`
- Create: `internal/group/group_test.go`
- Modify: `internal/contract/types.go`
- Modify: `internal/contract/validate.go`
- Modify: `internal/contract/schemas/*.json`
- Modify: `internal/correlate/correlate.go`
- Modify: `internal/correlate/correlate_test.go`

**Interfaces:**
- Produces v2 observation modes, aliased candidates, deterministic group summaries, page metadata, and root bindings.
- Consumes existing filesystem/process candidates without changing observation semantics.

- [ ] Write failing tests for mode validation, absolute-path suppression, root-bound alias round trips, parent-directory aggregation, conflicting-child preservation, deterministic cursors, and non-silent truncation.
- [ ] Run focused tests and confirm failures identify missing v2 behavior.
- [ ] Implement aliases, root binding, grouping, summary limits, and schema v2 validation.
- [ ] Run contract, alias, grouping, correlator, filesystem, and process tests to green.
- [ ] Commit the independently testable evidence-projection layer.

### Task 3: Owner lifecycle, executor delegation, and immutable revisions

**Files:**
- Modify: `internal/app/service.go`
- Modify: `internal/app/service_test.go`
- Modify: `internal/store/store.go`
- Modify: `internal/store/types.go`
- Create: `internal/store/revision_test.go`

**Interfaces:**
- `Begin` returns an observation ID plus recoverable owner handle or an ephemeral session binding.
- Owner operations accept owner authority; executor operations accept append-only delegated authority.
- `Verify` appends a revision and never modifies revision zero.

- [ ] Write failing tests for owner-only end/get/verify/retain/forget/resolve, executor append-only access and expiry, revocation at end, immutable verification history, and generic denial errors.
- [ ] Run focused tests and verify red behavior.
- [ ] Implement lifecycle authorization, executor delegation, candidate-only exact resolution, and revision append semantics.
- [ ] Run application/store tests to green and inspect race behavior with `go test -race` on affected packages.
- [ ] Commit the complete authorized application lifecycle.

### Task 4: Explicit retrospective grants and bounded interfaces

**Files:**
- Modify: `internal/app/service.go`
- Modify: `internal/app/service_test.go`
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/server_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`

**Interfaces:**
- Retrospective inspection consumes a short-lived exact-scope grant and always emits `RETROSPECTIVE` partial evidence.
- MCP/CLI accept handles through structured input and never echo them in errors or human output.

- [ ] Write failing tests proving report IDs alone fail, wrong handles are non-enumerating, retrospective calls without a grant fail, granted calls cannot widen roots/windows, and tool output is bounded.
- [ ] Run MCP and CLI tests to confirm red behavior.
- [ ] Implement v2 adapters, private grant issuance/validation, paged report reads, candidate resolution, retain, and forget without any cleanup tool.
- [ ] Run MCP/CLI/application tests to green and verify the advertised tool surface remains evidence-only.
- [ ] Commit the public v2 interface.

### Task 5: Privacy, offline, recovery, and native acceptance

**Files:**
- Modify: `scripts/run_privacy_acceptance.sh`
- Modify: `scripts/run_no_network_acceptance.sh`
- Modify: `scripts/run_native_acceptance.sh`
- Create: `scripts/run_task_isolation_acceptance.sh`
- Modify: `scripts/check.sh`
- Modify: `.github/workflows/quality.yml`

**Interfaces:**
- Produces deterministic acceptance evidence for plaintext absence, recoverable restart, ephemeral loss, two-owner denial, and native platform behavior.

- [ ] Write acceptance checks that fail against 0.1.0 for plaintext state and ID-only cross-task access.
- [ ] Run them and preserve the expected red result in test output only.
- [ ] Wire v2 inputs and final assertions for no network, privacy, isolation, recovery, expiry, and zero task-created residue.
- [ ] Run the complete local source gate and macOS native acceptance to green.
- [ ] Commit acceptance automation and CI matrix changes.

### Task 6: Plugin, documentation, version, and packaging

**Files:**
- Modify: `VERSION`
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Modify: `SECURITY.md`
- Modify: `docs/event-contract.md`
- Modify: `docs/report-contract.md`
- Modify: `docs/threat-model.md`
- Modify: `docs/quickstart.md`
- Modify: `docs/install.md`
- Modify: `plugin/agent-residue-evidence/skills/agent-residue-evidence/SKILL.md`
- Modify: `plugin/agent-residue-evidence/skills/agent-residue-evidence/agents/openai.yaml`
- Modify: `plugin/agent-residue-evidence/.codex-plugin/plugin.json`
- Modify: `plugin/agent-residue-evidence/.mcp.json`
- Modify: `packaging/mcpb/manifest.json.in`
- Modify: `packaging/mcp-registry/server.json.in`
- Modify: `scripts/build_release_assets.sh`
- Modify: `scripts/verify_release_assets.sh`
- Modify: `scripts/verify_release_metadata.sh`
- Modify: `scripts/test_packaging.sh`
- Modify: `scripts/test_plugin_surface.sh`

**Interfaces:**
- Produces consistent 0.2.0 source, plugin, MCPB, native archives, checksums, provenance, SBOM, and user instructions.

- [ ] Write or update version-consistency and plugin-surface tests first and verify they fail while metadata remains 0.1.0.
- [ ] Update all contracts and product copy, explicitly documenting recovery profiles and threat boundaries.
- [ ] Build release assets and run exact-byte, secret, absolute-path, and test-content scans.
- [ ] Run the full source, packaging, open-source, and local installed-lifecycle gates.
- [ ] Commit the release candidate metadata and artifacts policy.

### Task 7: Formal local Codex acceptance, release, and cleanup

**Files:**
- No source changes after final candidate bytes are built; any fix returns to the appropriate TDD task and invalidates later evidence.

**Interfaces:**
- Consumes the packaged candidate plugin and produces installed-runtime, two-task Codex, release, and hygiene evidence.

- [ ] Remove only verified task-owned 0.1.0 test state, install the candidate plugin into local Codex, restart/reconnect, and verify reported version and tool schemas.
- [ ] Run the two-task attack matrix: ID-only access, wrong owner, executor escalation, exact-path disclosure, retain/forget, end, and verify all fail across tasks.
- [ ] Complete the owner journey, restart recovery, ephemeral-loss, immutable verification, no-network, and plaintext-state scans on the installed candidate.
- [ ] Run final full gates on the exact commit, push through a short-lived branch, merge to `main`, tag and publish `v0.2.0`, and verify remote release assets/checksums/CI.
- [ ] Replace the local candidate with the official 0.2.0 plugin, rerun installed smoke and cross-task denial, then remove task-owned worktrees, build caches, logs, temporary apps, archives, and obsolete local/remote branches.
- [ ] Verify source checkout, installed plugin, ARE state, local branches/worktrees, remote branches, release, and disk usage before reporting completion.
