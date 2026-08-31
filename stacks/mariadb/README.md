# mariadb

[MariaDB](https://mariadb.org/) 11.8 LTS with [Adminer](https://www.adminer.org/)
as the optional web GUI.

```
spin up mariadb             # MariaDB on 3307
spin up mariadb --gui       # + Adminer on http://localhost:8097
spin cli mariadb            # the mariadb client, inside the container
spin url mariadb            # mysql://spinup:spinup@localhost:3307/spinup
```

## Ports

| Service | Host port              | Container | Env var                |
|---------|------------------------|-----------|------------------------|
| mariadb | `3307`                 | 3306      | `MARIADB_PORT`         |
| adminer | `8097` (`gui` profile) | 8080      | `MARIADB_ADMINER_PORT` |

`3307`, not the native `3306`: the `mysql` stack has that one, and every stack
in the catalog has to be able to run beside every other. Clients that assume
3306 need the port spelled out.

## Credentials

| What          | Default  |
|---------------|----------|
| Database      | `spinup` |
| User          | `spinup` |
| Password      | `spinup` |
| Root password | `spinup` |

```
mysql://spinup:spinup@localhost:3307/spinup
```

MariaDB speaks the MySQL wire protocol, so MySQL drivers, `mysql` clients and
`mysql://` URLs all work — that is the scheme to use, not `mariadb://`.

## Seeding

Anything in `init/` — `.sql`, `.sql.gz` or `.sh` — runs once, when the data
volume is empty. Nothing runs on later starts; `spin destroy mariadb` is what
gets you back to a first start.

## Storage

`mariadb-data` holds everything, so `spin down mariadb` keeps your data and
`spin destroy mariadb` is the only thing that deletes it.

## Notes

- Adminer is a container of its own, so it lives behind the `gui` profile and
  waits for MariaDB to be healthy before it starts. Inside the stack's network
  the server is `mariadb`, which is what its login form is prefilled with.
- The healthcheck is the image's own `healthcheck.sh --connect
  --innodb_initialized`, which is the difference between a server that answers
  and a server that is ready.
