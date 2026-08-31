# adminer

[Adminer](https://www.adminer.org/) is a database front end in a single PHP
file: MySQL, MariaDB, PostgreSQL, SQLite, MongoDB and more, from one page.

```
spin up adminer             # http://localhost:8084
spin open adminer           # the same, in your browser
```

## Ports

| Service | Host port | Container | Env var        |
|---------|-----------|-----------|----------------|
| Adminer | `8084`    | 8080      | `ADMINER_PORT` |

## Logging in

Adminer has no account of its own: you log in with the credentials of the
database you want to look at. Running it alongside spinup's databases, the
fields are:

| Field    | postgres                | mysql / mariadb         |
|----------|-------------------------|-------------------------|
| System   | PostgreSQL              | MySQL                   |
| Server   | `host.docker.internal`  | `host.docker.internal`  |
| Username | `spinup`                | `spinup`                |
| Password | `spinup`                | `spinup`                |
| Database | `spinup`                | `spinup`                |

`host.docker.internal` — not `localhost` — because localhost inside the Adminer
container is the container. Every spinup database publishes its port on your
machine, and that name is how a container reaches it. Docker Desktop provides
the name; on Linux the `extra_hosts: host-gateway` line in `compose.yaml` does.

If a database runs on a non-default port, add it: `host.docker.internal:15432`.

## Notes

- Adminer *is* this stack's web interface, so there is no `gui` profile — there
  is nothing else to start.
- The stack keeps no data of its own and declares no volume; everything it
  shows lives in the database you point it at.
- Each of spinup's database stacks also ships its own GUI (pgAdmin,
  phpMyAdmin, mongo-express). Adminer is the one page that reaches all of them
  at once, which is worth having when two of them are running.
