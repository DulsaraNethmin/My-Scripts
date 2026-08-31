# opensearch

[OpenSearch](https://opensearch.org/) 3 — the Apache-2.0 fork of
Elasticsearch — with OpenSearch Dashboards as the optional web GUI.

```
spinup up opensearch                 # REST API on http://localhost:9200
spinup up opensearch --gui           # + Dashboards on http://localhost:8094
spinup cli opensearch                # lists the indices
curl localhost:9200/_cluster/health
```

## Ports

| Service    | Host port              | Container | Env var                      |
|------------|------------------------|-----------|------------------------------|
| REST API   | `9200`                 | 9200      | `OPENSEARCH_PORT`            |
| Dashboards | `8094` (`gui` profile) | 5601      | `OPENSEARCH_DASHBOARDS_PORT` |

## Credentials

None, and that is a deliberate choice worth understanding.

With the security plugin on — the default for the image — port 9200 is
*HTTPS* with a self-signed certificate, and every request needs `-k` and an
admin password that must satisfy a complexity rule. Locally that buys nothing
and costs an afternoon, so this stack sets `DISABLE_SECURITY_PLUGIN=true` and
serves plain HTTP with no authentication.

The consequence is that a client configured against this stack is not
configured the way it would be against a real cluster: no TLS, no auth header,
no roles. If you are specifically testing that wiring, delete the two
`DISABLE_*` variables from `compose.yaml` and set
`OPENSEARCH_INITIAL_ADMIN_PASSWORD` — the URL then becomes
`https://admin:<password>@localhost:9200`.

## Using it

```
curl -X POST 'localhost:9200/books/_doc?refresh=true' \
  -H 'Content-Type: application/json' -d '{"title":"Dune","year":1965}'
curl 'localhost:9200/books/_search?q=Dune'
curl 'localhost:9200/_cat/indices?v'
```

The API is Elasticsearch 7.10's, so most Elasticsearch clients and query
bodies work unchanged. Dashboards is the fork of Kibana at the same point.

## Storage

`opensearch-data` holds the indices. `spinup down opensearch` keeps them,
`spinup destroy opensearch` deletes them.

## Notes

- A single-node cluster reports **yellow** as soon as it holds an index, and
  that is correct rather than broken: every index defaults to one replica and
  there is no second node to put it on. The healthcheck asks
  `/_cluster/health` without requiring green for exactly that reason. Set
  `number_of_replicas: 0` on an index to make it green.
- The heap is pinned to 512 MB with `OPENSEARCH_HEAP`. Left alone the JVM
  takes a quarter of the host's RAM, which is not what you want from one of
  several stacks on a laptop. Raise it for anything with real data in it.
- `bootstrap.memory_lock` is on with an unlimited `memlock` ulimit, which
  stops the heap being swapped out. Both halves are required; one without the
  other is a startup failure.
- Dashboards is a container of its own, so it lives behind the `gui` profile
  and waits for OpenSearch to be healthy before starting.
- This is the heaviest image in the catalog after `mssql` and `pytorch`
  (~2 GB), and the JVM takes around 40 seconds to come up. The default
  `spinup up` timeout of three minutes is comfortably enough.
