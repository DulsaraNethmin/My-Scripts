# pgvector

PostgreSQL 17 with [pgvector](https://github.com/pgvector/pgvector), for when
you would rather keep embeddings next to the rest of your data than run a
second database.

```
spin up pgvector            # database, extension already installed
spin cli pgvector           # psql, inside the container
spin url pgvector           # print the connection string
```

## Ports

| Service  | Host port | Container | Env var         |
|----------|-----------|-----------|-----------------|
| pgvector | `5433`    | 5432      | `PGVECTOR_PORT` |

`5433`, not `5432`: the `postgres` stack has that one, and every stack has to
be able to run beside every other. So `psql -h localhost` alone reaches
`postgres`; this one needs `-p 5433`.

## Credentials

| What     | Default  |
|----------|----------|
| User     | `spinup` |
| Password | `spinup` |
| Database | `spinup` |

```
postgres://spinup:spinup@localhost:5433/spinup
```

These are development defaults on a local machine. Change them in
`spin env pgvector --edit`, and never expose this stack to a network you do
not control.

## The extension

The image makes `vector` *available* but does not install it — a fresh database
lists it in `pg_available_extensions` with an empty `installed_version`. This
stack ships `init/01-vector.sql`, which installs it on first start, so there is
nothing for you to run:

```
spin cli pgvector
\dx
```

```
 vector | 0.8.6 | public | vector data type and ivfflat and hnsw access methods
```

The script also installs it into `template1`, so any database you create later
inherits it and `CREATE DATABASE app` does not hand you a Postgres with no
vector type.

## Querying

```sql
CREATE TABLE items (id bigserial PRIMARY KEY, embedding vector(3));
INSERT INTO items (embedding) VALUES ('[1,2,3]'), ('[4,5,6]');

-- nearest neighbours by L2 distance
SELECT id, embedding <-> '[3,1,2]' AS distance
  FROM items ORDER BY embedding <-> '[3,1,2]' LIMIT 5;
```

The three operators are `<->` L2, `<=>` cosine, `<#>` negative inner product.
For anything beyond a few thousand rows, add an index:

```sql
CREATE INDEX ON items USING hnsw (embedding vector_l2_ops);
```

## Seeding

Same contract as the `postgres` stack: anything in `init/` — `.sql`, `.sql.gz`
or `.sh` — runs once, in alphabetical order, the first time the data volume is
created. `01-vector.sql` is already there, so name yours `02-` or later.

## Storage

One named volume, `pgvector-data`, at `/var/lib/postgresql/data`. `spin down`
keeps it; only `spin destroy` deletes it.

## Gotchas

- **`init/` runs only on an empty data volume.** Edit `01-vector.sql`, or add a
  seed script, after the stack has started once and nothing happens.
  `spin destroy pgvector` first, then `spin up`. This surprises everyone at
  least once.
- The env vars are `PGVECTOR_*`, not `POSTGRES_*`, deliberately: compose gives
  your shell precedence over the env file, so a `POSTGRES_PASSWORD` you
  exported for something else would otherwise reach in and change this stack.
- `PGVECTOR_USER`, `PGVECTOR_PASSWORD` and `PGVECTOR_DB` are only read when the
  data volume is empty — the same rule as the `postgres` stack.
- Restoring a dump into a database created *outside* this container means
  running `CREATE EXTENSION vector;` yourself; only `template1` was seeded.
- **No GUI here.** The `postgres` stack already ships pgAdmin on `8080` and
  `adminer` sits on `8084`; a third admin container would cost a port and a
  few hundred MB for nothing. From either one, connect to host
  `host.docker.internal`, port `5433` — not `localhost`, which inside those
  containers is the container itself.
- Pinned to `pg17`, not `latest`: the tag family runs `pg13`…`pg18`, and the
  pgvector version rides along with it.
