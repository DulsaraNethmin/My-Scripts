# Contributing to spinup

The most useful contribution is a **stack**: a service someone would otherwise
spend an afternoon wiring up. A stack is four files and no Go code, so adding
one is a self-contained pull request that the CLI learns about for free.

Bug reports and CLI work are welcome too — see [Working on the CLI](#working-on-the-cli).

## Setting up

Go 1.25, Docker with Compose v2, and GNU Make:

```
make doctor            # checks go, docker, compose, the daemon and optional tooling
make build             # -> bin/spinup
make check             # everything CI runs: vet, lint, tests, stack lint, compose config
```

`make lint` needs [golangci-lint **v2**](https://golangci-lint.run/) — the
config is in the v2 format and v1 cannot read it. It is skipped when the binary
is not installed; CI runs it either way.

## Adding a stack

### 1. Claim your ports

[`docs/PORTS.md`](docs/PORTS.md) is the port registry, and it exists so that
every stack can run at the same time as every other. Add your rows there
**before** writing the compose file.

- Keep the service's well-known port when it is free — `5432`, `6379`, `9092`.
  Muscle memory and default connection strings should work.
- When it is taken, move and say why in a comment: `mariadb` is on `3307`
  because `mysql` has `3306`.
- Web GUIs go in the `80xx` range, never on `80`.

### 2. Write the four files

```
stacks/<name>/
├── compose.yaml     the Compose project
├── .env.example     every tunable with its default and a comment
├── spinup.yaml      the metadata the CLI reads
└── README.md        what `spin info <name>` prints
```

`stacks/postgres/` is the reference to copy, and
[`docs/PLAN.md` §4](docs/PLAN.md) has the schema. In short:

- **Images pinned to a real tag**, never `:latest`. Check it exists before you
  write it — `docker manifest inspect <image>:<tag>`. Suggested tags in issues
  and docs are often not real ones.
- **Every port and credential from the environment with an inline default**:
  `"${THING_PORT:-1234}:1234"`. Anything in `.env.example` must be used in
  `compose.yaml` and the other way round; the linter checks both directions.
- **A named volume for data**, so `down` keeps it and only `destroy` removes
  it. A stack with genuinely nothing to keep — `vault` in dev mode — declares
  no volumes, and says so in its README.
- **A healthcheck on the primary service** that proves the service answers,
  not merely that the port is open.
- `restart: unless-stopped`, no `container_name:`, no top-level `version:`.

### 3. Healthchecks: the three that catch everyone

1. **Probe `127.0.0.1`, never `localhost`.** In several images `localhost`
   resolves to `::1` first while the service listens on IPv4 only, and the
   probe fails against a perfectly healthy container.
2. **Check the tooling exists before relying on it.** Not every image has
   `curl`; not every one has `wget`. BusyBox `wget` has no `--user`. Prometheus
   is distroless and has no shell at all. Run the image and
   `docker exec` your probe before committing it — `couchdb` ships curl and no
   wget, `neo4j` the reverse, `keycloak` neither and the probe is bash's
   `/dev/tcp`.
3. **Liveness is enough.** For a GUI behind authentication, treat a 401 as
   healthy rather than putting credentials in the probe.

### 4. The GUI rule

A GUI that is a **container of its own** goes behind the `gui` profile, with
`profiles: [gui]` in both files and `depends_on: condition: service_healthy` on
the primary — that is what makes it optional.

A GUI **served by the primary service itself** — Fauxton, Neo4j Browser,
Portainer, Vault's UI, ClickHouse's Play — has nothing to gate, and correctly
declares no profile. Fourteen of the twenty-five stacks are like that against
eight that are not, so a rule derived from `stacks/postgres` alone rejects most
of the catalog.

### 5. Setup that the image will not do itself

Some services need one call after they start — CouchDB has no `_users`
database, Keycloak's master realm refuses plain HTTP, Portainer wants its admin
password in a file before it starts. The pattern is a one-shot service with
`restart: "no"` and `depends_on: condition: service_healthy` (or
`service_completed_successfully` for one that must run first). Make it
idempotent: it runs on every start. `docker compose up --wait` treats a
container that exits 0 as satisfied, so this does not break `spin up`.

### 6. Check it

```
make stacks-lint       # the structural rules
make stacks-validate   # docker compose config on every stack
make build && ./bin/spin up <name>
```

The catalog is compiled into the binary, so **`make build` after every change
under `stacks/`** — otherwise you are testing the old copy. If the stack is
already materialised in `~/.spinup/stacks`, `spin reset <name>` picks up the
new one.

Test in a scratch home so nothing of yours is touched:

```
export SPINUP_HOME=$(mktemp -d)
./bin/spin up <name>
./bin/spin destroy <name> -y
```

A stack is not done until it comes up healthy, does the thing it is for — index
a document, publish a message, log in — and `destroy` leaves no containers or
volumes behind.

### 7. Add it to CI

Append it to the `smoke` matrix in `.github/workflows/ci.yml`, with
`profiles: "--profile gui"` if it has a `gui` profile. CI brings it up and
waits for healthy, then checks that `down` keeps the volumes and `down -v`
removes them. Very large or very slow images are left out — `mssql` and
`pytorch` are — with a comment saying so.

### 8. Write the README

It is what `spin info <name>` prints, so write it for someone who has just
run the stack and wants to use it: ports, credentials, how to connect, how to
seed it, what `destroy` deletes, and any gotcha you hit while building it.

The gotchas are the valuable part. Say *why* the port moved, why the tag is
that one, why the healthcheck is unusual. Every one of those in the existing
stacks is there because it cost someone an hour.

## Working on the CLI

```
main.go                  entrypoint; owns the go:embed of stacks/
cmd/                     one file per cobra command; owns all output
internal/catalog/        spinup.yaml parsing, ~/.spinup materialisation
internal/compose/        wrapper over `docker compose` (exec, not the SDK)
internal/docker/         daemon, version, GPU and port checks
internal/config/         config.yaml and the per-stack env merge
internal/ui/             tables, colours, spinners
```

Two rules that shape the rest:

- **Shell out to `docker compose`, never the Docker SDK.** Users get exactly
  the behaviour they would get by hand, and every stack stays a plain Compose
  project that works without the tool.
- **`internal/` never prints and never calls `os.Exit`.** Commands own the UX.
  Respect `NO_COLOR`.

Exit codes are part of the interface: `0` ok, `1` usage, `2` Docker
unavailable, `3` stack not found, `4` compose failed.

`internal/catalog` and `scripts/lint-stacks.sh` enforce the same rules about
`spinup.yaml`. Change one and change the other, or a stack can pass CI and fail
in the CLI.

Tests that need Docker are behind the `integration` build tag
(`make test-integration`).

## Pull requests

- One stack, or one change, per pull request.
- `make check` passes.
- The commit message says *why*, not only what. The interesting half of most
  of these changes is the constraint that forced them.
- Add a line to `CHANGELOG.md` under `[Unreleased]`.

## Reporting a bug

`spin doctor` and `spin version` first — most first-run problems are a
port already taken, a credential changed after the volume was created, or
Compose v1. Then open an issue with the output of both, the exact command, and
what `spin logs <stack>` says.
