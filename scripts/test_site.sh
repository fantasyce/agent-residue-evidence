#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
index="$repo_dir/site/index.html"

[[ -f "$index" && -f "$repo_dir/site/styles.css" && -f "$repo_dir/site/are-residue-checkpoint.svg" && -f "$repo_dir/site/404.html" ]]
[[ "$(grep -Eoc '<h1([ >])' "$index")" -eq 1 ]]
for landmark in header main footer nav; do grep -Eq "<$landmark([ >])" "$index"; done
grep -Fq 'Tests finish. Residue remains.' "$index"
grep -Fq 'name="viewport"' "$index"
grep -Fq 'docs/demo.md' "$index"
grep -Fq 'releases/latest' "$index"
grep -Fq 'SECURITY.md' "$index"
grep -Fq 'github.com/fantasyce/agent-residue-evidence' "$index"
grep -Fq 'NO_CANDIDATES_OBSERVED' "$index"
grep -Fq '<title' "$repo_dir/site/are-residue-checkpoint.svg"
grep -Fq '<desc' "$repo_dir/site/are-residue-checkpoint.svg"
grep -Fq 'actions/configure-pages@983d7736d9b0ae728b81ab479565c72886d7745b' "$repo_dir/.github/workflows/pages.yml"
grep -Fq 'actions/upload-pages-artifact@7b1f4a764d45c48632c6b24a0339c27f5614fb0b' "$repo_dir/.github/workflows/pages.yml"
grep -Fq 'actions/deploy-pages@d6db90164ac5ed86f2b6aed7e0febac5b3c0c03e' "$repo_dir/.github/workflows/pages.yml"

if rg -n 'https?://[^" ]+\.(js|css|woff2?|ttf)|<script|googletag|segment\.com|plausible|analytics|href="#"|TODO|PLACEHOLDER' "$repo_dir/site"; then
  echo 'site contains an external dependency, tracker, script, or placeholder' >&2
  exit 1
fi

echo 'site tests passed'
