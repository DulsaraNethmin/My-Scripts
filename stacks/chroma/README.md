# chroma

[Chroma](https://www.trychroma.com/) 1.5, the vector database most embedding
tutorials reach for, running as a server so more than one process can share it.

```
spin up chroma              # the server
spin url chroma             # print the base URL
spin info chroma            # ports and what is running
```

## Ports

| Service | Host port | Container | Env var       |
|---------|-----------|-----------|---------------|
| chroma  | `8000`    | 8000      | `CHROMA_PORT` |

No GUI, no `gui` profile: this release serves an API and nothing else. The only
browsable page is the auto-generated Swagger reference at `/docs`, which
documents the API rather than showing your data.

## Credentials

**None.** This release has no server-side authentication of any kind — the
`CHROMA_SERVER_AUTHN_PROVIDER` and `CHROMA_SERVER_AUTHN_CREDENTIALS` variables
you will find in older guides were settings of the previous Python server and
are silently ignored by the Rust one. Anything that can reach the port can read
and write every collection.

That is fine on a local machine and is why the stack ships as it does. Never
bind it to a network you do not control.

## Using it

From Python, which is how most people use it:

```python
import chromadb
client = chromadb.HttpClient(host="localhost", port=8000)

col = client.get_or_create_collection("demo")
col.add(ids=["a", "b"],
        embeddings=[[0.1, 0.2, 0.3], [0.9, 0.8, 0.7]],
        documents=["first", "second"])
print(col.query(query_embeddings=[[0.1, 0.2, 0.3]], n_results=2))
```

Or over HTTP, noting the tenant and database in the path — v2 has no short form:

```
curl http://localhost:8000/api/v2/heartbeat
curl http://localhost:8000/api/v2/tenants/default_tenant/databases/default_database/collections
```

## Browsing your data

There is no web UI, but the CLI inside the container has a browser:

```
spin shell chroma
chroma browse demo --local --host http://localhost:8000
```

It needs a terminal — run it through `spin shell`, not from a script.

## Storage

One named volume, `chroma-data`, mounted at `/data`: `chroma.sqlite3` plus a
directory per index. `spin down` keeps it; only `spin destroy` deletes it.

## Gotchas

- **The v1 API is gone.** `/api/v1/heartbeat` answers `410 Gone` with *"The v1
  API is deprecated. Please use /v2 apis"*. Tutorials written before 2025 and
  any `chromadb` client older than 0.6 will fail against this image.
- **No authentication exists to turn on.** See Credentials.
- The image has no curl, wget or python, which is why the healthcheck is a bash
  `/dev/tcp` probe and there is no `spin cli chroma`.
- `chroma browse` panics without a TTY (`reader source not set`). Use
  `spin shell`.
- Telemetry is already off in this release — the log says *"No telemetry is
  configured"*, so there is no `ANONYMIZED_TELEMETRY` to set.
- Port `8000` is the most likely default in this catalog to already be busy on
  a Python developer's machine. `spin up chroma --port CHROMA_PORT=8010`.
