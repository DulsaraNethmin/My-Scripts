# spinup

Start local development services — databases, queues, GUIs, dev tooling — with
one command, on macOS, Linux and Windows, using Docker Compose underneath.

```
spinup up postgres        # Postgres 16 + pgAdmin, running in ~10s
```

No cloning, no editing YAML, no remembering ports and passwords.

> **Status: in development.** The stack catalog is complete, and the CLI now
> runs the whole lifecycle. There is no release to install yet — that is the
> next phase — so build it yourself, or use the stacks directly with
> `docker compose`. Both work, and both will keep working. This repository is
> the successor to `My-Scripts` and is still named that on GitHub.
>
> Progress: `make status`. Design: [`docs/PLAN.md`](docs/PLAN.md).

## Building the CLI

Go 1.25 and Docker with Compose v2:

```
make build                # -> bin/spinup
./bin/spinup doctor       # check docker, compose and spinup's own setup
./bin/spinup list         # the catalog, with ports and what is running
./bin/spinup up postgres  # start it and wait for healthy
```

`up` writes the stack to `~/.spinup/stacks/<name>/` and its ports and
credentials to `~/.spinup/env/<name>.env`, where you can edit them — neither is
ever overwritten once it exists. Anything in `~/.spinup/stacks` shadows the copy
built into the binary, so a stack you change stays changed.

`down` stops a stack and keeps its data; `destroy` is the only command that
deletes it, and it asks first.

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

## The catalog

| Stack                 | What you get                              | Ports                  |
|-----------------------|-------------------------------------------|------------------------|
| `postgres`            | PostgreSQL 16 + pgAdmin                   | 5432, 8080             |
| `mysql`               | MySQL 8.4 + phpMyAdmin                    | 3306, 8081             |
| `mongodb`             | MongoDB 7 + mongo-express                 | 27017, 8082            |
| `redis`               | Redis 7 + RedisInsight                    | 6379, 8083             |
| `mssql`               | SQL Server 2022 (Developer)               | 1433                   |
| `pytorch`             | PyTorch + JupyterLab, CPU or NVIDIA GPU   | 8888, 6006             |
| `nginx-static`        | nginx serving a static site or SPA build  | 8090                   |
| `nginx-proxy-manager` | Reverse proxy + Let's Encrypt, with a UI  | 80, 443, 81            |

Each folder has a `README.md` with its credentials, seeding and gotchas.
GUI ports are allocated centrally in [`docs/PORTS.md`](docs/PORTS.md) so every
stack can run at the same time.

Every stack is pinned to a real version tag, has a healthcheck, keeps its data
in a named volume, and drives every port and credential from `.env` — so
`down` never destroys your data and two stacks never fight over a port.

## Requirements

Docker and Compose v2 (`docker compose`, not the end-of-life `docker-compose`).
If you do not have them, [`setup/`](setup/) has an installer for Ubuntu/Debian,
Fedora/RHEL and macOS.

```
make doctor      # check your toolchain
```

## Contributing a stack

A stack is four files — `compose.yaml`, `.env.example`, `spinup.yaml` and
`README.md` — and needs no Go code. `stacks/postgres/` is the reference to copy.
`docs/PLAN.md` §4 has the conventions; `make stacks-lint` and
`make stacks-validate` enforce them, and CI additionally brings the light stacks
up and waits for them to report healthy.

## Licence

[MIT](LICENSE)
