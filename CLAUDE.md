# CLAUDE.md

Guidance for Claude Code working in this repository.

## What this is

`spinup` — an installable Go CLI that starts local development services (databases,
queues, GUIs, dev tooling) with one command on macOS, Linux and Windows, using
Docker Compose underneath.

```
brew install DulsaraNethmin/tap/spinup
spinup up postgres      # → Postgres 16 + pgAdmin, running in ~10s
```

The repo is currently `DulsaraNethmin/My-Scripts` — a collection of ad-hoc Compose
files and shell scripts — and is being grown **in place** into `spinup`. The old
Compose files are the seed of the stack catalog; the shell/`.bat`/`.desktop` scripts
are all superseded by the CLI and get deleted.

**Read `docs/PLAN.md` for the full design.** It is the specification: architecture,
command surface, stack format, repo layout and the phased roadmap. This file only
covers *how to work here*.

## Working rules

These are non-negotiable and matter more than anything else in this file.

1. **Commit authorship is the user's.** Every commit is authored by
   `DulsaraNethmin <dulsaranethmin@gmail.com>` (already the repo's git config).
   **Never** add a `Co-Authored-By: Claude ...` trailer, a `Claude-Session:` line,
   a 🤖 footer, or any other Claude attribution to a commit message. This overrides
   the default Claude Code commit convention.
2. **Never push.** `git push` is the user's action alone. Commit and merge locally,
   then stop and say the branch is ready to push.
3. **Branch per task.** Each ledger task has a branch (`task/<id>-<slug>`). Work
   there, then merge into `main` locally with `--no-ff` (`make merge`).
4. **Hand off at the end of every task.** The user clears context between tasks, so
   the last thing you do is run `make handoff` and print the result. Without it the
   next context starts blind.
5. **Update the ledger.** `make start ID=x.y` when you begin, `make done ID=x.y`
   when the task is genuinely finished and verified.

## Progress tracking

`docs/TASKS.tsv` is the source of truth for what is done and what is next — it is
what makes a cleared context recoverable. Pipe-delimited: `id|phase|status|branch|title`,
status one of `todo` / `wip` / `done` / `manual` (`manual` = the user's action, e.g.
renaming the GitHub repo, tagging a release).

```
make status            # progress bar, in-flight work, next 3 tasks
make tasks             # the full ledger
make start ID=1.1      # mark wip + create/checkout task/1.1-stack-postgres
make done  ID=1.1      # mark done
make merge             # merge current task branch into main (--no-ff, no push)
make handoff           # handoff prompt for the next context
```

Driven by `scripts/progress.sh`, which the Makefile wraps. Editing `docs/TASKS.tsv`
by hand is fine; keep tasks in phase order.

## Build & check

```
make doctor            # go / docker / compose / daemon / optional tooling
make build             # → bin/spinup
make run ARGS="up postgres"
make test              # go test ./... -race
make test-integration  # tests that need Docker (build tag: integration)
make check             # vet + lint + test + stacks-validate — what CI runs
make stacks-validate   # docker compose config on every stack
```

Targets that need Go degrade to a no-op message until `main.go` exists (Phase 2),
so `make check` is safe to run at any point in the roadmap.

## Layout

```
main.go                  entrypoint
cmd/                     one file per cobra command
internal/
  catalog/               embedded stacks (go:embed) + ~/.spinup/stacks overrides
  compose/               wrapper over `docker compose` (exec, not the SDK)
  docker/                daemon / version / GPU / port checks
  config/                ~/.spinup/config.yaml, per-stack .env
  ui/                    tables, colours, spinners (lipgloss)
stacks/<name>/           the catalog, embedded at build time
docs/PLAN.md             the specification
docs/TASKS.tsv           progress ledger
scripts/progress.sh      ledger tooling
```

## Conventions

**Go.** Module `github.com/DulsaraNethmin/spinup`. Cobra for commands, lipgloss for
output. Shell out to `docker compose` rather than using the Docker SDK — users get
exactly the behaviour they'd get by hand, and every stack stays a plain `compose.yaml`
that works without the tool. Keep `internal/` packages free of `os.Exit` and printing;
commands own the UX. Respect `NO_COLOR`.

Exit codes: `0` ok, `1` usage, `2` docker unavailable, `3` stack not found,
`4` compose failed.

**Stacks.** Every stack is a folder of four files — `compose.yaml`, `.env.example`,
`spinup.yaml` (metadata the CLI reads), `README.md` (shown by `spinup info`). The CLI
is fully generic: adding a stack never means editing Go code. See `docs/PLAN.md` §4
for the `spinup.yaml` schema and the Compose conventions (major-version image tags
never `:latest`, every port/credential env-driven with inline defaults, named volumes,
healthcheck on the primary service, `depends_on: service_healthy` for GUIs, GUIs on
non-colliding 8xxx ports, `gui`/`gpu` profiles).

Host ports are allocated centrally in `docs/PORTS.md` — claim a stack's ports
there before writing its `compose.yaml`, so stacks can all run side by side.

A stack is not done until `docker compose config` passes and it comes up healthy.
`stacks/postgres/` is the reference implementation; copy its shape.

**Runtime data.** Named Docker volumes prefixed `spinup-<stack>_`. Compose projects
are named `spinup-<stack>`. Nothing is ever written into the repo or the install
location; user state lives in `~/.spinup/`.

## Watch out for

- `docker compose` (v2, space) — never `docker-compose` (v1, EOL 2023). The legacy
  scripts in this repo all use v1; they are being deleted, not fixed.
- The old top-level folders (`mysql/`, `postgres/`, `redis/`, …) are legacy. New work
  goes in `stacks/`. Don't patch a legacy script — port it and delete it.
- `docs/PLAN.md` §1 lists every known bug in the legacy Compose files. When porting a
  stack, fix its listed bugs; that table is the acceptance checklist.
- In healthchecks always probe `127.0.0.1`, never `localhost`. In several images
  `localhost` resolves to `::1` first while the service binds IPv4 only, so the
  probe fails against a perfectly healthy container.
- Check a probe's tooling exists before relying on it. BusyBox `wget` (Alpine
  images) has no `--user`/`--password`, and not every image ships `curl`. Test
  the command with `docker exec` before committing the healthcheck.
- A healthcheck only needs to prove liveness. For a GUI behind auth, treat 401
  as healthy rather than embedding credentials in the probe.
- Pin to a tag that exists — `docker manifest inspect <image>:<tag>` before
  writing it into a compose file. PLAN's suggested tags are not all real.
- macOS ships GNU Make 3.81, so the Makefile uses tabs and avoids `.RECIPEPREFIX`.
- `docs/PLAN.md` §7 has open decisions. Settled so far: name is **spinup**; the CLI
  grows **in place** in this repo; junk files are deleted in a normal commit rather
  than purged from history with `git filter-repo`.
