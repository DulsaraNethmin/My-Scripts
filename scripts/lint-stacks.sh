#!/usr/bin/env bash
# Structural lint for the stack catalog.
#
# Enforces the conventions in docs/PLAN.md section 4, so that adding a stack
# never needs a Go change and so no stack drifts from the shape the CLI expects.
# `docker compose config` is a separate check (make stacks-validate); this one
# is about the contract around the compose file.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STACKS="$ROOT/stacks"

if [ -n "${NO_COLOR:-}" ] || [ ! -t 1 ]; then
  RED=""; GRN=""; YEL=""; R=""
else
  RED=$'\033[31m'; GRN=$'\033[32m'; YEL=$'\033[33m'; R=$'\033[0m'
fi

fails=0
warns=0
fail() { printf '  %sFAIL%s  %-22s %s\n' "$RED" "$R" "$1" "$2"; fails=$((fails+1)); }
warn() { printf '  %swarn%s  %-22s %s\n' "$YEL" "$R" "$1" "$2"; warns=$((warns+1)); }

[ -d "$STACKS" ] || { echo "no stacks/ directory yet"; exit 0; }

for dir in "$STACKS"/*/; do
  [ -d "$dir" ] || continue
  name="$(basename "$dir")"
  compose="$dir/compose.yaml"
  envex="$dir/.env.example"
  meta="$dir/spinup.yaml"

  # 1. the four required files
  for f in compose.yaml .env.example spinup.yaml README.md; do
    [ -f "$dir/$f" ] || fail "$name" "missing $f"
  done
  [ -f "$compose" ] || continue

  # 2. folder name is kebab-case and matches spinup.yaml's name:
  # LC_ALL=C: under other locales [A-Z] collates against lowercase too.
  if LC_ALL=C grep -qE '[^a-z0-9-]' <<<"$name"; then
    fail "$name" "folder name must be kebab-case"
  fi
  if [ -f "$meta" ]; then
    declared="$(awk -F': *' '/^name:/{print $2; exit}' "$meta")"
    [ "$declared" = "$name" ] || fail "$name" "spinup.yaml name: is '$declared', expected '$name'"
    # A worker stack binds no host port: there is no url to print and no
    # port to claim, and its cli is the only way in. internal/catalog
    # applies the same exception — change one and change the other.
    required="description category primary url ports"
    if grep -qE '^worker: *true' "$meta"; then
      required="description category primary cli"
    fi
    for k in $required; do
      grep -q "^$k:" "$meta" || fail "$name" "spinup.yaml missing key: $k"
    done
    cat="$(awk -F': *' '/^category:/{print $2; exit}' "$meta")"
    case "$cat" in
      database|messaging|storage|tooling|ml|web) ;;
      *) fail "$name" "unknown category: $cat" ;;
    esac
  fi

  # 3. compose hygiene
  grep -qE '^version:' "$compose"           && fail "$name" "obsolete top-level version: key"
  grep -qE 'image:.*:latest' "$compose"     && fail "$name" "image pinned to :latest"
  grep -qE 'image:[^:]*$' "$compose"        && fail "$name" "image with no tag"
  grep -q  'container_name:' "$compose"     && fail "$name" "container_name: collides with user projects"
  grep -q  'runtime: *nvidia' "$compose"    && fail "$name" "deprecated runtime: nvidia (use deploy.resources)"
  grep -qE '^\s*-\s*"?\$\{[A-Z_]+:-80\}?:' "$compose" \
    && [ "$name" != "nginx-proxy-manager" ] && warn "$name" "binds host port 80 (reserved for the proxy stack)"

  # 4. healthchecks probe 127.0.0.1, never localhost (localhost can resolve
  #    to ::1 while the service listens on IPv4 only)
  grep -qE '^\s*test:.*localhost' "$compose" \
    && fail "$name" "healthcheck probes localhost; use 127.0.0.1"

  # 5. env contract: declared vars are used, used vars are declared.
  #    $${VAR} is a container-side reference, not an env-file variable.
  if [ -f "$envex" ]; then
    while IFS= read -r v; do
      [ -n "$v" ] || continue
      grep -qF "\${$v" "$compose" || fail "$name" "$v declared in .env.example but unused in compose.yaml"
    done < <(grep -oE '^[A-Z_][A-Z0-9_]*=' "$envex" | tr -d '=')

    while IFS= read -r v; do
      [ -n "$v" ] || continue
      grep -qE "^$v=" "$envex" || fail "$name" "\${$v} used in compose.yaml but not documented in .env.example"
    done < <(perl -ne 'while (/(?<!\$)\$\{([A-Z_][A-Z0-9_]*)[:}]/g) { print "$1\n" }' "$compose" | sort -u)
  fi

  # 6. every port in spinup.yaml is a real env var with a documented default
  if [ -f "$meta" ] && [ -f "$envex" ]; then
    while IFS= read -r v; do
      [ -n "$v" ] || continue
      grep -qE "^$v=" "$envex" || fail "$name" "spinup.yaml port $v missing from .env.example"
    done < <(awk '/^ports:/,/^[a-z_]+:/' "$meta" | awk -F': *' '/- *name:/{print $2}')
  fi

  [ "$fails" -eq 0 ] && printf '  %sok%s    %s\n' "$GRN" "$R" "$name"
done

printf '\n'
if [ "$fails" -gt 0 ]; then
  printf '%s%d failure(s)%s' "$RED" "$fails" "$R"
  [ "$warns" -gt 0 ] && printf ', %d warning(s)' "$warns"
  printf '\n'; exit 1
fi
printf '%sall stacks pass%s' "$GRN" "$R"
[ "$warns" -gt 0 ] && printf ' (%d warning(s))' "$warns"
printf '\n'
