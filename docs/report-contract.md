# Residue report contract

Reports use `agent-residue-report/2.0`. Status is one of:

- `NO_CANDIDATES_OBSERVED`
- `REVIEW_REQUIRED`
- `PARTIAL_EVIDENCE`
- `INTERRUPTED_TASK`
- `OBSERVATION_FAILED`

Candidates are files, directories, stable processes, or listening ports owned
by an attributed process. Each candidate has a stable ID, exactly one evidence
level, an independent current status, provenance references when available,
limitations/conflicts, and only the recommendation `review`.

Evidence strength is `BASELINE_OBSERVED`, `RECEIPT_BOUND`, `EVENT_BOUND`,
`INFERRED`, or `UNATTRIBUTED`. Current state is `PRESENT`,
`ACTIVE_REFERENCE`, `NO_LONGER_PRESENT`, `CHANGED_SINCE_REPORT`, or `UNKNOWN`.
Verification rechecks only report candidates by stable identity and appends an
immutable, digest-chained revision; it does not widen or rewrite the original
report.

Stored state is encrypted and public reports use root aliases. Exact local
paths require the same owner capability and an explicit candidate resolution.
Do not publish capabilities or resolved paths. `NO_CANDIDATES_OBSERVED` is limited to
the declared task roots and available evidence.
