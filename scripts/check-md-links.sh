#!/usr/bin/env bash
# Checks that every relative link in a tracked Markdown file resolves to an
# existing path. Prints each broken link and exits with 1 if it finds one.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

broken=0
while read -r f; do
  while read -r link; do
    target="$(dirname "$f")/${link%%#*}"
    if [ ! -e "$target" ]; then
      echo "broken: $f -> $link"
      broken=1
    fi
  done < <(grep -oE '\]\((\.[^)]+)\)' "$f" | sed 's/^](//; s/)$//')
done < <(git ls-files '*.md')
exit $broken
