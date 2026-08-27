#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"

GOPROXY=off GOSUMDB=off go test -C "$repo_dir" -count=1 \
  -run 'TestPublicIdentifiersNeverAuthorizeCrossTaskOperations|TestReportIDAloneAndWrongOwnerCannotAccessAnotherTask|TestEphemeralObservationStaysInSessionAndCannotResumeAfterRestart|TestExecutorHandleFieldCompletesMCPAppendRoundTrip' \
  ./internal/mcpserver
GOPROXY=off GOSUMDB=off go test -C "$repo_dir" -count=1 \
  -run 'TestExecutorHandleDoesNotContainOwnerRecordKey|TestOwnedExecutorIsAppendOnlyAndRevokedOnCompletion|TestCapacityPreservesRetainedEncryptedReport' \
  ./internal/capability ./internal/store

echo 'TASK_ISOLATION_ACCEPTANCE=PASS public_ids=NON_AUTHORITY executor=APPEND_ONLY ephemeral=SESSION_BOUND retained=PROTECTED'
