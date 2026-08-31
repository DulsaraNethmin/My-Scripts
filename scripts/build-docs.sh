#!/usr/bin/env bash
# Assemble the mkdocs source tree from the markdown already in the repository.
#
#   scripts/build-docs.sh     # -> build/docs/
#   make docs-serve           # build, then mkdocs serve
#
# Nothing here is written by hand: the README, CONTRIBUTING, the docs/ pages
# and every stack's README are the single source of truth, and this copies
# them with their relative links rewritten for the site's layout. Editing
# anything under build/ is editing a file that the next build overwrites.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/build/docs"
GH="https://github.com/DulsaraNethmin/spinup/blob/main"

rm -rf "$OUT"
mkdir -p "$OUT/stacks"

# rewrite reads a file on stdin and fixes the links that only make sense in a
# checkout. One sed per link rather than a clever pattern, so that a link this
# does not know about shows up as a broken one in `mkdocs build --strict`
# rather than being silently mangled.
rewrite() {
  sed \
    -e 's|](docs/PLAN\.md)|](design.md)|g' \
    -e 's|](docs/PORTS\.md)|](ports.md)|g' \
    -e 's|](CONTRIBUTING\.md)|](contributing.md)|g' \
    -e "s|](setup/)|]($GH/setup)|g" \
    -e "s|](LICENSE)|]($GH/LICENSE)|g"
}

rewrite < "$ROOT/README.md"        > "$OUT/index.md"
rewrite < "$ROOT/CONTRIBUTING.md"  > "$OUT/contributing.md"
rewrite < "$ROOT/docs/PLAN.md"     > "$OUT/design.md"
rewrite < "$ROOT/docs/PORTS.md"    > "$OUT/ports.md"
rewrite < "$ROOT/docs/RELEASING.md" > "$OUT/releasing.md"
rewrite < "$ROOT/CHANGELOG.md"     > "$OUT/changelog.md"

# One page per stack, from the README `spinup info` already prints.
for dir in "$ROOT"/stacks/*/; do
  name="$(basename "$dir")"
  [ -f "$dir/README.md" ] || continue
  rewrite < "$dir/README.md" > "$OUT/stacks/$name.md"
done

# The catalog index, built from each stack's own metadata so it cannot drift
# from what the CLI reads.
{
  echo "# The catalog"
  echo
  echo "Every stack keeps its service on its well-known port and puts any web"
  echo "GUI in the \`80xx\` range, so all of them can run at the same time. A"
  echo "\`gui\` in the last column means the web interface is a container of its"
  echo "own and starts only with \`--gui\`; otherwise it is the service itself."
  echo
  echo "| Stack | What you get | Ports | GUI |"
  echo "| --- | --- | --- | --- |"
  for dir in "$ROOT"/stacks/*/; do
    name="$(basename "$dir")"
    meta="$dir/spinup.yaml"
    [ -f "$meta" ] || continue
    desc="$(awk -F': *' '/^description:/{sub(/^description: */,""); print; exit}' "$meta")"
    ports="$(awk '/^ *- *name:/{n=1} /^ *default:/{gsub(/[^0-9]/,""); printf "%s%s", (c++?", ":""), $0}' "$meta")"
    # shellcheck disable=SC2016  # the backticks are markdown, not a subshell
    if grep -qE '^profiles: *\[gui\]' "$meta"; then gui='`gui`'; else gui='—'; fi
    echo "| [\`$name\`](${name}.md) | $desc | $ports | $gui |"
  done
  echo
  echo "Ports are allocated centrally in [the port registry](../ports.md)."
} > "$OUT/stacks/index.md"

echo "built $OUT ($(find "$OUT" -name '*.md' | wc -l | tr -d ' ') pages)"
