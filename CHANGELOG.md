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

### Notes

The Compose files are deliberately kept as-is; they are the input to the Phase 1
stack catalog, where each is rewritten under `stacks/<name>/` with its bugs fixed.

[unreleased]: https://github.com/DulsaraNethmin/My-Scripts/commits/main
