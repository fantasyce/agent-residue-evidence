# Agent Residue Evidence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and natively validate Agent Residue Evidence (ARE), a local task-scoped CLI, stdio MCP server, and thin Agent Plugin that reports files, directories, processes, and process-bound ports left by an Agent test task without modifying user resources.

**Architecture:** A single Go application core owns scope validation, file/process observation, optional event correlation, lifecycle storage, and compact reports. CLI and stdio MCP are thin adapters over that core; OS-specific packages supply file identity and process/port evidence for macOS arm64, Linux amd64, and Windows amd64. A thin plugin and deterministic MCPB package the same binary and lifecycle instructions without adding host-specific business logic.

**Tech Stack:** Go 1.26 language baseline with Go 1.26.x and 1.27.x CI, official stable `github.com/modelcontextprotocol/go-sdk`, standard library JSON and filesystem APIs, minimal OS-specific syscalls, shell/Python packaging validators, GitHub Actions native macOS/Linux/Windows runners.

**Spec:** `docs/superpowers/specs/2026-08-27-agent-residue-evidence-design.md`

## Global Constraints

- Observe only explicit task roots; reject `/`, home directories, drive roots, workspace-collection roots, symlink escapes, junction escapes, and reparse-point escapes.
- User resources are read-only. ARE never deletes or moves files, terminates processes, closes ports, runs cleanup commands, or emits a safe-to-delete verdict.
- Runtime is fully local: no network client, listener, telemetry, account, cloud sync, or remote dependency. Installation-time artifact retrieval is outside runtime.
- Store no file contents, raw environment values, tokens, cookies, full transcripts, raw events, or full command lines.
- Baselines are primary evidence; generic task events are optional attribution evidence.
- Final reports default to 7-day retention and a 100 MB total limit; active task baselines are never evicted for capacity.
- A task becomes interrupted after 24 hours without a successful event append or explicit empty-event heartbeat.
- First release evidence types are files, directories, task-related processes, and ports bound by attributed processes.
- Native acceptance is mandatory on macOS 14+ arm64, Linux amd64, and Windows 11 amd64. Cross-build success is not native acceptance.
- The user-facing install is one atomic Agent Plugin + MCP Server operation; standalone CLI/MCP and MCPB remain supported distribution paths.
- Use Apache-2.0 for the independent ARE core and distribution assets so Agent hosts can integrate it without inheriting AAA's license boundary.
- Tests, builds, acceptance helpers, databases, reports, packages, and worktrees use task-owned paths and leave no residue after completion.

## Planned File Structure

```text
cmd/agent-residue-evidence/main.go                 executable entrypoint only
internal/contract/types.go                        versioned task, event, candidate, report types
internal/contract/validate.go                     contract validation and safe projection rules
internal/contract/schema.go                       embedded JSON schema access
internal/contract/schemas/*.json                  public event/report/lifecycle schemas
internal/scope/guard.go                            narrow-root validation and canonical boundaries
internal/scope/guard_windows.go                    Windows drive/root/reparse-aware checks
internal/fsobserve/observer.go                     baseline/end snapshot orchestration
internal/fsobserve/diff.go                         metadata-only candidate diff
internal/fsobserve/identity_unix.go                macOS/Linux stable file identity
internal/fsobserve/identity_windows.go             Windows volume/file identity
internal/event/normalize.go                        generic event validation and safe summaries
internal/correlate/correlate.go                    evidence-grade assignment and conflicts
internal/process/observer.go                       platform-neutral process evidence interface
internal/process/observer_darwin.go                macOS process/port observation
internal/process/observer_linux.go                 Linux process/port observation
internal/process/observer_windows.go               Windows process/port observation
internal/store/store.go                            atomic lifecycle/report persistence
internal/store/retention.go                        7-day/100 MB compaction and interruption sweep
internal/app/service.go                            begin/append/end/inspect/get/verify application API
internal/cli/run.go                                CLI parser and JSON projection
internal/mcpserver/server.go                       six MCP tools over application API
internal/versioninfo/version.go                    build version and commit
plugin/agent-residue-evidence/.codex-plugin/plugin.json
plugin/agent-residue-evidence/.mcp.json
plugin/agent-residue-evidence/skills/agent-residue-evidence/SKILL.md
packaging/mcpb/manifest.json.in                    platform command overrides
scripts/check.sh                                   complete local source gate
scripts/build_release_assets.sh                    deterministic native assets and MCPB
scripts/verify_release_assets.sh                   archive/MCPB exact-byte and secret checks
scripts/run_native_acceptance.sh                   installed task journey and zero-residue gate
.github/workflows/quality.yml                      Go/security/native acceptance matrix
```

---

### Task 1: Repository foundation, contracts, and scope guard

**Files:**
- Create: `go.mod`
- Create: `go.sum`
- Create: `VERSION`
- Create: `cmd/agent-residue-evidence/main.go`
- Create: `internal/versioninfo/version.go`
- Create: `internal/contract/types.go`
- Create: `internal/contract/validate.go`
- Create: `internal/contract/schema.go`
- Create: `internal/contract/schemas/agent-task-event.schema.json`
- Create: `internal/contract/schemas/residue-report.schema.json`
- Create: `internal/contract/schemas/task-lifecycle.schema.json`
- Create: `internal/contract/contract_test.go`
- Create: `internal/scope/guard.go`
- Create: `internal/scope/guard_windows.go`
- Create: `internal/scope/guard_test.go`
- Create: `scripts/check.sh`
- Create: `.gitignore`

**Interfaces:**
- Consumes: Approved design constants and state names.
- Produces: `contract.TaskScope`, `contract.TaskEvent`, `contract.Candidate`, `contract.Report`, `contract.TaskState`, `scope.Guard.Validate(TaskScope) (scope.Validated, error)`, and embedded schemas used by every later task.

- [ ] **Step 1: Write failing contract and scope tests**

Create tests that construct `TaskScope{TaskID, Workspace, TempRoots}`, validate every enum, reject unknown JSON fields, reject secrets/raw commands in events, and reject `/`, the current home, drive roots, parent traversal, duplicate/nested broad roots, and symlink escapes. Include a positive fixture with one repository root and one task-owned temp root.

```go
func TestGuardRejectsHomeDirectory(t *testing.T) {
    home, err := os.UserHomeDir()
    if err != nil { t.Fatal(err) }
    _, err = NewGuard().Validate(contract.TaskScope{TaskID: "task-1", Workspace: home})
    if !errors.Is(err, ErrScopeTooBroad) { t.Fatalf("got %v", err) }
}

func TestEventRejectsRawCommand(t *testing.T) {
    raw := []byte(`{"schema_version":"agent-task-event/1.0","task_id":"task-1","event_id":"e1","type":"command_started","command":"printenv"}`)
    if _, err := contract.DecodeTaskEvent(raw); err == nil { t.Fatal("raw command accepted") }
}
```

- [ ] **Step 2: Run the focused tests and confirm red**

Run: `go test ./internal/contract ./internal/scope -count=1`

Expected: FAIL because the packages and exported contracts do not exist.

- [ ] **Step 3: Implement versioned contracts and embedded schemas**

Define exact constants:

```go
const (
    EventSchemaVersion  = "agent-task-event/1.0"
    ReportSchemaVersion = "agent-residue-report/1.0"
    LifecycleVersion    = "agent-residue-lifecycle/1.0"
)
```

Use `json.Decoder.DisallowUnknownFields`, explicit enum validation, UTC RFC3339 timestamps, bounded string/list sizes, and prohibited-field checks. `Candidate` must keep `EvidenceLevel` separate from `CurrentStatus`. Embed schemas using `//go:embed schemas/*.json`.

- [ ] **Step 4: Implement canonical task scope validation**

Resolve absolute roots, reject broad targets before walking, verify roots exist, record root file identity, and reject symlink/reparse traversal. `scope.Validated` must expose only canonical roots and stable root identities, never the unvalidated input.

- [ ] **Step 5: Add the minimal executable and complete source gate**

Initialize module `github.com/fantasyce/agent-residue-evidence`, set `go 1.26`, add `--version`, and make `scripts/check.sh` run `gofmt -l`, `go vet ./...`, `go mod verify`, `go test -count=1 -race ./...`, and `git diff --check`.

- [ ] **Step 6: Run and confirm green**

Run: `bash scripts/check.sh`

Expected: all contract/scope tests pass, no formatting or diff errors.

- [ ] **Step 7: Commit**

```bash
git add .gitignore go.mod go.sum VERSION cmd internal/contract internal/scope internal/versioninfo scripts/check.sh
git commit -m "feat: define ARE contracts and scope guard"
```

---

### Task 2: Metadata-only filesystem baseline and candidate diff

**Files:**
- Create: `internal/fsobserve/types.go`
- Create: `internal/fsobserve/observer.go`
- Create: `internal/fsobserve/diff.go`
- Create: `internal/fsobserve/identity_unix.go`
- Create: `internal/fsobserve/identity_windows.go`
- Create: `internal/fsobserve/observer_test.go`
- Create: `internal/fsobserve/diff_test.go`
- Modify: `internal/contract/types.go`

**Interfaces:**
- Consumes: `scope.Validated` and `contract.Candidate`.
- Produces: `fsobserve.Observer.Capture(ctx, scope) (Baseline, error)` and `fsobserve.Observer.Compare(ctx, Baseline) (Diff, error)`, with file/directory candidates and explicit limitations.

- [ ] **Step 1: Write adversarial failing filesystem tests**

Cover new file, new directory, changed metadata, removed object, pre-existing ignored build output, symlink escape, root replacement, scan-time mutation, permission failure, special file, and bounded entry/byte/time limits. Assert that file contents never appear in snapshots or reports.

```go
func TestCompareReportsNewDirectoryWithoutReadingContents(t *testing.T) {
    root := t.TempDir()
    observer := NewObserver(Limits{MaxEntries: 1000, MaxDuration: time.Second})
    base := mustCapture(t, observer, root)
    secretDir := filepath.Join(root, "test-output")
    if err := os.Mkdir(secretDir, 0o700); err != nil { t.Fatal(err) }
    if err := os.WriteFile(filepath.Join(secretDir, "secret.txt"), []byte("never-copy-me"), 0o600); err != nil { t.Fatal(err) }
    diff := mustCompare(t, observer, base)
    if strings.Contains(fmt.Sprint(diff), "never-copy-me") { t.Fatal("content leaked") }
}
```

- [ ] **Step 2: Run and confirm red**

Run: `go test ./internal/fsobserve -count=1`

Expected: FAIL because observer types and platform identity adapters do not exist.

- [ ] **Step 3: Implement stable file identity and no-follow walking**

On macOS/Linux bind identity to device/inode/type from no-follow metadata. On Windows bind volume serial and file ID from an opened handle and reject reparse traversal. Store relative paths, type, identity, size, mtime, mode, and link state only.

- [ ] **Step 4: Implement baseline and comparison**

Walk only validated roots. Compare identity and metadata to produce `PRESENT`, `CHANGED_SINCE_REPORT`, `NO_LONGER_PRESENT`, or `UNKNOWN`; attach limitations for permission/race/limit failures. Do not pre-hash the whole workspace. A disappearing object during comparison is a state change, not a fatal scan failure.

- [ ] **Step 5: Run focused and complete checks**

Run: `go test ./internal/fsobserve -count=1 -race && bash scripts/check.sh`

Expected: all filesystem adversarial tests and the full gate pass.

- [ ] **Step 6: Commit**

```bash
git add internal/contract/types.go internal/fsobserve
git commit -m "feat: observe task-scoped filesystem residue"
```

---

### Task 3: Generic event normalization and evidence correlation

**Files:**
- Create: `internal/event/normalize.go`
- Create: `internal/event/normalize_test.go`
- Create: `internal/correlate/correlate.go`
- Create: `internal/correlate/correlate_test.go`
- Modify: `internal/contract/types.go`
- Modify: `internal/contract/schemas/agent-task-event.schema.json`
- Modify: `internal/contract/schemas/residue-report.schema.json`

**Interfaces:**
- Consumes: versioned `contract.TaskEvent`, filesystem `Diff`, optional receipt references, and current object state.
- Produces: `event.Normalize(batch, validatedScope) ([]event.Summary, error)` and `correlate.BuildReport(Input) (contract.Report, error)`.

- [ ] **Step 1: Write failing event privacy and correlation tests**

Test all eight event types, empty heartbeat batches, duplicate event IDs, out-of-order timestamps, wrong task IDs, paths outside scope, oversized batches, prohibited raw command/environment fields, and current-state conflicts. Assert baseline evidence outranks inference while Event never overrides a contradictory current observation.

```go
func TestCurrentPresenceWinsOverCleanupEvent(t *testing.T) {
    got := BuildReport(Input{
        Diff: filePresent("tmp/output"),
        Events: []event.Summary{{Type: contract.EventCleanupAttempted, Path: "tmp/output"}},
    })
    if got.Candidates[0].CurrentStatus != contract.StatusPresent { t.Fatalf("got %s", got.Candidates[0].CurrentStatus) }
    if len(got.Candidates[0].Conflicts) == 0 { t.Fatal("missing event conflict") }
}
```

- [ ] **Step 2: Run and confirm red**

Run: `go test ./internal/event ./internal/correlate -count=1`

Expected: FAIL because normalization and correlation packages do not exist.

- [ ] **Step 3: Implement safe event summaries**

Accept only `command_started`, `command_completed`, `process_started`, `process_exited`, `artifact_declared`, `test_phase_started`, `test_phase_completed`, and `cleanup_attempted`. Preserve event/task IDs, UTC time, scoped working directory, command fingerprint, exit code, stable PID identity, declared scoped outputs, and receipt ID. Treat an empty valid batch as heartbeat-only.

- [ ] **Step 4: Implement deterministic correlation**

Assign exactly one evidence level per candidate using ordered rules: `BASELINE_OBSERVED`, `RECEIPT_BOUND`, `EVENT_BOUND`, `INFERRED`, `UNATTRIBUTED`. Keep current status independent, preserve conflicts/limitations, sort candidates deterministically by kind and stable ID, and never emit deletion instructions.

- [ ] **Step 5: Run checks and schema round trips**

Run: `go test ./internal/event ./internal/correlate ./internal/contract -count=1 -race && bash scripts/check.sh`

Expected: event/privacy/conflict tests pass and reports validate against embedded schemas.

- [ ] **Step 6: Commit**

```bash
git add internal/contract internal/event internal/correlate
git commit -m "feat: correlate generic task events with residue evidence"
```

---

### Task 4: Native process identity and attributed ports

**Files:**
- Create: `internal/process/types.go`
- Create: `internal/process/observer.go`
- Create: `internal/process/observer_darwin.go`
- Create: `internal/process/observer_linux.go`
- Create: `internal/process/observer_windows.go`
- Create: `internal/process/observer_test.go`
- Create: `internal/process/native_acceptance_test.go`
- Modify: `internal/correlate/correlate.go`

**Interfaces:**
- Consumes: task roots, process IDs/creation times from events or receipts, and current-user process metadata.
- Produces: `process.Observer.Baseline(ctx)`, `process.Observer.Resolve(ctx, Hints) ([]Evidence, []Limitation)`, and attributed local listening ports for stable task processes only.

- [ ] **Step 1: Write failing process attribution tests**

Use a test helper that starts a child with its working directory inside the task root and a loopback listener on an OS-assigned port. Cover PID reuse defense, exited child, unrelated user process, parent-child chain, workdir attribution, receipt attribution, permission denial, and port ownership. Assert unrelated processes and command lines are absent from output.

- [ ] **Step 2: Run and confirm red**

Run: `go test ./internal/process -count=1`

Expected: FAIL because process observers do not exist.

- [ ] **Step 3: Implement platform-neutral attribution rules**

Define stable `Identity{PID, CreatedAt}` and require both fields for deterministic attribution. Retain only event/receipt PID matches, stable descendants, task-root working directories, or processes holding a candidate path. Return limitations instead of widening scope.

- [ ] **Step 4: Implement native adapters**

Use native process creation-time and parent APIs on macOS/Linux/Windows. Query local listening sockets only for already attributed PIDs. Do not scan remote addresses, open connections, other-user command lines, or environments. Windows must use creation time plus PID and native TCP table ownership; Unix must avoid shelling out in the product runtime.

- [ ] **Step 5: Run native focused checks**

Run on the current native host: `go test ./internal/process -count=1 -race -run 'TestNative|TestAttribution|TestPort'`

Expected: helper is attributed, listener port is bound to its stable PID, unrelated process is excluded, helper exits, and the task-owned directory is removed by the test harness.

- [ ] **Step 6: Run complete gate and commit**

Run: `bash scripts/check.sh`

```bash
git add internal/process internal/correlate/correlate.go
git commit -m "feat: observe attributed task processes and ports"
```

---

### Task 5: Atomic lifecycle store, interruption recovery, and bounded retention

**Files:**
- Create: `internal/store/types.go`
- Create: `internal/store/store.go`
- Create: `internal/store/retention.go`
- Create: `internal/store/store_test.go`
- Create: `internal/store/retention_test.go`
- Create: `internal/store/recovery_test.go`
- Modify: `internal/contract/types.go`

**Interfaces:**
- Consumes: task lifecycle, baseline, normalized event summaries, and final reports.
- Produces: `store.Open(home)`, atomic `CreateTask`, `AppendEvents`, `CompleteTask`, `GetReport`, `VerifyReport`, `RetainReport`, `ForgetReport`, and `Sweep(now)` operations.

- [ ] **Step 1: Write failing atomicity, recovery, and retention tests**

Test crash before rename, corrupt index, concurrent event append, MCP restart, 24-hour heartbeat boundary, interrupted compaction, 7-day expiry, 100 MB eviction ordering, retained reports, active baseline protection, and exact report forgetting. Use a fake clock and task-owned `ARE_HOME`.

```go
func TestSweepNeverEvictsActiveBaseline(t *testing.T) {
    s := newTestStore(t, WithCapacity(100))
    active := seedActiveTask(t, s, 200)
    seedCompletedReport(t, s, "old", 80, time.Now().Add(-8*24*time.Hour))
    if err := s.Sweep(context.Background(), time.Now()); err != nil { t.Fatal(err) }
    if _, err := s.LoadTask(active); err != nil { t.Fatalf("active baseline lost: %v", err) }
}
```

- [ ] **Step 2: Run and confirm red**

Run: `go test ./internal/store -count=1`

Expected: FAIL because the store does not exist.

- [ ] **Step 3: Implement atomic local persistence**

Write task/report JSON through a same-directory staging file, `fsync`, rename, and parent-directory sync where supported. Use private user permissions at creation time. Never accept an arbitrary path for retain/forget; resolve only validated report IDs under the store root.

- [ ] **Step 4: Implement interruption and retention sweeps**

Refresh heartbeat on successful event append including empty batches. At `>=24h` inactive, mark interrupted, call a provided bounded finalizer, compact the task, and start report retention. Evict expired/unretained completed reports oldest-first until under 100 MB; preserve retained reports and every active baseline.

- [ ] **Step 5: Run fault-injection checks and full gate**

Run: `go test ./internal/store -count=1 -race && bash scripts/check.sh`

Expected: all crash/recovery/retention tests pass with no files outside task-owned state roots.

- [ ] **Step 6: Commit**

```bash
git add internal/contract/types.go internal/store
git commit -m "feat: persist bounded ARE task evidence"
```

---

### Task 6: Application service and JSON CLI

**Files:**
- Create: `internal/app/service.go`
- Create: `internal/app/service_test.go`
- Create: `internal/cli/run.go`
- Create: `internal/cli/run_test.go`
- Modify: `cmd/agent-residue-evidence/main.go`
- Modify: `internal/versioninfo/version.go`

**Interfaces:**
- Consumes: scope guard, filesystem/process observers, event normalization, correlator, and store.
- Produces: `app.Service` methods `Begin`, `AppendEvents`, `End`, `InspectCompleted`, `GetReport`, and `Verify`, plus matching CLI subcommands and `doctor`.

- [ ] **Step 1: Write failing end-to-end service tests**

Create one task root and one temp root; begin, create a directory/file, start an attributed helper/listener, append safe events, end, assert `REVIEW_REQUIRED`, simulate user-authorized fixture cleanup outside ARE, verify, and assert `NO_LONGER_PRESENT`. Add tests for no-event baseline, retrospective partial evidence, event conflict, interrupted task, broad scope rejection, and observation failure.

- [ ] **Step 2: Run and confirm red**

Run: `go test ./internal/app ./internal/cli -count=1`

Expected: FAIL because service and CLI do not exist.

- [ ] **Step 3: Implement application orchestration**

Make `Service` the only layer that orders begin/end/verify operations. `End` must capture current filesystem/process state before correlation, persist one report, and compact intermediate indexes. `InspectCompleted` must never emit `BASELINE_OBSERVED`. `Verify` must inspect only stable candidate identities from an existing report.

- [ ] **Step 4: Implement CLI commands**

Add `begin`, `event append`, `end`, `inspect-completed`, `report get`, `report retain`, `report forget`, `verify`, `doctor`, `mcp`, and `--version`. Every machine-facing command supports deterministic JSON output and non-zero exit for contract/observation failures. No CLI command deletes user resources or takes an arbitrary cleanup command.

- [ ] **Step 5: Run installed-style CLI journey**

Run:

```bash
build_dir="$(mktemp -d "${TMPDIR:-/tmp}/are-cli-build.XXXXXX")"
go test ./internal/app ./internal/cli -count=1 -race
go build -trimpath -o "$build_dir/agent-residue-evidence" ./cmd/agent-residue-evidence
"$build_dir/agent-residue-evidence" --version
find "$build_dir" -depth -delete
```

Expected: service/CLI tests pass and the built binary reports the expected version and healthy local capabilities.

- [ ] **Step 6: Run full gate and commit**

Run: `bash scripts/check.sh`

```bash
git add cmd internal/app internal/cli internal/versioninfo
git commit -m "feat: expose ARE task lifecycle through CLI"
```

---

### Task 7: Standard stdio MCP and thin Agent Plugin

**Files:**
- Create: `internal/mcpserver/server.go`
- Create: `internal/mcpserver/server_test.go`
- Create: `internal/mcpserver/protocol_test.go`
- Create: `plugin/agent-residue-evidence/.codex-plugin/plugin.json`
- Create: `plugin/agent-residue-evidence/.mcp.json`
- Create: `plugin/agent-residue-evidence/skills/agent-residue-evidence/SKILL.md`
- Create: `scripts/test_plugin_surface.sh`
- Modify: `cmd/agent-residue-evidence/main.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Consumes: `app.Service` and embedded contracts.
- Produces: six MCP tools with exact design names, correct input/output schemas, lifecycle guidance Skill, and generic stdio configuration.

- [ ] **Step 1: Write failing MCP contract tests**

Test initialize, notifications/initialized, tools/list, every tool schema, begin/end/get/verify round trip, retrospective partial report, invalid scope, unknown fields, stderr-only diagnostics, EOF shutdown, and zero network listeners. Assert exactly six tools and no cleanup/execute/delete tool.

```go
func TestToolSurfaceIsEvidenceOnly(t *testing.T) {
    got := sortedToolNames(New(testService(t)))
    want := []string{"append_task_events", "begin_task_observation", "end_task_observation", "get_residue_report", "inspect_completed_task", "verify_task_residue"}
    if diff := cmp.Diff(want, got); diff != "" { t.Fatal(diff) }
}
```

- [ ] **Step 2: Run and confirm red**

Run: `go test ./internal/mcpserver -count=1`

Expected: FAIL because MCP server and dependency are absent.

- [ ] **Step 3: Implement MCP adapter over the application service**

Pin the stable official Go MCP SDK. Register exactly six tools, expose embedded schemas, keep stdout protocol-only, send diagnostics to stderr, and shut down cleanly on EOF/cancellation. Tool descriptions must state task-scoped evidence-only behavior and that ARE never cleans resources.

- [ ] **Step 4: Implement the thin Plugin**

The Skill instructs Agents to call `begin` before the first test/build command, optionally append safe events/heartbeats, call `end` before the final answer, explain candidates to the user, wait for explicit cleanup authorization, let the Agent clean with its own tools, and call `verify`. It forbids full-disk scopes, raw transcript submission, secrets, and automatic cleanup.

- [ ] **Step 5: Run protocol and plugin checks**

Run: `go test ./internal/mcpserver -count=1 -race && bash scripts/test_plugin_surface.sh && bash scripts/check.sh`

Expected: exactly six tools, protocol stdout clean, plugin manifests valid, and no forbidden cleanup capability.

- [ ] **Step 6: Commit**

```bash
git add cmd go.mod go.sum internal/mcpserver plugin scripts/test_plugin_surface.sh
git commit -m "feat: add ARE MCP server and Agent Plugin"
```

---

### Task 8: Deterministic cross-platform packaging and atomic install lifecycle

**Files:**
- Create: `packaging/mcpb/manifest.json.in`
- Create: `packaging/mcp-registry/server.json.in`
- Create: `assets/icon.svg`
- Create: `scripts/build_release_assets.sh`
- Create: `scripts/build_mcpb.py`
- Create: `scripts/package_release_asset.py`
- Create: `scripts/verify_release_assets.sh`
- Create: `scripts/test_install_lifecycle.sh`
- Create: `scripts/test_packaging.sh`
- Create: `docs/install.md`
- Modify: `scripts/check.sh`

**Interfaces:**
- Consumes: final binary, Plugin, schemas, version, and commit metadata.
- Produces: deterministic native archives, exact-byte MCPB, Registry metadata, checksum/SBOM inputs, and clean install/repair/upgrade/rollback/uninstall gates.

- [ ] **Step 1: Write failing packaging and lifecycle tests**

Require fixed archive timestamps, sorted entries, normalized modes, exact embedded native bytes, platform command overrides, no development paths, no caches/tests/secrets, version/commit match, and no network permissions. Lifecycle tests must stop the old MCP process, stage replacement, commit program/config provenance atomically, reconnect, list six tools, and restore the old verified version after an injected failure.

- [ ] **Step 2: Run and confirm red**

Run: `bash scripts/test_packaging.sh && bash scripts/test_install_lifecycle.sh`

Expected: FAIL because packaging and lifecycle scripts do not exist.

- [ ] **Step 3: Implement deterministic native archives and MCPB**

Build macOS arm64 natively on macOS, Linux amd64 natively on Linux, and Windows amd64 natively on Windows. Generate MCPB with all three exact binaries, Plugin Skill/config, icon, license, and platform overrides. Do not download or execute code at first runtime.

- [ ] **Step 4: Implement verification and lifecycle gates**

Verify checksums, executable modes, embedded byte equality, manifest schemas, version/commit, no absolute developer paths, no network permission, and installed `doctor`/MCP tools. Use task-owned install roots and ensure success/failure cleanup.

- [ ] **Step 5: Run local packaging tests and full gate**

Run: `bash scripts/test_packaging.sh && bash scripts/test_install_lifecycle.sh && bash scripts/check.sh`

Expected: deterministic rebuilds match, lifecycle rollback works, and task-owned package/install roots are absent after tests.

- [ ] **Step 6: Commit**

```bash
git add assets docs/install.md packaging scripts go.mod go.sum
git commit -m "build: package ARE for atomic cross-platform install"
```

---

### Task 9: Native acceptance matrix and CI security gates

**Files:**
- Create: `scripts/run_native_acceptance.sh`
- Create: `scripts/run_privacy_acceptance.sh`
- Create: `scripts/run_no_network_acceptance.sh`
- Create: `scripts/verify_zero_residue.sh`
- Create: `.github/workflows/quality.yml`
- Create: `docs/native-acceptance.md`
- Modify: `scripts/check.sh`

**Interfaces:**
- Consumes: source, packages, installed binary/Plugin/MCPB, and task-owned native fixtures.
- Produces: reproducible native macOS/Linux/Windows evidence for standard, retrospective, interrupted, privacy, no-network, install, and zero-residue journeys.

- [ ] **Step 1: Write acceptance assertions before the scripts pass**

The native script must fail unless it proves: standard begin/end, no-event baseline, event binding, conflict preservation, retrospective downgrade, MCP restart recovery, 24-hour fake-clock interruption, Agent-owned cleanup followed by verify, rejected broad roots, symlink/reparse escape defense, process/port attribution, report retention/capacity, no secrets, no network sockets, and zero task-owned residue.

- [ ] **Step 2: Run current-platform acceptance and confirm the first unmet assertion**

Run: `bash scripts/run_native_acceptance.sh`

Expected: FAIL at the first installed or platform contract that the current implementation does not yet satisfy; the script must not silently skip it.

- [ ] **Step 3: Complete native adapters and acceptance fixtures**

Fix only through focused failing tests. Keep platform-specific helpers under unique task roots; use no administrator privileges; record OS, architecture, Go version, commit, binary digest, report IDs, and cleanup result.

- [ ] **Step 4: Add no-network and privacy gates**

Reject runtime imports/configurations that create network clients/listeners, run the installed binary with outbound access unavailable, inspect local sockets, scan source/final archives for secret patterns and absolute developer paths, and assert no raw command/environment/file content reaches reports.

- [ ] **Step 5: Add GitHub Actions native matrix**

Run Go 1.26.x and 1.27.x source gates plus native installed acceptance on macOS arm64, Linux amd64, and Windows amd64. Upload only sanitized acceptance summaries on failure; never upload raw task roots or reports containing private paths.

- [ ] **Step 6: Run all locally available gates**

Run: `bash scripts/check.sh && bash scripts/run_privacy_acceptance.sh && bash scripts/run_no_network_acceptance.sh && bash scripts/run_native_acceptance.sh && bash scripts/verify_zero_residue.sh`

Expected: every locally applicable mandatory gate passes; unavailable remote-native gates remain pending CI evidence and are not called passed.

- [ ] **Step 7: Commit**

```bash
git add .github/workflows/quality.yml docs/native-acceptance.md scripts
git commit -m "test: enforce ARE native acceptance matrix"
```

---

### Task 10: Public documentation, release readiness, and final clean-state audit

**Files:**
- Create: `README.md`
- Create: `LICENSE`
- Create: `NOTICE`
- Create: `SECURITY.md`
- Create: `CONTRIBUTING.md`
- Create: `CHANGELOG.md`
- Create: `docs/quickstart.md`
- Create: `docs/event-contract.md`
- Create: `docs/report-contract.md`
- Create: `docs/threat-model.md`
- Create: `scripts/open_source_check.sh`
- Create: `scripts/verify_release_metadata.sh`
- Modify: `VERSION`
- Modify: `scripts/check.sh`

**Interfaces:**
- Consumes: final contracts, CLI/MCP/Plugin surfaces, packaging, and native evidence.
- Produces: an open-source release candidate whose public claims exactly match native evidence and whose local repository remains clean.

- [ ] **Step 1: Write failing public-surface and metadata tests**

Require README to lead with the task workflow, state evidence-only/no-cleanup/no-network boundaries, document all six tools, show one standard task and one retrospective limitation, list only natively accepted platforms, and avoid pipe-to-shell installation. Require version consistency across binary, Plugin, MCPB, Registry metadata, changelog, and docs.

- [ ] **Step 2: Run and confirm red**

Run: `bash scripts/open_source_check.sh && bash scripts/verify_release_metadata.sh`

Expected: FAIL because public files and version surfaces are incomplete.

- [ ] **Step 3: Write public docs, Apache-2.0 licensing, and security policy**

Use the unmodified Apache License 2.0 text in `LICENSE` and an ARE-specific attribution in `NOTICE`. Explain the workflow `observe → Agent review → user authorization → Agent cleanup → ARE verify`. Document that `NO_CANDIDATES_OBSERVED` is scope-limited, retrospective reports are weaker, Event is optional, reports are local, runtime never uses network, and uninstall does not silently delete retained reports.

- [ ] **Step 4: Freeze the first candidate version and release metadata**

Set one version across every surface, generate changelog and native evidence references, and keep release claims limited to completed macOS/Linux/Windows native gates. Do not create a tag or GitHub Release in this task.

- [ ] **Step 5: Run final complete verification**

Run: `bash scripts/check.sh && bash scripts/open_source_check.sh && bash scripts/verify_release_metadata.sh && bash scripts/run_privacy_acceptance.sh && bash scripts/run_no_network_acceptance.sh && bash scripts/run_native_acceptance.sh && bash scripts/verify_zero_residue.sh`

Then verify:

```bash
git status --short --branch
git worktree list
find "${TMPDIR:-/tmp}" -maxdepth 2 -name 'agent-residue-evidence-*' -print
```

Expected: all mandatory local gates pass, repository has only intended tracked changes before commit, one worktree remains after integration cleanup, and no task-owned temporary paths/processes/ports remain.

- [ ] **Step 6: Commit**

```bash
git add README.md LICENSE NOTICE SECURITY.md CONTRIBUTING.md CHANGELOG.md VERSION docs scripts
git commit -m "docs: prepare ARE open-source release candidate"
```

- [ ] **Step 7: Stop at the publication gate**

Report source tests, native gates, packaging, installed journeys, Git state, and remaining remote publication work separately. Creating the public GitHub repository, tag, Release, Registry entry, or community announcement requires a subsequent explicit release execution step.
