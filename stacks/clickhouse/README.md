# clickhouse

[ClickHouse](https://clickhouse.com/) 26.3 LTS — a column store built for
analytical queries over a lot of rows, with its Play query UI served from the
same port as the HTTP API.

```
spin up clickhouse          # http://localhost:8123
spin open clickhouse        # Play, at /play
spin cli clickhouse         # clickhouse-client, inside the container
spin url clickhouse         # the HTTP connection string
```

## Ports

| Service         | Host port | Container | Env var                   |
|-----------------|-----------|-----------|---------------------------|
| HTTP / Play     | `8123`    | 8123      | `CLICKHOUSE_HTTP_PORT`    |
| Native protocol | `9001`    | 9000      | `CLICKHOUSE_NATIVE_PORT`  |

`9001`, not the native `9000`: the `minio` stack has that one, and every stack
in the catalog has to be able to run beside every other. Native drivers that
assume 9000 need the port spelled out.

## Credentials

| What     | Default  |
|----------|----------|
| User     | `spinup` |
| Password | `spinup` |
| Database | `spinup` |

```
http://spinup:spinup@localhost:8123
clickhouse-client --host localhost --port 9001 --user spinup --password spinup
```

Setting `CLICKHOUSE_USER` *replaces* `default` rather than adding a second
account — the image writes a `users.d` file that removes it — so there is no
passwordless superuser left listening on a published port. `SELECT name FROM
system.users` returns exactly one row.

The credentials are applied on the first start. Changing them later needs
`spin destroy clickhouse`.

## Querying

Everything is reachable over HTTP, so `curl` is a working client:

```
curl 'http://spinup:spinup@localhost:8123/?query=SELECT%20version()'
curl 'http://spinup:spinup@localhost:8123/' --data-binary \
  "SELECT number, number*2 FROM numbers(5) FORMAT Pretty"
```

Play (`spin open clickhouse`) is the same thing with a text box, a results
grid and query history. It asks for the credentials above.

## Seeding

Anything in `init/` — `.sql`, `.sql.gz` or `.sh` — runs once, when the data
volume is empty. `CLICKHOUSE_DB` is created first, but the scripts run against
`default`, not against it, so a seed file has to say which database it means:

```sql
USE spinup;
CREATE TABLE events (id UInt32, name String) ENGINE = MergeTree ORDER BY id;
INSERT INTO events VALUES (1, 'created');
```

Without that `USE` the tables land in `default` and every later query looks for
them in `spinup`, which is a confusing five minutes.

## Storage

`clickhouse-data` holds the tables, `clickhouse-logs` the server logs — the
second is a volume of its own because ClickHouse writes a lot of them and they
would otherwise grow the container's writable layer. `spin down clickhouse`
keeps both; `spin destroy clickhouse` deletes them.

## Notes

- Play is served by the database container, not a separate one, so this stack
  has no `gui` profile and `--gui` has nothing to select.
- The `nofile` ulimit is raised to 262144, which is what ClickHouse's own
  packaging sets. It opens a file per column per part, and on a default 1024
  it logs a warning it means.
- The healthcheck probes `/ping` with `wget`: this image is Ubuntu-based and
  ships no curl. `/ping` needs no credentials.
- ClickHouse is not a drop-in for a row store — no transactions, and `UPDATE`
  and `DELETE` are asynchronous `ALTER TABLE ... UPDATE` mutations. Reach for
  it when the query is an aggregate over a lot of rows.
