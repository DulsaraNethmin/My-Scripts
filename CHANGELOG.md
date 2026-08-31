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

### Added — catalog v2 (Phase 5)

- `mailpit` — an SMTP server that catches every message an app sends and shows
  it in a web inbox on `8085`, so nothing can reach a real person. It accepts
  any username and password over an unencrypted connection, because a mail
  catcher that refuses the frameworks that insist on auth is one you spend an
  afternoon on, and it keeps its inbox in a volume rather than in memory,
  which is what the image does by default.
- `minio` — S3-compatible object storage on `9000` with its console on `8086`.
  The root password is `spinup-secret` rather than the catalog's usual
  `spinup`: MinIO refuses to start with fewer than eight characters. `spinup
  cli minio` runs `mc ls local` inside the container, which needed the alias
  the image ships to be given credentials — it is anonymous by default, enough
  for the healthcheck and not enough to list a bucket.
- `adminer` — one PHP page that administers every database you run, on `8084`.
  Its login form starts on `host.docker.internal`, with the `host-gateway`
  entry that makes that name resolve on Linux too, so it reaches the databases
  spinup publishes on your machine rather than looking inside its own
  container.

All three serve their web interface from the primary container, so none has a
`gui` profile, and all three are in the CI smoke matrix.

### Added — connect commands (Phase 4)

- `spinup shell <stack> [service]` opens a shell in a running container —
  bash when the image has it, sh otherwise — and `spinup cli <stack>` runs the
  stack's own client (psql, mysql, mongosh, redis-cli) already pointed at the
  database and authenticated. Both hand the terminal straight to the
  container: `internal/compose` grew an `Attach` that lets the child inherit
  stdin, stdout and stderr, because a copy loop is not a TTY and psql behind
  one cannot read a password, size its window or run a pager.
- The `cli` template is split into arguments *before* its `${VARS}` are
  expanded, so a password containing a space stays one argument and one
  containing a quote or a semicolon cannot become a shell injection — no shell
  is involved at any point. Only the client's name is echoed, never the
  expanded command, which carries the password.
- `spinup url <stack>` prints the connection string, `--gui` the web
  interface's address; `spinup open <stack>` launches that address in the
  browser with the login beside it (`--print` for a headless machine).
  `spinup info <stack>` is the stack's page: what it is, its ports,
  credentials and addresses, then its README.
- `spinup doctor` now checks the two things that stop a stack from starting
  after Docker itself is fine: whether anything already holds the host ports
  the catalog wants — probed by connecting, because binding gets it wrong in
  both directions on a developer machine (a port under 1024 fails with
  "permission denied" and looks taken, and Docker Desktop's published ports do
  not stop a second bind and look free) — and whether two stacks claim the
  same port, which `docs/PORTS.md` prevents for the built-in catalog but
  cannot for a stack of your own.
- doctor also reports the NVIDIA container runtime, warning only in the case
  worth a warning: a machine with an NVIDIA driver that docker cannot use.
  A machine with no GPU is not a problem and is not reported as one. And it
  checks that `docker compose up` has `--wait-timeout` by asking the CLI
  rather than comparing version numbers, since every `spinup up` passes it.
- doctor's closing line acknowledges warnings instead of saying "everything
  checks out" over three of them.
- `spinup up --port NAME=1234` moves a host port for one run without editing
  anything, for the common case of a port already being taken. Compose gives
  the process environment precedence over `--env-file`, which is what makes it
  possible; a name the stack does not declare is refused rather than passed
  through, because compose would accept it silently and bind the port the user
  was trying to change.
- The summary after `up` now names what actually came up — "services  postgres
  5432, pgadmin 8080" — read back from compose rather than from the stack's
  metadata, so an overridden port shows the port that was bound and
  `up --no-gui` no longer advertises a GUI that is not running.
- `--json` on `list` and `ps`, for scripts. `list --json` carries the resolved
  ports and connection string rather than the defaults; `ps --json` carries
  compose's own view with the duplicate IPv4/IPv6 publishers collapsed, and
  answers `[]` rather than prose when nothing is running.
- `spinup new <name>` scaffolds a stack of your own into
  `~/.spinup/stacks/<name>/` — and what it writes runs as it stands: an nginx
  that comes up healthy on the first `spinup up`, carrying every convention a
  stack has to follow (a pinned tag, env-driven ports with inline defaults, a
  healthcheck against 127.0.0.1, a named volume) as something to copy rather
  than a comment to read. `--from <stack>` starts from a copy of an existing
  stack instead, with the name in its `spinup.yaml` rewritten so the copy
  loads, and a warning that it inherits the original's ports.
- `spinup reset <stack>` puts back the copy of a built-in stack compiled into
  the binary, for when an edit has broken it. It asks first, keeps the env
  file with its edited ports and passwords unless `--env` says otherwise, and
  never touches a data volume. A stack of your own is refused rather than
  reset: there is no other copy to restore it from, so it would be a delete.
- `spinup up <tab>` completes stack names, in every command that takes one.
  Cobra's `completion` command generates the scripts; what makes them useful
  is that the names come from the live catalog, the user's own stacks
  included, and that a name already on the line is not offered twice.
- A command that needs a running stack says so — "redis is not running —
  start it with `spinup up redis`" — rather than passing compose's own
  "service is not running" through.

[unreleased]: https://github.com/DulsaraNethmin/My-Scripts/commits/main
