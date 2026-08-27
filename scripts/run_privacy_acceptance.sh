#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"; repo_dir="$(cd "$script_dir/.." && pwd -P)"
GOPROXY=off GOSUMDB=off go test -C "$repo_dir" -count=1 ./internal/contract ./internal/event ./internal/app
if rg -n --glob '!**/*_test.go' --glob '!docs/**' --glob '!scripts/run_privacy_acceptance.sh' \
  'os\.Environ|CommandLine|Cmdline|ReadFile\([^)]*candidate|raw_command|environment_values' "$repo_dir/internal"; then
  echo 'privacy-forbidden runtime source found' >&2; exit 1
fi
if rg -n 'BEGIN (RSA |OPENSSH |EC )?PRIVATE KEY|AKIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9]{20,}' "$repo_dir" \
  --glob '!docs/superpowers/**' --glob '!scripts/run_privacy_acceptance.sh'; then
  echo 'credential-like source content found' >&2; exit 1
fi
echo 'PRIVACY_ACCEPTANCE=PASS raw_commands=ABSENT environments=ABSENT file_contents=ABSENT secrets=ABSENT'
