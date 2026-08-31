# spinup — project plan

`spinup` is the successor to `DulsaraNethmin/My-Scripts`: an installable CLI that starts local
development services (databases, queues, GUIs, dev tooling) with one command, on macOS, Linux and
Windows, using Docker Compose under the hood.

The goal in one line:

```
brew install DulsaraNethmin/tap/spinup     # or curl | sh, or scoop
spin up postgres                          # → Postgres 16 + pgAdmin, running in ~10 s
```

No cloning, no editing YAML, no remembering ports and passwords.

---

## 1. Where we're starting from (audit of My-Scripts)

Ten folders of Compose files and helper scripts, a README covering two of them, and several things
that don't work.

### Bugs that break for anyone who tries the scripts

| Where | Problem |
|---|---|
| `mysql/*.sh`, `mssql/*.sh`, `postgres/postgres-start.sh`, `pytorch/*.sh` | `cd ./docker-compose.yml` — cd into a *file*; the script errors or runs compose in whatever directory the user is in. |
| `mysql/mysql-stop.sh` | `docker-compose down -v` deletes the data volume on every stop. |
| `mysql/docker-compose.yml` | phpMyAdmin password `11111111` ≠ MySQL root `root`, so the GUI can't log in. `MYSQL_ROOT` isn't a real env var. `--default-authentication-plugin` is removed in MySQL 8.4+. |
| `mysql/mysql-run.bat` | Hard-codes `C:\Users\Nethmin\Documents\scripts\…`. |
| `nginx-proxy-manager/docker-compsoe.yml` | Misspelled filename and a pasted `GNU nano 7.2 …` header line → invalid YAML. |
| `postgres/docker-compose.yaml` | pgAdmin on host port 80: collides with nginx and needs root. |
| `pytorch/docker-compose.yaml` | `runtime: nvidia` is mandatory → fails on every machine without the NVIDIA toolkit. Jupyter with no token on `0.0.0.0`. Unused `workspace` volume. |
| `redis/docker-compose.yaml` | No data volume; `redisinsight:1.13.0` is the discontinued v1; `redis/Dockerfile` is unused. |
| `mssql/docker-compose.yml` | `SA_PASSWORD` deprecated (→ `MSSQL_SA_PASSWORD`); 2019 image; no healthcheck. |
| `nginxServer/run_server.sh` | Depends on a personal Docker Hub image instead of building the adjacent Dockerfile. |
| `docker-setup.sh` | Ubuntu-only, installs the EOL `docker-compose` v1 binary, runs a full `apt-get upgrade`. |
| all `*.sh` | Use `docker-compose` (v1, EOL 2023) rather than `docker compose`. |

### Things that shouldn't be in a public repo

`terraform/.terraform/**` (a 20 MB Windows provider `.exe`), `terraform/terraform.tfstate` +
`.backup`, `mysql.zip`, `mysql/run_mysql.desktop`. `.gitignore` covers only `*.sql` in two folders.

### Consistency problems

Mixed `yml`/`yaml`, five different start/stop script naming schemes, camelCase vs kebab-case
folders, hard-coded credentials and ports that differ per stack, obsolete `version:` keys,
`:latest` tags, no healthchecks, no docs beyond MySQL-on-Windows.

The Compose files themselves are the valuable part — they become the built-in stack catalog of
`spinup`. Everything else (start/stop scripts, `.bat`, `.desktop`, setup script) is replaced by
the CLI.

---

## 2. Architecture

```
┌─────────────────────────────┐
│  spinup (single Go binary)  │
│                             │
│  cmd/      cobra commands   │
│  internal/                  │
│    catalog/  embedded stacks (go:embed) + ~/.spinup/stacks overrides
│    compose/  wrapper over `docker compose` (exec, not SDK)
│    docker/   daemon/version/GPU/port checks
│    config/   ~/.spinup/config.yaml, per-stack .env
│    ui/       tables, colours, spinners (lipgloss)
└──────────────┬──────────────┘
               │ shells out to
        docker compose -p spinup-<stack> -f <stack>/compose.yaml --env-file ~/.spinup/env/<stack>.env
```

Key decisions:

* **Go**, Cobra for commands, Charm's lipgloss/bubbles for output. Single static binary,
  ~10 MB, no runtime deps beyond Docker itself.
* **Shell out to `docker compose`** rather than using the Docker SDK. Users get the exact
  behaviour they'd get by hand, `spinup` stays small, and every stack is still a plain
  `compose.yaml` anyone can copy and use without the tool.
* **Stacks are embedded** with `go:embed stacks/**`. On first `up`, the stack's files are
  materialised into `~/.spinup/stacks/<name>/` (so users can see and tweak them) and a
  `~/.spinup/env/<name>.env` is created from the stack's `.env.example`.
* **User overrides**: anything in `~/.spinup/stacks/<name>/` wins over the embedded copy;
  `spin new <name>` scaffolds a user stack there; `spin reset <name>` restores the
  embedded version.
* **Compose project naming**: `spinup-<stack>`, so `docker ps` and Docker Desktop show where
  containers came from and stacks never collide with the user's own projects.
* **Data** lives in named Docker volumes prefixed `spinup-<stack>_…`; nothing is written into the
  install location or the repo.
* **Windows** is first-class: native binary, no WSL required (Docker Desktop provides
  `docker compose`). Path handling and `open` use OS-specific code.

---

## 3. Command surface

```
spin up <stack>... [--gui] [--gpu] [--build] [--port name=1234]
spin down <stack>...              stop, keep data
spin restart <stack>...
spin destroy <stack>... [-y]      stop AND delete volumes (confirms)
spin list                         all stacks: name, description, status, ports
spin ps [stack]                   containers with health, ports, uptime
spin logs <stack> [-f] [service]
spin shell <stack> [service]      exec into the primary container
spin cli <stack>                  open the native client (psql / mysql / mongosh / redis-cli)
spin open <stack>                 open the stack's GUI URL in the browser
spin env <stack> [--edit]         show or edit ports/credentials for the stack
spin url <stack>                  print the connection string (postgres://user:pass@localhost:5432/db)
spin info <stack>                 README of the stack: what it is, ports, creds, gotchas
spin doctor                       docker daemon, compose v2, port collisions, disk, GPU
spin new <name>                   scaffold a user stack in ~/.spinup/stacks/<name>
spin reset <name>                 restore a built-in stack to its embedded version
spin update                       self-update the binary (GitHub Releases)
spin completion <shell>           bash/zsh/fish/powershell completions
spin version
```

Behavioural rules:

* `up` is idempotent and never removes anything; `destroy` is the only command that deletes
  volumes and it asks first (skip with `-y`).
* `--gui` and `--gpu` map to Compose **profiles** in the stack (`gui`, `gpu`). Whether GUIs are on
  by default is a config option (`spinup config set gui true`).
* `--port` overrides a host port for this invocation without editing the env file; a persistent
  change goes through `spin env <stack> --edit`.
* Anything after `--` is passed straight to `docker compose`, so power users are never blocked.
* Output is colourised with `NO_COLOR` respected; `--json` on `list`/`ps` for scripting.
* Exit codes: 0 ok, 1 usage, 2 docker not available, 3 stack not found, 4 compose failed.
* After `up`, print a compact card: services, host ports, GUI URL + login, and the
  connection string — the info people otherwise go digging for.

---

## 4. Stack format

Each stack is a folder with three files; the CLI is fully generic and never needs editing to add one.

```
stacks/postgres/
├── compose.yaml        # Compose v2, no version: key, env-driven, profiles for gui/gpu
├── .env.example        # every tunable with its default, one comment each
├── spinup.yaml         # metadata read by the CLI
└── README.md           # shown by `spin info postgres`
```

`spinup.yaml`:

```yaml
name: postgres
description: PostgreSQL 16 with pgAdmin
category: database            # database | messaging | storage | tooling | ml | web
primary: postgres             # service used by shell/cli/logs default
cli: psql -U ${POSTGRES_USER} ${POSTGRES_DB}
gui:
  service: pgadmin
  url: http://localhost:${PGADMIN_PORT}
  login: ${PGADMIN_EMAIL} / ${PGADMIN_PASSWORD}
url: postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}
ports:
  - name: POSTGRES_PORT
    default: 5432
  - name: PGADMIN_PORT
    default: 8080
profiles: [gui]

# Optional, for a stack whose services all sit behind profiles (stacks/pytorch,
# where the CPU and GPU services share ports so neither can start by default):
default_profiles: [cpu]       # what `up` selects when the user selects nothing
gpu:                          # what `up --gpu` swaps in
  profile: gpu
  service: jupyter-gpu
```

`name`, `description`, `category`, `primary`, `url` and at least one port are required;
`name` must match the directory. A `gui` whose service is a *separate* container must be
behind the `gui` profile — that is what makes it optional. A `gui` served by the primary
service itself (nginx, Nginx Proxy Manager, JupyterLab) has nothing to gate and declares
no profile. `internal/catalog` and `scripts/lint-stacks.sh` enforce the same rules, and
unknown keys are an error rather than being ignored.

Compose conventions (applied to every stack):

1. Services named after the product; images pinned to a **major** tag (`postgres:16`,
   `redis:7-alpine`, `mongo:7`), never `:latest`.
2. Every port/password/db name from env with inline defaults:
   `"${POSTGRES_PORT:-5432}:5432"`.
3. Named volumes for data; `healthcheck` on the primary service;
   `depends_on: condition: service_healthy` for GUIs; `restart: unless-stopped`.
4. GUIs on non-colliding 8xxx ports so many stacks run side by side (port table in README).
5. Optional `init/` mounted to the image's `docker-entrypoint-initdb.d` for seed data.
6. `README.md` per stack: purpose, ports, creds, GUI, seeding, gotchas.

---

## 5. Repo layout

```
spinup/
├── main.go
├── cmd/                     # one file per command
├── internal/{catalog,compose,docker,config,ui}/
├── stacks/                  # the catalog (embedded at build time)
│   ├── postgres/  mysql/  mariadb/  mssql/  mongodb/  redis/
│   ├── opensearch/  clickhouse/  neo4j/  couchdb/
│   ├── rabbitmq/  kafka/  nats/  minio/  mailpit/  localstack/
│   ├── portainer/  traefik/  monitoring/  keycloak/  vault/  adminer/
│   ├── nginx-static/  nginx-proxy-manager/
│   └── pytorch/
├── install.sh               # curl | sh installer (detects OS/arch, fetches latest release)
├── install.ps1              # Windows equivalent
├── .goreleaser.yaml         # binaries, checksums, Homebrew tap, Scoop bucket, winget manifest
├── .github/workflows/
│   ├── ci.yml               # go vet/test, golangci-lint, `docker compose config` for every stack,
│   │                        # and a smoke matrix that `up`s the light stacks and waits for healthy
│   └── release.yml          # on tag → GoReleaser
├── docs/                    # optional later: mkdocs site with one page per stack
├── setup/                   # docker-ubuntu.sh, docker-fedora.sh, docker-macos.sh (from docker-setup.sh)
├── CONTRIBUTING.md  README.md  LICENSE  CHANGELOG.md
└── Makefile                 # build / test / lint / snapshot-release
```

Two small repos of your own that GoReleaser commits into (it does not create
them): `DulsaraNethmin/homebrew-tap` and `DulsaraNethmin/scoop-bucket`. Homebrew
gets a **cask**, not a formula — GoReleaser deprecated formula generation, and
casks are macOS-only, so Linux is served by the archives and `install.sh`.
`docs/RELEASING.md` has the one-time setup.

---

## 6. Phased roadmap

### Phase 0 — Repo hygiene + rename (½ day)

* Rename `My-Scripts` → `spinup` on GitHub (old links redirect); set description + topics
  (`docker`, `docker-compose`, `local-development`, `cli`, `devtools`, `golang`).
* Delete `mysql.zip`, `terraform/.terraform/`, `*.tfstate*`, `run_mysql.desktop`, `redis/Dockerfile`,
  all `.bat`/`.sh` start-stop scripts (superseded by the CLI). Purge the 20 MB binary from
  history with `git filter-repo` (one-time force push, noted in README).
* Real `.gitignore`, `.editorconfig`, `CHANGELOG.md` started.

### Phase 1 — Catalog v1: standardize the existing stacks (1 day)

* Move to `stacks/<name>/`, kebab-case, `compose.yaml`, add `spinup.yaml`, `.env.example`,
  `README.md`; apply §4 conventions; fix every bug in §1.
* Image updates: `mysql:8.4`, `postgres:16`, `mongo:7` (+ mongo-express), `redis:7-alpine` +
  `redis/redisinsight:2`, `mcr.microsoft.com/mssql/server:2022-latest`, pytorch GPU behind a
  `gpu` profile with Jupyter token enabled.
* nginx-static builds locally from its Dockerfile; nginx-proxy-manager fixed and documented.
* CI job: `docker compose config` on every stack.

### Phase 2 — CLI core (2 days)

* Go module, Cobra skeleton, `internal/` packages, `go:embed` of `stacks/`.
* Commands: `up`, `down`, `restart`, `destroy`, `list`, `ps`, `logs`, `env`, `doctor`, `version`.
* Materialise-on-first-use into `~/.spinup/`, env-file handling, project naming, profiles.
* Unit tests for catalog parsing and env merging; integration test that ups `redis` in CI.

### Phase 3 — Distribution (1 day)

* GoReleaser: darwin/linux/windows × amd64/arm64, checksums, cosign signatures.
* Homebrew tap + Scoop bucket auto-updated by the release workflow; winget manifest PR script.
* `install.sh` / `install.ps1`; `spin update` self-updater; shell completions.
* First tagged release `v1.1.0` with the Phase-1 catalog (§7.6 explains the number).
  README hero switches to the install one-liner.

### Phase 4 — CLI polish (1 day)

* `shell`, `cli`, `open`, `url`, `info`, `new`, `reset`, `completion`, `--json`, `--port`.
* Post-`up` summary card; `doctor` checks port collisions, GPU runtime, Compose version.

### Phase 5 — Catalog v2: new stacks (2–3 days, one PR each)

Order by usefulness-per-effort: `mailpit`, `minio`, `adminer`, `rabbitmq`, `mariadb`, `nats`,
`portainer`, `couchdb`, `neo4j`, `clickhouse`, `localstack`, `vault`, `traefik`, `kafka`
(KRaft + kafka-ui), `opensearch` (+ Dashboards), `monitoring` (Prometheus + Grafana +
node-exporter + cAdvisor with a provisioned Docker dashboard), `keycloak` (Postgres-backed).

Each new stack must pass `docker compose config`, come up healthy in the CI smoke matrix, and
ship a README before merging.

### Phase 6 — Docs & community (1 day)

* README: install one-liner, 30-second demo GIF (`vhs`), stack table, platform notes, FAQ.
* `CONTRIBUTING.md` with the stack checklist; issue templates ("request a stack", "bug").
* Optional: `docs/` mkdocs site on GitHub Pages, one page per stack, auto-generated from
  `spinup.yaml` + README.

---

## 7. Decisions still open

1. **Rename timing** — rename the GitHub repo in Phase 0 (recommended; redirects keep your pinned
   link working) or wait until v0.1.0 ships.
2. **GUIs on by default?** Plan says *on* (`gui` profile enabled unless `--no-gui`), because the
   GUI is half the reason to use a stack; flip to opt-in if you'd rather keep `up` lean.
3. **Elasticsearch vs OpenSearch** — plan uses OpenSearch (Apache-2.0, no licence env var).
4. **Traefik integration** — opt-in shared `spinup` network so `http://pgadmin.localhost` works
   when traefik is running; stacks join it only if `spinup config set traefik true`.
5. **Go module path** — `github.com/DulsaraNethmin/spinup` (public repo name must match).
   *Settled in task 2.1: the module is `github.com/DulsaraNethmin/spinup`.*
6. **First release number** — the repo already carries a `v1.0.0` tag from its My-Scripts
   days, so `git describe` reports `v1.0.0-N-g<sha>` and a v0.1.0 release would sort
   *below* a tag that is already published.
   *Settled in task 3.1: the old tag stays where it is and spinup's releases start at
   `v1.1.0`. Nothing to delete, no link to break; the cost is that 1.x arrives before
   the command surface is finished, so Phases 4–5 land as minor versions.*
