# spinup

Start local development services — databases, queues, GUIs, dev tooling — with
one command, on macOS, Linux and Windows, using Docker Compose underneath.

```
spinup up postgres        # Postgres 16 + pgAdmin, running in ~10s
```

No cloning, no editing YAML, no remembering ports and passwords. **25 stacks**,
each pinned to a real version, healthchecked, and able to run beside every
other one.

> **Status: in development.** The catalog and the CLI are complete; the first
> release is not tagged yet, so the install commands below start working once
> it is. Until then, [build it from source](#from-source) or use the stacks
> directly with `docker compose` — both work, and both will keep working. This
> repository is the successor to `My-Scripts` and is still named that on
> GitHub, so the URLs below will not resolve until it is renamed.
>
> Progress: `make status`. Design: [`docs/PLAN.md`](docs/PLAN.md).

## What it looks like

<!-- TODO(demo): `scripts/demo.sh` drives this sequence at a readable pace;
     record it with asciinema, convert with agg, and replace this comment with
     ![demo](docs/demo.gif). The recipe is in the script's header. -->

```console
$ spinup up postgres --gui
=> postgres
 Container spinup-postgres-postgres-1  Healthy
 Container spinup-postgres-pgadmin-1  Healthy
  services  pgadmin 8080, postgres 5432
  url       postgres://spinup:spinup@localhost:5432/spinup
  gui       http://localhost:8080  (admin@example.com / spinup)
  env       ~/.spinup/env/postgres.env

$ spinup ps
STACK     SERVICE   STATUS         HEALTH   PORTS
postgres  pgadmin   Up 16 seconds  healthy  8080->80
postgres  postgres  Up 22 seconds  healthy  5432->5432

$ spinup url postgres
postgres://spinup:spinup@localhost:5432/spinup

$ spinup down postgres          # stop, keep the data
$ spinup destroy postgres       # stop and delete the data (asks first)
```

Without `--gui` you get the database alone — pgAdmin is a container of its own
and starts only when asked for.

## Install

**macOS** — Homebrew:

```
brew install DulsaraNethmin/tap/spinup
```

**Windows** — Scoop:

```
scoop bucket add DulsaraNethmin https://github.com/DulsaraNethmin/scoop-bucket
scoop install spinup
```

**Linux and macOS** — the install script. It downloads the archive for your
platform, checks it against the release's `checksums.txt`, and installs one
binary:

```
curl -fsSL https://raw.githubusercontent.com/DulsaraNethmin/spinup/main/install.sh | sh
```

**Windows** — PowerShell:

```powershell
irm https://raw.githubusercontent.com/DulsaraNethmin/spinup/main/install.ps1 | iex
```

Then `spinup update` keeps it current — except where Homebrew or Scoop owns the
binary, in which case it says so and prints the right command instead.

### From source

Go 1.25 and Docker with Compose v2:

```
git clone https://github.com/DulsaraNethmin/My-Scripts.git spinup && cd spinup
make build                # -> bin/spinup
./bin/spinup doctor       # check docker, compose and spinup's own setup
```

## Commands

| | |
| --- | --- |
| `up <stack>` | start it and wait for healthy; `--gui`, `--gpu`, `--port NAME=n` |
| `down` / `restart` / `destroy` | stop keeping data / restart / delete data |
| `list` / `ps` / `logs` | the catalog / running containers / logs |
| `info <stack>` | ports, credentials and the stack's README |
| `url <stack>` | the connection string, for pasting into a client |
| `open <stack>` | the web interface, in your browser |
| `shell <stack>` | a shell in the container |
| `cli <stack>` | the stack's own client — `psql`, `mongosh`, `redis-cli`, … |
| `env <stack>` | print its resolved ports and credentials; `--edit` to change them |
| `new <name>` | scaffold a stack of your own |
| `reset <stack>` | restore a built-in stack to the version inside the binary |
| `doctor` | check Docker, Compose, ports and the GPU runtime |

`list` and `ps` take `--json`, for scripting.

Several stacks at once work everywhere a stack is taken:
`spinup up postgres redis mailpit`.

## The catalog

Every stack keeps its service on its well-known port and puts any web GUI in
the `80xx` range, so all 25 can run at the same time. A `gui` in the last
column means the web interface is a container of its own and starts only with
`--gui`; otherwise it is the service itself and is always there.

### Databases

| Stack | What you get | Ports | GUI |
| --- | --- | --- | --- |
| `postgres` | PostgreSQL 16 with pgAdmin | 5432, 8080 | `gui` |
| `mysql` | MySQL 8.4 with phpMyAdmin | 3306, 8081 | `gui` |
| `mariadb` | MariaDB 11.8 LTS with Adminer | 3307, 8097 | `gui` |
| `mssql` | SQL Server 2022 (Developer edition) | 1433 | — |
| `mongodb` | MongoDB 7 with mongo-express | 27017, 8082 | `gui` |
| `redis` | Redis 7 with RedisInsight | 6379, 8083 | `gui` |
| `couchdb` | CouchDB 3, Fauxton on the same port | 5984 | — |
| `neo4j` | Neo4j 5 Community with Neo4j Browser | 7687, 8091 | — |
| `clickhouse` | ClickHouse 26.3 LTS with the Play query UI | 8123, 9001 | — |
| `opensearch` | OpenSearch 3 with OpenSearch Dashboards | 9200, 8094 | `gui` |

### Messaging

| Stack | What you get | Ports | GUI |
| --- | --- | --- | --- |
| `rabbitmq` | RabbitMQ 4 with the management UI | 5672, 8087 | — |
| `kafka` | Kafka 4 in KRaft mode, with kafka-ui | 9092, 8093 | `gui` |
| `nats` | NATS 2 with JetStream and the monitoring endpoints | 4222, 8088 | — |

### Storage and web

| Stack | What you get | Ports | GUI |
| --- | --- | --- | --- |
| `minio` | S3-compatible object storage with a console | 9000, 8086 | — |
| `nginx-static` | nginx serving a static site or SPA build | 8090 | — |
| `nginx-proxy-manager` | Reverse proxy with a UI and Let's Encrypt | 80, 443, 81 | — |
| `traefik` | Traefik v3, routing to containers by label | 8098, 8092 | — |

### Tooling

| Stack | What you get | Ports | GUI |
| --- | --- | --- | --- |
| `adminer` | One PHP page that administers every database you run | 8084 | — |
| `mailpit` | Catches every mail your app sends, with a web inbox | 1025, 8085 | — |
| `portainer` | A web UI for the local Docker daemon | 8089 | — |
| `localstack` | AWS on your machine — S3, SQS, Lambda, DynamoDB | 4566 | — |
| `vault` | HashiCorp Vault in dev mode, with the built-in UI | 8200 | — |
| `keycloak` | Keycloak 26 — OAuth2, OIDC, SAML — on its own Postgres | 8096 | — |
| `monitoring` | Prometheus and Grafana, with a provisioned dashboard | 9090, 8095 | `gui` |

### ML

| Stack | What you get | Ports | GUI |
| --- | --- | --- | --- |
| `pytorch` | PyTorch with JupyterLab, CPU or NVIDIA GPU | 8888, 6006 | — |

Each folder has a `README.md` with its credentials, seeding and gotchas —
`spinup info <stack>` prints it. Ports are allocated centrally in
[`docs/PORTS.md`](docs/PORTS.md).

## Where things live

```
~/.spinup/stacks/<name>/    the stack, written on first use — edit it freely
~/.spinup/env/<name>.env    its ports and credentials — edit these too
~/.spinup/config.yaml       your defaults
```

Neither is ever overwritten once it exists, and anything in `~/.spinup/stacks`
shadows the copy built into the binary — so a stack you change stays changed.
`spinup reset <stack>` puts a built-in one back.

Data lives in named Docker volumes called `spinup-<stack>_*`. Nothing is ever
written into the repository or the install location.

## Using the stacks without the CLI

Every stack is a plain Compose project. That is deliberate and will stay true:
spinup shells out to `docker compose`, so there is nothing it can do that you
cannot do by hand.

```
cd stacks/postgres
docker compose --env-file .env.example up -d                 # database only
docker compose --env-file .env.example --profile gui up -d   # plus the web GUI
docker compose --env-file .env.example down                  # stop, keep data
docker compose --env-file .env.example down -v               # stop and delete data
```

Copy `.env.example` to `.env` in the stack folder to change ports or
credentials.

## Requirements

Docker and Compose v2 (`docker compose`, not the end-of-life `docker-compose`).
If you do not have them, [`setup/`](setup/) has an installer for Ubuntu/Debian,
Fedora/RHEL and macOS.

```
spinup doctor    # or `make doctor` from a source checkout
```

## FAQ

**Is this just a folder of Compose files?**
Underneath, yes — and on purpose. spinup runs `docker compose`, so you get
exactly the behaviour you would get by hand, every stack keeps working if you
stop using the tool, and `docker compose logs` still does what you expect. What
it adds is not having to find the file, remember the ports, or invent a
password.

**Where is my data, and what deletes it?**
In a named volume, `spinup-<stack>_<volume>`. `down` stops containers and keeps
it. `destroy` is the only command that deletes it, and it asks first.

**Can I run several stacks at once?**
That is the point of the port table. Every stack keeps its native port where it
is free and puts its GUI in the `80xx` range, and where two want the same number
the newer one moves — `mariadb` is on 3307, `clickhouse`'s native protocol on
9001. `spinup doctor` reports collisions with things already on your machine,
and `--port NAME=n` overrides one for a single run.

**How do I change a port or a password?**
`spinup env <stack>` prints what a stack will start with; `spinup env <stack>
--edit` opens `~/.spinup/env/<stack>.env` in `$EDITOR`. Most credentials are
applied when a database first initialises, so changing one afterwards needs a
`spinup destroy` first — each stack's README says which of its settings are
like that.

**Can I add my own stack?**
`spinup new <name>` scaffolds one, and `~/.spinup/stacks/<name>/` is where it
goes. The CLI never needs a code change to learn about it. If it is something
other people would want, see below.

**Something is unhealthy. Where do I look?**
`spinup logs <stack>` and `spinup ps`. Almost every first-run failure is one of
three things: a port already taken (`spinup doctor`), a credential changed after
the volume was created (`spinup destroy`), or not enough memory given to Docker
Desktop for a JVM stack like `opensearch` or `kafka`.

**Does it need root, or change anything outside `~/.spinup`?**
No. It shells out to `docker`, writes to `~/.spinup`, and nothing else.
`portainer`, `traefik`, `localstack` and `monitoring` mount the Docker socket
because that is what they are for; their READMEs say so plainly.

**Windows?**
Docker Desktop with the WSL 2 backend. Scoop or `install.ps1` for the binary.
Where a stack mounts the Docker socket, set its `*_DOCKER_SOCK` variable to
`//./pipe/docker_engine`.

## Contributing

The most useful contribution is a stack: four files —
`compose.yaml`, `.env.example`, `spinup.yaml` and `README.md` — and no Go code.
`stacks/postgres/` is the reference to copy.

[**CONTRIBUTING.md**](CONTRIBUTING.md) has the whole of it: claiming ports, the
three healthcheck mistakes everyone makes, when a GUI belongs behind the `gui`
profile and when it does not, and what a stack has to do before it is done.

```
make stacks-lint       # the structural rules
make stacks-validate   # docker compose config on every stack
make check             # what CI runs
```

CI additionally brings the light stacks up and waits for them to report
healthy. A stack is not done until it does.

## Licence

[MIT](LICENSE)
