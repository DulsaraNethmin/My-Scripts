# spinup

Start local development services — databases, queues, GUIs, dev tooling — with
one command, on macOS, Linux and Windows, using Docker Compose underneath.

```
spinup up postgres        # Postgres 16 + pgAdmin, running in ~10s
```

No cloning, no editing YAML, no remembering ports and passwords.

> **Status: in development.** The stack catalog is complete and working; the
> `spinup` CLI that wraps it is Phase 2 and does not exist yet. Until it does,
> use the stacks directly with `docker compose` — see below. This repository is
> the successor to `My-Scripts` and is still named that on GitHub.
>
> Progress: `make status`. Design: [`docs/PLAN.md`](docs/PLAN.md).

## Using the stacks today

Every stack is a plain Compose project that works without the CLI. That is
deliberate and will stay true.

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
