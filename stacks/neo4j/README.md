# neo4j

[Neo4j](https://neo4j.com/) 5 Community — a property-graph database queried in
Cypher, with Neo4j Browser served by the database itself.

```
spin up neo4j               # bolt://localhost:7687
spin open neo4j             # Neo4j Browser on http://localhost:8091
spin cli neo4j              # cypher-shell, inside the container
spin url neo4j              # the Bolt connection string
```

## Ports

| Service       | Host port | Container | Env var              |
|---------------|-----------|-----------|----------------------|
| Bolt          | `7687`    | 7687      | `NEO4J_BOLT_PORT`    |
| Neo4j Browser | `8091`    | 7474      | `NEO4J_BROWSER_PORT` |

Bolt keeps its native port — drivers and `neo4j://` URLs assume it. Browser
moves off 7474 onto 8091 so it sits with every other GUI in the catalog's
`80xx` range.

## Credentials

| What     | Default      |
|----------|--------------|
| User     | `neo4j`      |
| Password | `spinuppass` |

```
bolt://neo4j:spinuppass@localhost:7687
```

The user is always `neo4j`: `NEO4J_AUTH` can set the initial password but not
the name. The password needs at least 8 characters, which is why the default
is not the catalog's usual `spinup`. It is written on the first start only —
`spin destroy neo4j` to change it later.

## Querying

In Browser, or through `spin cli neo4j`:

```cypher
CREATE (a:Person {name: 'Ada'})-[:KNOWS]->(b:Person {name: 'Grace'});
MATCH (p:Person)-[:KNOWS]->(q) RETURN p.name, q.name;
```

## Seeding

Files in `import/` are visible to the database as `file:///`:

```cypher
LOAD CSV WITH HEADERS FROM 'file:///people.csv' AS row
CREATE (:Person {name: row.name});
```

The directory is mounted read-only, so `LOAD CSV` works and nothing in the
container can write back into your stack folder.

## Storage

`neo4j-data` holds the graph, `neo4j-logs` the server logs. `spin down neo4j`
keeps both; `spin destroy neo4j` deletes them.

## Notes

- Browser is served by the database container, not a separate one, so this
  stack has no `gui` profile and `--gui` has nothing to select.
- `NEO4J_server_bolt_advertised__address` is what makes Browser's connect form
  point at the right host port when `NEO4J_BOLT_PORT` is overridden. Without it
  Browser would advertise 7687 whatever the host binding actually is. The
  double underscore is Neo4j's escape for a dot in a config key.
- The healthcheck probes HTTP on 7474 rather than Bolt: it comes up *after*
  Bolt does, so a healthy container means both are answering.
- Community edition, so one database per server (`neo4j`) and no role-based
  access control. The enterprise image needs a licence.
