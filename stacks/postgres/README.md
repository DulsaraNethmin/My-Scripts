# postgres

PostgreSQL 16 with [pgAdmin 4](https://www.pgadmin.org/) as the optional web GUI.

```
spinup up postgres            # database only
spinup up postgres --gui      # database + pgAdmin
spinup url postgres           # print the connection string
spinup cli postgres           # psql, inside the container
```

## Ports

| Service  | Host port                | Container | Env var         |
|----------|--------------------------|-----------|-----------------|
| postgres | `5432`                   | 5432      | `POSTGRES_PORT` |
| pgadmin  | `8080` (`gui` profile)   | 80        | `PGADMIN_PORT`  |

pgAdmin sits on 8080, not 80 as in the old script — port 80 collides with nginx
and needs root on Linux.

## Credentials

| What              | Default             |
|-------------------|---------------------|
| Postgres user     | `spinup`            |
| Postgres password | `spinup`            |
| Database          | `spinup`            |
| pgAdmin login     | `admin@example.com`|
| pgAdmin password  | `spinup`            |

```
postgres://spinup:spinup@localhost:5432/spinup
```

These are development defaults on a local machine. Change them in
`spinup env postgres --edit`, and never expose this stack to a network you do
not control.

## Connecting from pgAdmin

pgAdmin runs in its own container, so `localhost` there is not your machine.
Register the server with:

- **Host** `postgres` (the service name, not `localhost`)
- **Port** `5432` (the container port, not `POSTGRES_PORT`)
- **Username / password** as above

## Seeding

Anything in `init/` — `.sql`, `.sql.gz` or `.sh` — is executed once, in
alphabetical order, the first time the data volume is created:

```
stacks/postgres/init/
├── 01-schema.sql
└── 02-seed.sql
```

It will **not** re-run on later starts. To reseed, `spinup destroy postgres`
(which deletes the volume) and `spinup up postgres` again.

## Gotchas

- `POSTGRES_USER`, `POSTGRES_PASSWORD` and `POSTGRES_DB` are only read when the
  data volume is empty. Editing them later changes nothing until you destroy the
  volume — this surprises everyone at least once.
- `spinup down` keeps your data; only `spinup destroy` deletes it.
- The database initialises with `--locale=C` for reproducible sort order across
  hosts. If you need locale-aware collation, set `POSTGRES_INITDB_ARGS` before
  the first start.
- pgAdmin takes noticeably longer to become healthy than Postgres does; it waits
  for Postgres to pass its healthcheck before starting at all.
- `PGADMIN_EMAIL` is validated by pgAdmin, which rejects reserved TLDs like
  `.local` and `.test` and crash-loops on startup if you use one. Stick to a
  real-looking domain.
- Port 5432 or 8080 already taken? That is common if you run other Compose
  projects. Override per machine with `spinup env postgres --edit`, or for one
  run with `spinup up postgres --port POSTGRES_PORT=15432`. Defaults for every
  stack are listed in `docs/PORTS.md`.
