# Contributing

Open an issue before a large behavioral change. Keep ARE evidence-only,
task-scoped, fully local, and free of cleanup or network capability.

Use Go 1.26 or 1.27, add a failing test before implementation, and run:

```bash
bash scripts/check.sh
bash scripts/run_privacy_acceptance.sh
bash scripts/run_no_network_acceptance.sh
bash scripts/run_native_acceptance.sh
bash scripts/verify_zero_residue.sh
```

Platform code requires native evidence on the affected target. Never commit
credentials, private paths, generated release archives, state directories,
test residue, or raw reports from a user's machine. Contributions are licensed
under Apache-2.0.
