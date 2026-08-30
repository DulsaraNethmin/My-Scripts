#!/usr/bin/env bash
# spinup progress tracker.
# The ledger in docs/TASKS.tsv is the source of truth for what is done and what
# is next, so that a fresh context can pick up exactly where the last one stopped.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LEDGER="$ROOT/docs/TASKS.tsv"

if [ -n "${NO_COLOR:-}" ] || [ ! -t 1 ]; then
  B=""; DIM=""; RED=""; GRN=""; YEL=""; BLU=""; R=""
else
  B=$'\033[1m'; DIM=$'\033[2m'; RED=$'\033[31m'; GRN=$'\033[32m'
  YEL=$'\033[33m'; BLU=$'\033[34m'; R=$'\033[0m'
fi

die() { printf '%serror:%s %s\n' "$RED" "$R" "$*" >&2; exit 1; }
rows() { grep -v '^#' "$LEDGER" | grep -v '^[[:space:]]*$'; }
field() { rows | awk -F'|' -v id="$1" -v n="$2" '$1==id{print $n; found=1} END{exit !found}'; }
have()  { rows | awk -F'|' -v id="$1" '$1==id{f=1} END{exit !f}'; }

icon() {
  case "$1" in
    done)   printf '%s[x]%s' "$GRN" "$R" ;;
    wip)    printf '%s[~]%s' "$YEL" "$R" ;;
    manual) printf '%s[!]%s' "$BLU" "$R" ;;
    *)      printf '%s[ ]%s' "$DIM" "$R" ;;
  esac
}

cmd_list() {
  local phase="" phase_r id st br title
  while IFS='|' read -r id phase_r st br title; do
    if [ "$phase_r" != "$phase" ]; then
      phase="$phase_r"
      printf '\n%s%s%s\n' "$B" "$phase" "$R"
    fi
    printf '  %s %-5s %s' "$(icon "$st")" "$id" "$title"
    [ "$st" = "wip" ] && printf '  %s(%s)%s' "$DIM" "$br" "$R"
    printf '\n'
  done < <(rows)
  printf '\n'
}

cmd_status() {
  local total done_n wip_n manual_n todo_n pct filled i bar
  total=$(rows | wc -l | tr -d ' ')
  done_n=$(rows | awk -F'|' '$3=="done"' | wc -l | tr -d ' ')
  wip_n=$(rows | awk -F'|' '$3=="wip"' | wc -l | tr -d ' ')
  manual_n=$(rows | awk -F'|' '$3=="manual"' | wc -l | tr -d ' ')
  todo_n=$(rows | awk -F'|' '$3=="todo"' | wc -l | tr -d ' ')
  pct=$(( done_n * 100 / total ))
  filled=$(( pct * 30 / 100 ))
  bar=""
  for ((i=0;i<30;i++)); do
    if [ "$i" -lt "$filled" ]; then bar="$bar#"; else bar="$bar."; fi
  done
  printf '\n%sspinup progress%s\n\n' "$B" "$R"
  printf '  %s%s%s  %s%d%%%s  (%d/%d complete)\n\n' "$GRN" "$bar" "$R" "$B" "$pct" "$R" "$done_n" "$total"
  printf '  %sdone%s %-3d  %swip%s %-3d  %stodo%s %-3d  %smanual%s %-3d\n\n' \
    "$GRN" "$R" "$done_n" "$YEL" "$R" "$wip_n" "$DIM" "$R" "$todo_n" "$BLU" "$R" "$manual_n"

  if [ "$wip_n" -gt 0 ]; then
    printf '  %sIn progress:%s\n' "$B" "$R"
    rows | awk -F'|' '$3=="wip"{printf "    %-5s %s  [%s]\n", $1, $5, $4}'
    printf '\n'
  fi
  printf '  %sNext up:%s\n' "$B" "$R"
  rows | awk -F'|' '$3=="todo"{printf "    %-5s %s\n", $1, $5; if (++n==3) exit}'
  printf '\n  %sgit%s  branch %s, %s uncommitted file(s)\n\n' "$DIM" "$R" \
    "$(git -C "$ROOT" rev-parse --abbrev-ref HEAD)" \
    "$(git -C "$ROOT" status --porcelain | wc -l | tr -d ' ')"
}

cmd_next() { rows | awk -F'|' '$3=="todo"{print $1"  "$5; exit}'; }

cmd_set() {
  local id="$1" new="$2"
  have "$id" || die "no such task: $id"
  local tmp; tmp="$(mktemp)"
  awk -F'|' -v OFS='|' -v id="$id" -v st="$new" \
    '/^#/ || NF<5 {print; next} $1==id {$3=st} {print}' "$LEDGER" > "$tmp"
  mv "$tmp" "$LEDGER"
  printf '%s%s%s -> %s  %s\n' "$B" "$id" "$R" "$new" "$(field "$id" 5)"
}

cmd_start() {
  local id="${1:-}"; [ -n "$id" ] || die "usage: progress.sh start <id>"
  have "$id" || die "no such task: $id"
  local br; br="$(field "$id" 4)"
  if [ "$br" != "-" ] && [ -n "$br" ]; then
    if git -C "$ROOT" show-ref --quiet --verify "refs/heads/$br"; then
      git -C "$ROOT" checkout "$br"
    else
      git -C "$ROOT" checkout -b "$br"
    fi
  fi
  cmd_set "$id" wip
}

cmd_done() {
  local id="${1:-}"; [ -n "$id" ] || die "usage: progress.sh done <id>"
  cmd_set "$id" done
}

cmd_handoff() {
  local id="${1:-}" br title phase
  if [ -z "$id" ]; then
    id="$(rows | awk -F'|' '$3=="todo"{print $1; exit}')"
  fi
  [ -n "$id" ] || die "nothing left to hand off"
  have "$id" || die "no such task: $id"
  phase="$(field "$id" 2)"; br="$(field "$id" 4)"; title="$(field "$id" 5)"

  cat <<HANDOFF
# Handoff — spinup task $id

Continue the spinup project at /Users/nethmindulsara/Projects/BuildwNethmin/My-Scripts.
Read CLAUDE.md first — it has the working rules — then docs/PLAN.md for the full design.

## Your task

**$id — $title**
Phase: $phase
Branch: $br

## State right now

$(cd "$ROOT" && "$0" list | sed 's/^/    /')

## Rules (also in CLAUDE.md)

- Commits are authored by DulsaraNethmin <dulsaranethmin@gmail.com>. Never add a
  Co-Authored-By: Claude trailer or any Claude attribution to a commit message.
- Never push to origin. The user pushes.
- Work on branch \`$br\`, then merge into main locally with --no-ff.
- Run \`make start ID=$id\` to begin and \`make done ID=$id\` when finished.
- Finish by writing the next handoff: \`make handoff\`.

Last commit on main: $(git -C "$ROOT" log -1 --format='%h %s' main)
HANDOFF
}

case "${1:-status}" in
  list|ls)      cmd_list ;;
  status|st)    cmd_status ;;
  next)         cmd_next ;;
  start)        shift; cmd_start "$@" ;;
  done)         shift; cmd_done "$@" ;;
  wip)          shift; cmd_set "${1:?usage: wip <id>}" wip ;;
  todo|reopen)  shift; cmd_set "${1:?usage: todo <id>}" todo ;;
  handoff)      shift; cmd_handoff "${1:-}" ;;
  *)            die "unknown command: $1 (list|status|next|start|done|todo|handoff)" ;;
esac
