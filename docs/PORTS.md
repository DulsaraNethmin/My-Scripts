# Host port registry

Every stack must be able to run at the same time as every other stack, so host
ports are allocated centrally here. **Claim your ports in this table before
writing a stack's `compose.yaml`.**

Conventions:

- Services keep their well-known native port where it is free (`5432` for
  Postgres, `6379` for Redis) — muscle memory and default connection strings
  should just work.
- Web GUIs live in the `80xx` range, one per stack, never on port 80 (it
  collides with local web servers and needs root on Linux).
- Every port is overridable per machine via the stack's env file; this table is
  the default, not a constraint.

## Native / data ports

| Port    | Stack                 | Service                     |
|---------|-----------------------|-----------------------------|
| `1025`  | mailpit               | SMTP                        |
| `1433`  | mssql                 | SQL Server                  |
| `3306`  | mysql                 | MySQL                       |
| `3307`  | mariadb               | MariaDB (offset, mysql has 3306) |
| `4222`  | nats                  | NATS client                 |
| `4566`  | localstack            | edge / all AWS services     |
| `5432`  | postgres              | PostgreSQL                  |
| `5433`  | pgvector              | PostgreSQL (offset, postgres has 5432) |
| `5672`  | rabbitmq              | AMQP                        |
| `5984`  | couchdb               | HTTP API (+ `/_utils` GUI)  |
| `6006`  | pytorch               | TensorBoard                 |
| `6333`  | qdrant                | REST API + `/dashboard`     |
| `6334`  | qdrant                | gRPC                        |
| `6379`  | redis                 | Redis                       |
| `7687`  | neo4j                 | Bolt                        |
| `8000`  | chroma                | HTTP API                    |
| `8123`  | clickhouse            | HTTP                        |
| `8200`  | vault                 | HTTP API + built-in UI      |
| `8888`  | pytorch               | Jupyter                     |
| `9000`  | minio                 | S3 API                      |
| `9001`  | clickhouse            | native protocol (9000 taken)|
| `9080`  | weaviate              | REST/GraphQL (8080 is pgAdmin's) |
| `9090`  | monitoring            | Prometheus                  |
| `9092`  | kafka                 | broker                      |
| `9200`  | opensearch            | REST API                    |
| `27017` | mongodb               | MongoDB                     |
| `50051` | weaviate              | gRPC                        |

## Web GUIs (`80xx`)

| Port   | Stack                 | GUI                  |
|--------|-----------------------|----------------------|
| `8080` | postgres              | pgAdmin              |
| `8081` | mysql                 | phpMyAdmin           |
| `8082` | mongodb               | mongo-express        |
| `8083` | redis                 | RedisInsight         |
| `8084` | adminer               | Adminer              |
| `8085` | mailpit               | Mailpit UI           |
| `8086` | minio                 | MinIO Console        |
| `8087` | rabbitmq              | Management UI        |
| `8088` | nats                  | monitoring endpoint  |
| `8089` | portainer             | Portainer            |
| `8090` | nginx-static          | the served site      |
| `8091` | neo4j                 | Neo4j Browser        |
| `8092` | traefik               | dashboard            |
| `8093` | kafka                 | kafka-ui             |
| `8094` | opensearch            | OpenSearch Dashboards|
| `8095` | monitoring            | Grafana              |
| `8096` | keycloak              | admin console        |
| `8097` | mariadb               | Adminer (stack-local)|
| `8098` | traefik               | `web` entrypoint     |
| `8099` | *(scaffold)*          | `spin new` default |

## Exceptions

`8099` is not a stack: it is the port `spin new` puts in a scaffolded stack,
kept out of the table above so a new stack of your own does not collide with a
built-in one on its first `spin up`.

`nginx-proxy-manager` binds `80`, `443` and `81` by design — it *is* the edge
proxy. It is the one stack that expects to own port 80, and it cannot run
alongside another stack that binds 80.

`traefik` is the other proxy, and it does *not* take 80 for that reason: its
`web` entrypoint is on `8098`, in the GUI range even though it is not a GUI,
because that is where the free numbers are. Routed URLs therefore carry the
port (`http://whoami.localhost:8098`), which is the price of the two proxy
stacks being able to run at the same time.

`weaviate` binds `9080` rather than its own `8080`, which pgAdmin has in the
`postgres` stack, and `50051` for gRPC. Both have to be passed to clients
explicitly, because `weaviate.connect_to_local()` assumes `8080`/`50051`.
`50051` is the canonical gRPC port and so the likeliest default in this table
to collide with something outside spinup; `WEAVIATE_GRPC_PORT` moves it.

`pgvector` is a second PostgreSQL and cannot have `5432`, so it takes `5433` —
the same offset-with-a-reason as `mariadb` on `3307`.
