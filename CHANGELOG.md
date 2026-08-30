# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

The repository is being grown in place from `My-Scripts` — a collection of ad-hoc
Compose files and shell scripts — into `spinup`, an installable CLI that starts
local development services with one command. See `docs/PLAN.md` for the design and
`docs/TASKS.tsv` for progress.

### Added

- `CLAUDE.md`, `Makefile` and `scripts/progress.sh` — development workflow and
  phase/task progress tracking.
- `docs/PLAN.md` — the spinup specification: architecture, command surface, stack
  format, repo layout and phased roadmap.
- `.editorconfig`, and a real `.gitignore` covering Go build artefacts, local `.env`
  files, database dumps, terraform state, archives, editor and OS cruft.

### Removed

- `mysql.zip` and the `terraform/` directory, including a 20 MB checked-in Windows
  provider binary and local `.tfstate` files. Terraform is not part of the spinup
  design; recover the config from history with
  `git show 7e6b159:terraform/main.tf` if needed.
- Every legacy start/stop script — `*.sh`, `*.bat` and `run_mysql.desktop` across
  `mongodb/`, `mssql/`, `mysql/`, `nginxServer/`, `postgres/`, `pytorch/` and
  `redis/`. They shell out to `docker-compose` v1 (EOL 2023), several `cd` into a
  file, and `mysql-stop.sh` deleted its data volume on every stop. All are
  superseded by the CLI.
- `redis/Dockerfile`, which nothing referenced.

### Added — stack catalog (Phase 1)

Eight stacks under `stacks/<name>/`, each with `compose.yaml`, `.env.example`,
`spinup.yaml` and a `README.md`: `postgres`, `mysql`, `mongodb`, `redis`,
`mssql`, `pytorch`, `nginx-static` and `nginx-proxy-manager`. Every bug listed
in `docs/PLAN.md` §1 is fixed, and each stack was brought up and verified
healthy rather than only being config-checked.

Applied throughout: pinned image tags instead of `:latest`, healthchecks on
primary services with GUIs waiting on `service_healthy`, named volumes so
`down` never destroys data, env-driven ports and credentials with inline
defaults, and GUIs on non-colliding `80xx` ports recorded in `docs/PORTS.md`.

- `setup/` replaces `docker-setup.sh` with per-platform installers for
  Ubuntu/Debian, Fedora/RHEL and macOS, installing Compose **v2** rather than
  the end-of-life v1 binary and no longer running a full system upgrade.
- `scripts/lint-stacks.sh` and `.github/workflows/ci.yml` — structural lint of
  the catalog, a compose-config matrix that discovers stacks automatically, and
  a smoke matrix that brings the light stacks up and asserts `down -v` really
  removes volumes.

### Added — CLI skeleton (Phase 2)

- Go module `github.com/DulsaraNethmin/spinup`: `main.go`, a Cobra command surface
  under `cmd/`, and the `internal/{catalog,compose,docker,config,ui}` packages.
- The stack catalog is compiled into the binary with `go:embed`, so an installed
  spinup needs no clone and no network to bring a stack up.
- `spinup version` (`--short` for scripts), a global `--no-color`, and the exit
  codes from `docs/PLAN.md` §3 — `1` usage, `2` docker unavailable, `3` stack not
  found, `4` compose failed.
- CI now vets, tests, lints and cross-compiles all six release targets, and
  asserts the embedded catalog matches `stacks/` on disk.
- `spinup.yaml` parsing: the schema is decoded strictly, so an unknown or
  mistyped key is an error rather than being silently ignored, and every rule
  a stack must satisfy is reported in one pass. `internal/catalog` and
  `scripts/lint-stacks.sh` now enforce the same rules.
- `spinup doctor`: the docker CLI, a running daemon and Compose v2, plus
  spinup's own home directory, config and catalog. Compose v1 is reported as a
  failure rather than being accepted — the old scripts in this repo ran on it,
  and it does not support the profiles and healthcheck conditions the stacks
  now use. Exit code 2 means Docker is the problem, 1 means spinup's own
  configuration is.
- `~/.spinup/config.yaml` with `gui` (GUIs come up with their stack by default),
  read strictly so a mistyped key is reported rather than silently doing nothing.
- Per-stack environment resolution: `spinup.yaml` port defaults, then the stack's
  `.env.example`, then `~/.spinup/env/<stack>.env`, then the process environment
  for variables the stack already defines — the same precedence `docker compose`
  applies to `--env-file`, verified against compose itself in an integration test.
- Stacks materialise into `~/.spinup/stacks/<name>/` with their env file seeded
  from `.env.example` — never overwriting a file that is already there, because
  the point of writing them out is that they can be edited. `~/.spinup/stacks`
  shadows the copies embedded in the binary. `SPINUP_HOME` overrides the
  location.

[unreleased]: https://github.com/DulsaraNethmin/My-Scripts/commits/main
