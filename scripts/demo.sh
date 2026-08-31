#!/usr/bin/env bash
# Drive the sequence the README's demo shows, at a readable pace.
#
# It exists so that recording the GIF is one command rather than a
# performance: run this under a recorder and the timing is the same every time.
#
#   brew install asciinema agg          # or your package manager's equivalent
#   asciinema rec docs/demo.cast -c scripts/demo.sh --overwrite
#   agg --cols 84 --rows 24 docs/demo.cast docs/demo.gif
#
# Then drop the image into README.md where the TODO(demo) comment is.
#
# It uses a scratch SPINUP_HOME and destroys the stack afterwards, so it
# cannot touch your own ~/.spinup or leave containers behind. It does bind
# postgres's real ports — 5432 and 8080 — so stop anything already on them.
#
# SPINUP_DEMO_PORTS passes extra flags to `up`, which is how to rehearse the
# script on a machine where those ports are busy:
#
#   SPINUP_DEMO_PORTS='--port POSTGRES_PORT=15432 --port PGADMIN_PORT=18080' \
#     scripts/demo.sh
#
# Record with the defaults, though — the numbers in the README are the ones
# people will see.
set -euo pipefail

SPINUP="${SPINUP:-./bin/spinup}"
[ -x "$SPINUP" ] || { echo "no $SPINUP — run make build first" >&2; exit 1; }

SPINUP_HOME="$(mktemp -d)"
export SPINUP_HOME
cleanup() {
  "$SPINUP" destroy postgres -y >/dev/null 2>&1 || true
  rm -rf "$SPINUP_HOME"
}
trap cleanup EXIT

# type prints a command the way a person would, then runs it.
type_out() {
  printf '$ '
  printf '%s' "$*" | while IFS= read -r -n1 c; do printf '%s' "$c"; sleep 0.03; done
  printf '\n'
  sleep 0.4
  "$@"
  sleep 1.2
}

# shellcheck disable=SC2206  # deliberate word splitting: these are flags
extra=(${SPINUP_DEMO_PORTS:-})

clear
type_out "$SPINUP" up postgres --gui "${extra[@]}"
type_out "$SPINUP" ps postgres
type_out "$SPINUP" url postgres
type_out "$SPINUP" destroy postgres -y
sleep 1
