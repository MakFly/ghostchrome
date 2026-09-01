#!/usr/bin/env bash
# Validate the canonical Ghostchrome skill without installing or modifying it.
set -euo pipefail

skill_dir=${1:-$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)}
skill_file=$skill_dir/SKILL.md

fail=0
error() {
  printf 'validate-skill: %s\n' "$1" >&2
  fail=1
}

[[ -f $skill_file ]] || { error "missing SKILL.md"; exit 1; }

if [[ $(sed -n '1p' "$skill_file") != '---' ]]; then
  error 'frontmatter must start on the first line'
fi
if ! awk 'NR > 1 && $0 == "---" { found=1; exit } END { exit(found ? 0 : 1) }' "$skill_file"; then
  error 'frontmatter closing delimiter is missing'
fi

frontmatter=$(awk '
  NR == 1 && $0 == "---" { inside=1; next }
  inside && $0 == "---" { exit }
  inside { print }
' "$skill_file")
body=$(awk '
  NR == 1 && $0 == "---" { inside=1; next }
  inside && $0 == "---" { inside=0; closed=1; next }
  closed { print }
' "$skill_file")

grep -Eq '^name: ghostchrome[[:space:]]*$' <<<"$frontmatter" || error 'frontmatter name must be ghostchrome'
grep -Eq '^description: This skill should be used when ' <<<"$frontmatter" || error 'description must use the third-person trigger form'
grep -Eiq '\b(you|your|you'"'"'re|you'"'"'ll)\b' <<<"$body" && error 'body contains second-person wording'
grep -Eiq '\b(TODO|FIXME|PLACEHOLDER)\b' <<<"$body" && error 'body contains an unfinished placeholder'

word_count=$(printf '%s\n' "$body" | wc -w | tr -d '[:space:]')
if (( word_count < 1500 || word_count > 2200 )); then
  error "body has $word_count words; target is 1500-2200"
fi

for ref in cli.md mcp.md troubleshooting.md packaging.md; do
  [[ -f $skill_dir/references/$ref ]] || error "missing reference: references/$ref"
  grep -Fq "references/$ref" "$skill_file" || error "SKILL.md does not link references/$ref"
done
for example in cli-flow.sh mcp-config.toml; do
  [[ -f $skill_dir/examples/$example ]] || error "missing example: examples/$example"
done
[[ -x $skill_dir/examples/cli-flow.sh ]] || error 'examples/cli-flow.sh must be executable'

if (( fail != 0 )); then
  exit 1
fi
printf 'validate-skill: OK (%s words)\n' "$word_count"
