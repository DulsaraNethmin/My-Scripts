# mssql

Microsoft SQL Server 2022, Developer edition — free and feature-complete for
non-production use.

```
spin up mssql
spin url mssql              # print the connection string
spin cli mssql              # sqlcmd, inside the container
```

There is no GUI in this stack. Use Azure Data Studio, DBeaver or SSMS from the
host against `localhost:1433`.

## Ports

| Service | Host port | Container | Env var      |
|---------|-----------|-----------|--------------|
| mssql   | `1433`    | 1433      | `MSSQL_PORT` |

## Credentials

| What        | Default           |
|-------------|-------------------|
| User        | `sa`              |
| Password    | `Spinup!Passw0rd` |

```
sqlserver://sa:Spinup!Passw0rd@localhost:1433
```

SQL Server enforces its own password policy — at least 8 characters using three
of uppercase, lowercase, digits and symbols. This is why the default is not
plain `spinup` like the other stacks. A weak password does **not** produce a
clear error: the server just exits during setup, and you have to read the
container logs to find out why.

## Seeding

SQL Server has no `docker-entrypoint-initdb.d` equivalent, so unlike the other
database stacks there is no `init/` directory here. Seed it after the container
is healthy:

```
spin up mssql
docker exec -i spinup-mssql-mssql-1 /opt/mssql-tools18/bin/sqlcmd \
  -S 127.0.0.1 -U sa -P 'Spinup!Passw0rd' -C -i /dev/stdin < schema.sql
```

Or restore a `.bak` by mounting it and using `RESTORE DATABASE`.

## Apple Silicon

Microsoft publishes this image for **amd64 only**, so on an M-series Mac it runs
under Docker Desktop's emulation. It does work — verified on an M-series host,
where the container reports `x86_64` — but expect a noticeably slower first
start; the healthcheck allows a 60 s `start_period` and up to 5 minutes total
for this reason.

If startup times bother you, [Azure SQL Edge] used to be the arm64 alternative,
but it is retired and no longer receives updates, so this stack stays on the
real thing.

[Azure SQL Edge]: https://learn.microsoft.com/en-us/azure/azure-sql-edge/

## Gotchas

- `SA_PASSWORD` is **deprecated**. The 2022 image reads `MSSQL_SA_PASSWORD`; the
  old stack used the deprecated name and the 2019 image.
- `sqlcmd` lives at `/opt/mssql-tools18/bin/sqlcmd`, not on `PATH`, and tools18
  verifies TLS by default — pass `-C` to trust the container's self-signed
  certificate or every connection fails.
- You must accept the [Microsoft EULA](https://go.microsoft.com/fwlink/?linkid=857698)
  (`MSSQL_ACCEPT_EULA=Y`) for the image to start at all.
- `spin down` keeps your data; only `spin destroy` deletes it.
