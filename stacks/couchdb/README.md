# couchdb

[Apache CouchDB](https://couchdb.apache.org/) 3 — a document database whose API
*is* HTTP, with [Fauxton](https://docs.couchdb.org/en/stable/fauxton/) served
from the same port.

```
spin up couchdb                     # http://localhost:5984
spin open couchdb                   # Fauxton, at /_utils
spin cli couchdb                    # lists the databases
curl http://spinup:spinup@localhost:5984/_all_dbs
```

## Ports

| Service  | Host port | Container | Env var        |
|----------|-----------|-----------|----------------|
| HTTP API | `5984`    | 5984      | `COUCHDB_PORT` |

Fauxton is not a second service — CouchDB serves it at `/_utils` on the API
port — so this stack has no `gui` profile and one port.

## Credentials

| What     | Default  |
|----------|----------|
| User     | `spinup` |
| Password | `spinup` |

```
http://spinup:spinup@localhost:5984
```

The admin is written from `COUCHDB_USER` / `COUCHDB_PASSWORD` on every start,
not just the first, because CouchDB keeps it in a config file rather than in
the data directory. Changing them in the env file and restarting is enough —
no `destroy` needed, unlike the SQL stacks.

## Using it

Everything is HTTP, so `curl` is a complete client:

```
curl -X PUT    http://spinup:spinup@localhost:5984/notes
curl -X POST   http://spinup:spinup@localhost:5984/notes \
     -H 'Content-Type: application/json' -d '{"title":"hello"}'
curl           http://spinup:spinup@localhost:5984/notes/_all_docs
curl           http://spinup:spinup@localhost:5984/notes/_changes?feed=continuous
```

## Storage

`couchdb-data` holds the databases. `spin down couchdb` keeps them,
`spin destroy couchdb` deletes them.

Note that the *config* — the admin user and the auth secret — is not on that
volume; it is rebuilt from the environment each start. That is why the two
behave differently.

## Notes

- `couchdb-setup` is a one-shot container that runs CouchDB's documented
  single-node setup (`POST /_cluster_setup`) once the server is healthy, which
  creates the `_users` and `_replicator` system databases. A stock CouchDB 3 has
  neither, and Fauxton nags about it on every page. It is idempotent, runs
  on every start, and shows as `Exited (0)` in `spin ps`.
- The healthcheck uses `curl`, not `wget`: this image is Debian-based and ships
  curl only. `/_up` is CouchDB's own readiness endpoint and needs no auth.
- `COUCHDB_SECRET` is pinned so that a restart does not invalidate browser
  sessions. Locally that is a convenience; a real deployment wants a secret
  that is actually secret.
- CouchDB replicates to any other CouchDB (or PouchDB in a browser) over the
  same HTTP API — `POST /_replicate` — which is the reason to reach for it
  rather than another document store.
