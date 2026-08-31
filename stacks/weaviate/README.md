# weaviate

[Weaviate](https://weaviate.io/) 1.39, a schema-first vector database with REST,
GraphQL and gRPC interfaces.

```
spin up weaviate            # the server
spin cli weaviate           # dump the schema
spin url weaviate           # print the base URL
```

## Ports

| Service  | Host port | Container | Env var              |
|----------|-----------|-----------|----------------------|
| weaviate | `9080`    | 8080      | `WEAVIATE_HTTP_PORT` |
| weaviate | `50051`   | 50051     | `WEAVIATE_GRPC_PORT` |

**`9080`, not Weaviate's own `8080`** — pgAdmin in the `postgres` stack has that
one, and every stack has to be able to run beside every other. Every client
default assumes `8080`, so this is the one thing you must remember to pass.

`50051` is gRPC, which the modern clients need for batch writes and search; it
is not optional if you use them.

## Credentials

**None.** Anonymous access is enabled, so anything that can reach the port can
read and write. That is the right default for a local development database and
the wrong one for anything else — never bind this stack to a network you do not
control.

## Connecting

```python
import weaviate
client = weaviate.connect_to_local(port=9080, grpc_port=50051)
```

Both ports have to be named. `connect_to_local()` with no arguments goes to
`8080`/`50051` and will reach pgAdmin, or nothing at all.

Over HTTP:

```
curl http://localhost:9080/v1/meta
curl http://localhost:9080/v1/schema
```

## Using it

Bring your own vectors — the stack sets `DEFAULT_VECTORIZER_MODULE=none`,
because every built-in vectorizer wants a third-party API key.

```
# a collection
curl -X POST http://localhost:9080/v1/schema \
  -H 'content-type: application/json' \
  -d '{"class": "Document", "vectorizer": "none",
       "properties": [{"name": "title", "dataType": ["text"]}]}'

# an object, with its vector
curl -X POST http://localhost:9080/v1/objects \
  -H 'content-type: application/json' \
  -d '{"class": "Document", "properties": {"title": "first"},
       "vector": [0.1, 0.2, 0.3]}'

# nearest neighbours, in GraphQL
curl -X POST http://localhost:9080/v1/graphql \
  -H 'content-type: application/json' \
  -d '{"query": "{ Get { Document(nearVector: {vector: [0.1,0.2,0.3]} limit: 2) { title } } }"}'
```

## Vectorizers

`DEFAULT_VECTORIZER_MODULE=none` means a collection that does not name one gets
none. The API-based modules are compiled in and listed by `/v1/meta`, but they
are inert without a provider key, so the honest default is to expect vectors
from you.

## Storage

One named volume, `weaviate-data`, at `/var/lib/weaviate` — which is *not* the
image's default of `/data`, so `PERSISTENCE_DATA_PATH` in `compose.yaml` names
it explicitly. Remove that variable and the volume would mount an empty
directory Weaviate never writes to.

`spin down` keeps it; only `spin destroy` deletes it.

## Gotchas

- **`CLUSTER_HOSTNAME` is load-bearing, not cosmetic.** Weaviate names its raft
  node after the container's hostname, and compose sets that to the container
  ID — a new one on every recreate. Without a fixed value the second
  `spin up weaviate` after a `spin down` never becomes ready: every read fails
  with `could not read schema with strong consistency: failed to execute query:
  leader not found` and `/v1/.well-known/ready` answers 503 indefinitely. Do
  not change `node1` on an existing volume.
- **The port moved to `9080`.** See Ports. This is the single most common thing
  to get wrong with this stack.
- **`50051` is the most contended port in this catalog** — it is *the* gRPC
  port, and plenty of unrelated tooling binds it. If `spin up weaviate` reports
  it taken, `spin up weaviate --port WEAVIATE_GRPC_PORT=50052`, or change it
  for good with `spin env weaviate --edit`.
- `PERSISTENCE_DATA_PATH` must stay in step with the volume mount. See Storage.
- **No web interface.** `/` is a JSON link document; `/console` and `/ui` are
  404. That is why this stack has no `gui` block. Use `spin cli weaviate`,
  curl, or a client library.
- BusyBox `wget` in this image has no `--user`/`--password`. Nothing here needs
  it, but do not copy the healthcheck into a stack that does.
