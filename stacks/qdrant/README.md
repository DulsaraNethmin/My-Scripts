# qdrant

[Qdrant](https://qdrant.tech/) 1.19, a vector database written in Rust, with its
own web dashboard served from the same port as the API.

```
spin up qdrant              # database + dashboard
spin open qdrant            # the dashboard, in your browser
spin url qdrant             # print the base URL
spin info qdrant            # ports, credentials, what is running
```

## Ports

| Service | Host port | Container | Env var             |
|---------|-----------|-----------|---------------------|
| qdrant  | `6333`    | 6333      | `QDRANT_HTTP_PORT`  |
| qdrant  | `6334`    | 6334      | `QDRANT_GRPC_PORT`  |

`6333` carries both the REST API and the dashboard at `/dashboard`; `6334` is
gRPC, which the official Python, Rust and Go clients prefer for batch upserts
and search. There is no separate GUI container and so no `gui` profile.

## Credentials

| What    | Default  |
|---------|----------|
| API key | `spinup` |

The key is a header, not part of the URL, so `spin url qdrant` prints a bare
address:

```
curl -H 'api-key: spinup' http://localhost:6333/collections
```

`/healthz`, `/readyz`, `/livez` and the dashboard shell are exempt from the
check; everything else is not. Unlike a database password, the key is re-read
on every start — change it in `spin env qdrant --edit` and restart, no destroy
needed. Set it to nothing at all and Qdrant logs `api_key is set but empty, it
is treated as unset` and serves everything unauthenticated.

These are development defaults on a local machine. Never expose this stack to a
network you do not control.

## Using it

The container has no HTTP client of its own, so run these from your machine:

```
# a collection of 4-dimensional vectors compared by cosine distance
curl -X PUT http://localhost:6333/collections/demo \
  -H 'api-key: spinup' -H 'content-type: application/json' \
  -d '{"vectors": {"size": 4, "distance": "Cosine"}}'

# two points, with payloads
curl -X PUT http://localhost:6333/collections/demo/points \
  -H 'api-key: spinup' -H 'content-type: application/json' \
  -d '{"points": [
        {"id": 1, "vector": [0.1, 0.2, 0.3, 0.4], "payload": {"kind": "a"}},
        {"id": 2, "vector": [0.9, 0.8, 0.7, 0.6], "payload": {"kind": "b"}}]}'

# nearest neighbours
curl -X POST http://localhost:6333/collections/demo/points/search \
  -H 'api-key: spinup' -H 'content-type: application/json' \
  -d '{"vector": [0.1, 0.2, 0.3, 0.4], "limit": 2, "with_payload": true}'
```

From Python:

```python
from qdrant_client import QdrantClient
client = QdrantClient(url="http://localhost:6333", api_key="spinup")
```

## The dashboard

`http://localhost:6333/dashboard` — collections, a point browser, a visualiser
and a console for raw API calls. It asks for the API key once and keeps it in
the browser.

## Storage

Two named volumes, because Qdrant keeps them apart:

| Volume              | Container path       | What                        |
|---------------------|----------------------|-----------------------------|
| `qdrant-storage`    | `/qdrant/storage`    | collections and their index |
| `qdrant-snapshots`  | `/qdrant/snapshots`  | snapshots you create        |

Snapshots live in a *sibling* directory of storage rather than inside it, so a
single volume would have quietly thrown away every backup when the container
was recreated.

`spin down` keeps both; only `spin destroy` deletes them.

## Gotchas

- The image ships **no curl and no wget** — it is Debian with a Rust binary and
  little else. That is why the healthcheck is a bash `/dev/tcp` probe and why
  this stack has no `spin cli`; `spin shell qdrant` gets you a bash prompt.
- Snapshots need their own volume. See Storage above.
- The API key is a header, so it never appears in `spin url`.
- The dashboard is on the API port. There is no second port to open and no
  `--gui` flag to pass.
- gRPC on `6334` is a second published port; if you only ever use REST you can
  ignore it, but the Python client opens it when `prefer_grpc=True`.
