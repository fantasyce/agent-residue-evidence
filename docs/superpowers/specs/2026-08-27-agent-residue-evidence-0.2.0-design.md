# Agent Residue Evidence 0.2.0 Task-Isolation Design

- Date: 2026-08-27
- Status: approved for autonomous implementation and acceptance
- Supersedes: task access and report lifecycle portions of the 0.1.0 design

## 1. Outcome

ARE 0.2.0 remains a fully local, deterministic evidence provider. It observes
task-scoped test and build residue, gives an Agent evidence for a user decision,
and verifies only the previously reported candidates. It never deletes files,
stops processes, closes ports, runs cleanup commands, or decides that an object
is safe to remove.

The 0.2.0 security correction is possession-based task isolation. A caller that
knows a task or report identifier but does not possess a valid capability must
not read, change, retain, forget, resolve, or verify that task's evidence.

## 2. Approved product decisions

- There is no global recovery key. Loss of the owner recovery handle makes the
  task evidence permanently inaccessible; expiry later removes ARE's ciphertext.
- Prospective observation is the default. Retrospective inspection requires an
  explicit local grant and is always labelled lower-confidence evidence.
- Agent-visible file paths use bound aliases such as `workspace://build/logs`.
  Exact paths stay inside encrypted state and are revealed only for an approved
  candidate by an authorized owner.
- Reports and verification snapshots are immutable. Verification appends a new
  snapshot linked to the original observation; it never overwrites history.
- Candidate presentation is grouped and bounded. A generated directory tree is
  one summary candidate by default; file detail is local and paginated.
- One owner may create short-lived, append-only executor capabilities. Executors
  cannot read reports, resolve paths, end tasks, retain, forget, or verify.
- Reports declare one of `MANAGED`, `GUIDED`, or `RETROSPECTIVE` observation
  modes. The mode is evidence, not marketing copy.
- Active observations expire after 24 hours without heartbeat. Completed report
  chains expire after seven days unless an authorized owner retains them.
- The evidence engine is deterministic and performs no network or model calls.
  An external Agent may explain evidence but cannot alter ARE's conclusions.
- No 0.1.0 migration layer is shipped. Existing 0.1.0 local test state is removed
  before formal acceptance and 0.2.0 begins with a fresh state home.

## 3. Trust boundary and recovery profiles

ARE protects against accidental or opportunistic cross-task access by callers
that can enumerate ARE state or guess identifiers. Encrypted records use random
owner key material and authenticated encryption; filenames are opaque digests.
Plaintext task identifiers, absolute paths, candidate details, commands,
transcripts, environments, secrets, and credentials must not appear in state
files or filenames.

ARE cannot isolate a task from a process that can read that task's complete
host transcript, process memory, or valid recovery handle. Generic MCP has no
standard hidden credential channel or portable task identity. Therefore:

- `recoverable` is the generic default: an opaque owner handle may cross the MCP
  tool boundary and can be stored by the host. It is never included in reports,
  logs, errors, documentation examples with real values, or final prose.
- `ephemeral` keeps ownership in the current server session. Restart loses access.
- Integrated hosts may keep the same owner handle in a host-private credential
  channel. This is an optional host improvement, never a core security premise.

Handles are capability envelopes, not identities. Sharing a handle delegates its
authority. Invalid-capability errors are deliberately generic and do not confirm
whether a requested task or report exists.

## 4. Lifecycle and public model

### 4.1 Begin

`begin_task_observation` accepts display metadata, narrow roots, observation
mode, and recovery profile. ARE generates:

- random observation and owner IDs;
- random owner key material;
- a root alias table and stable root-binding fingerprints;
- encrypted filesystem/process baselines;
- an opaque owner handle when the profile is recoverable.

Caller-provided `task_id` is display metadata only. It is never an authorization
key or a storage filename.

### 4.2 Append and delegation

The owner may append safe events or mint executor handles. An executor handle is
bound to one observation, append-only, expiry-limited, and optionally restricted
to event types and root aliases. Ending an observation revokes all executors.

### 4.3 End

Only the owner may end an observation. ARE compares the encrypted baseline with
current state and writes immutable report revision zero. The normal response is
a bounded summary: report ID, status, observation mode, limitations, counts,
aggregate sizes, and the first page of grouped candidates.

### 4.4 User decision and exact target resolution

ARE does not store the conversation or assert authorization. After the Agent has
obtained explicit user approval, the owner may resolve only candidate IDs already
present in the report. Resolution is not a directory browser and returns no
siblings, parents, or unrelated children.

### 4.5 Verification

Verification rechecks only stable identities from an existing report chain and
appends revision N+1. The original status remains readable. Each revision binds
the previous revision digest, creation time, candidate states, limitations, and
mode. Revisions use authenticated encryption; a digest alone is not presented as
protection against a same-user writer.

### 4.6 Retention

Unfinished encrypted observations expire after 24 hours. Completed chains expire
after seven days. Retain and forget require owner authority. Capacity eviction
uses only a minimal unencrypted envelope containing opaque object name, record
kind, ciphertext size, and expiry time. It does not decrypt another task.

## 5. Path aliases and grouped evidence

Roots are assigned stable aliases inside one observation: `workspace://`,
`temp://0/`, and subsequent bounded roots. Aliases are mapped to canonical roots
inside ciphertext and bound to stable root identity. Symlink, junction, reparse,
or root-replacement escape yields `UNKNOWN` or a limitation.

Directory aggregation follows these rules:

- a newly created directory covers newly created descendants unless a descendant
  has a conflicting status, process reference, or independently declared output;
- summary includes descendant count, aggregate bytes, evidence level, status,
  limitations, and at most three representative relative samples;
- process and port candidates remain separate;
- detail pages are deterministic, filtered, cursor-based, and capped;
- no response may silently truncate: it includes total count and next cursor.

`NO_CANDIDATES_OBSERVED` remains scope-limited and never means host clean.

## 6. Retrospective inspection

Retrospective inspection is disabled unless the request carries an explicit,
short-lived scope grant minted locally for the exact roots and time window. Its
report mode is `RETROSPECTIVE`, status is at least `PARTIAL_EVIDENCE`, and no
candidate receives `BASELINE_OBSERVED`. It proves only current observations and
time/path correlation, not that the named task created the objects.

## 7. Interface evolution

The 0.2.0 MCP surface stays evidence-only. Existing tools receive owner handles
instead of trusting `task_id` or `report_id`; bounded detail and candidate
resolution may be exposed as read-only evidence tools. Delegation may be exposed
as a capability-management tool, but there is still no cleanup, delete-resource,
execute, terminate, or close-port tool.

CLI JSON uses the same contracts. Secrets are accepted through JSON stdin or a
private handle file, never command-line flags. Human-readable output redacts all
handles. CLI and MCP errors never echo supplied capabilities.

## 8. Storage format

Each record is an envelope plus authenticated ciphertext:

```text
format_version, record_kind, opaque_id, created_at, expires_at,
nonce, ciphertext
```

The encryption key is derived from random owner key material plus record context.
AES-256-GCM from the Go standard library is used with a fresh random nonce per
write. Associated data binds format version, record kind, opaque ID, and chain
revision. Atomic private writes remain mandatory.

The store provides no list/read-by-ID API without a capability. A task or report
ID is insufficient to derive a filename or key. Plaintext scans over a populated
state home must not find task display IDs, project paths, candidate names, or
report contents.

## 9. Required acceptance

Source gates:

- red/green tests for capability enforcement, opaque filenames, authenticated
  encryption, wrong-handle denial, tamper rejection, expiry, and atomic writes;
- red/green tests for aliases, root binding, grouping, pagination, immutable
  revisions, and retrospective grants;
- MCP and CLI contract tests proving identifiers alone never authorize access;
- scans proving state and normal output contain no absolute paths or capabilities;
- no-network and evidence-only tool-surface checks.

Real local Codex acceptance uses the formally installed candidate plugin:

1. Codex task A starts an observation and produces a unique residue candidate.
2. Codex task B receives only task/report IDs and cannot read, verify, resolve,
   retain, forget, append to, or end task A.
3. Task A can end, inspect grouped aliases, resolve an approved candidate, and
   append a verification revision.
4. Restart recovery is tested for the recoverable profile; ephemeral loss is
   separately verified.
5. A clean state-home scan finds no plaintext task IDs, workspace paths, candidate
   names, or real capability values.

Native macOS arm64, Linux amd64, and Windows amd64 acceptance remains mandatory.
Release is permitted only after source, packaged asset, installed plugin, live
MCP, cross-task Codex, no-network, privacy, and zero-residue gates pass on the
final bytes.

