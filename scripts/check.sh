#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
cd "$repo_dir"

unformatted="$(gofmt -l .)"
if [[ -n "$unformatted" ]]; then
  printf 'gofmt required:\n%s\n' "$unformatted" >&2
  exit 1
fi

go vet ./...
go mod verify
go test -count=1 -race ./...
bash scripts/test_plugin_surface.sh
bash scripts/run_privacy_acceptance.sh
bash scripts/run_task_isolation_acceptance.sh
bash scripts/test_site.sh
bash scripts/test_launch_surface.sh
bash scripts/test_residue_demo.sh
bash scripts/open_source_check.sh
bash scripts/verify_release_metadata.sh
git diff --check
