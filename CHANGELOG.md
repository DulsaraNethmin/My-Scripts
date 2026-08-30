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
- Integration tests in CI: the compose wrapper and the CLI both bring redis up
  against a real daemon and assert the data-safety invariant in both
  directions. The CLI test installs its own copy of the stack under another
  name, so it can never touch a stack the developer is running.
- `spinup list`, `ps`, `logs` and `env`. `list` works without Docker (the status
  column is all that needs it) and `-q` prints bare names for scripting; `env`
  shows a stack's resolved ports and credentials, or opens the file in $EDITOR.
- `spinup up`, `down`, `restart` and `destroy`. `up` materialises the stack,
  seeds its env file, waits for healthy and prints the connection string and
  GUI address; it is idempotent and never removes anything. `down` keeps data,
  `destroy` is the only command that deletes it and asks first — a prompt it
  cannot ask (no terminal) refuses rather than proceeding. Anything after `--`
  goes straight to docker compose.
- `internal/compose`: every invocation carries the same project name
  (`spinup-<stack>`), compose file and env file, so a stack can never collide
  with a project the user made themselves, and compose runs in the stack's own
  directory so relative build contexts and bind mounts resolve. Integration
  tests bring redis up through it and assert that `down` keeps volumes while
  `down --volumes` removes them.
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

### Fixed

- `internal/compose` no longer races when the same writer is handed to it for
  both stdout and stderr. `os/exec` copies the two streams on separate
  goroutines, so a caller collecting everything into one buffer had both of
  them writing to it at once — half the output was silently lost. Found by
  `make test-integration`, which runs with `-race`; the regression test now
  catches it without Docker.

### Added — release pipeline (Phase 3)

- `.goreleaser.yaml` and `.github/workflows/release.yml`: pushing a `v*` tag
  builds darwin/linux/windows × amd64/arm64, publishes a GitHub release with
  `checksums.txt`, and signs that file with cosign keyless — no private key,
  the signature is bound to the workflow's GitHub OIDC identity. The binaries
  carry the same `-s -w -X main.version/commit/date` ldflags `make build`
  uses, and are stamped with the commit time so a rebuild of a tag is
  byte-identical.
- Release archives are the binary plus `LICENSE`, `README.md` and
  `CHANGELOG.md` — deliberately no `stacks/` directory. The catalog is
  compiled into the binary, so shipping it alongside would hand users a
  second copy that goes stale the moment they upgrade. The release workflow
  unpacks every archive and fails if `stacks/` appears in one.
- Running the release workflow manually (`workflow_dispatch`) is a dry run: it
  builds all six targets as a snapshot, checks the archives, runs
  `spinup version` and `spinup list` out of the linux/amd64 build and uploads
  the result as a workflow artefact, without touching the Releases page.
- `make release-check` (`goreleaser check`) and a CI job that runs it on every
  push, so the config cannot rot between tags.
- A tagged release now also updates the two package repositories:
  `DulsaraNethmin/homebrew-tap` gets a Homebrew **cask** and
  `DulsaraNethmin/scoop-bucket` a Scoop manifest, both committed by GoReleaser
  with a PAT (the workflow's own token cannot write to another repository).
  A cask rather than a formula because GoReleaser deprecated formula
  generation — the cost is that casks are macOS-only, so Linux is served by
  the archives and, from 3.3, `install.sh`. The cask strips the quarantine
  attribute macOS puts on an unsigned download, and `brew uninstall --zap`
  removes `~/.spinup`, which a plain uninstall leaves alone.
- `install.sh` and `install.ps1`: a one-line install for macOS, Linux and
  Windows that reads the latest release, checks the archive against the
  release's `checksums.txt` before it installs anything, and puts a single
  binary on the PATH. Both take `--version` and `--dir`, and both refuse to
  install a download whose checksum does not match. `install.sh` is tested
  end to end against a local release server, so what CI runs is the script
  users pipe into `sh`.
- `spinup update`: replaces the running binary with the latest release, after
  the same checksum check. It stops when Homebrew or Scoop owns the binary —
  overwriting a file the package manager tracks is undone by the next upgrade
  — and prints the command to use instead; `--check` only reports, `--force`
  overrides. `SPINUP_REPO` and `SPINUP_API` point it at another repository.
- Shell completions come with a Homebrew install: the cask generates them from
  the binary it just installed, so `spinup up <tab>` completes with nothing to
  source. Elsewhere `spinup completion bash|zsh|fish|powershell` prints them.
- `docs/RELEASING.md`: the one-time setup a release needs — the two
  repositories, the `TAP_GITHUB_TOKEN` secret, the dry run, the tag, and how
  to back a release out before anyone has installed it.
- The first spinup release will be `v1.1.0`, not `v0.1.0`: this repository was
  already tagged `v1.0.0` in its My-Scripts days, and a v0 tag would sort below
  it (PLAN §7.6). Binaries print a `v`-prefixed version so what
  `spinup version` shows is exactly the tag it came from.

[unreleased]: https://github.com/DulsaraNethmin/My-Scripts/commits/main
