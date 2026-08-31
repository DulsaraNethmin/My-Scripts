# keycloak

[Keycloak](https://www.keycloak.org/) 26 — OAuth 2.0, OpenID Connect and SAML,
with users, realms, clients and social logins — backed by its own Postgres so
what you configure survives a restart.

```
spin up keycloak            # http://localhost:8096
spin open keycloak          # the admin console
spin cli keycloak -- get realms --fields realm
```

## Ports

| Service  | Host port       | Container | Env var          |
|----------|-----------------|-----------|------------------|
| Keycloak | `8096`          | 8080      | `KEYCLOAK_PORT`  |
| Postgres | *not published* | 5432      | —                |

The database is Keycloak's own and is deliberately not published. When you
want a Postgres of your own, `spin up postgres` is the one with `5432` and a
GUI.

## Credentials

| What           | Default  |
|----------------|----------|
| Admin user     | `spinup` |
| Admin password | `spinup` |

Created once, on an empty database. Changing `KEYCLOAK_ADMIN_PASSWORD`
afterwards does nothing — change it in the console, or `spin destroy
keycloak` and start over.

## Wiring an app to it

```
Issuer            http://localhost:8096/realms/master
Discovery         http://localhost:8096/realms/master/.well-known/openid-configuration
Authorization     http://localhost:8096/realms/master/protocol/openid-connect/auth
Token             http://localhost:8096/realms/master/protocol/openid-connect/token
```

Most OIDC libraries need only the issuer and will find the rest. In the
console, create a realm of your own rather than using `master` — `master` is
where the admins live, and its tokens are short-lived by design.

A quick check that it is really answering:

```
curl -s -X POST http://localhost:8096/realms/master/protocol/openid-connect/token \
  -d client_id=admin-cli -d username=spinup -d password=spinup -d grant_type=password
```

## Storage

`keycloak-db` holds everything — realms, clients, users, the admin account.
`spin down keycloak` keeps it; `spin destroy keycloak` deletes it and the
next start is a first start.

## Notes

- `start-dev`, not `start`: the production command needs a TLS certificate and
  a build step, and refuses to run without a hostname. Dev mode is HTTP, no
  hostname strictness, and it is otherwise the same server — realms exported
  from it import into a real one.
- **The `sslRequired` fix.** Keycloak's `master` realm ships with
  `sslRequired=EXTERNAL`, which rejects plain HTTP from any address it does
  not think is private — and behind Docker Desktop's port forwarding your own
  machine is one of those. Left alone, `curl http://localhost:8096/realms/...`
  answers `403 {"error":"invalid_request","error_description":"HTTPS
  required"}` and the admin console cannot log in, while the very same request
  from inside the container works. The `keycloak-setup` one-shot runs
  `kcadm.sh update realms/master -s sslRequired=NONE` once the server is
  healthy. It is idempotent, runs on every start and shows as `Exited (0)` in
  `spin ps`. A realm you create yourself needs the same setting — it is on
  the realm's Login tab as "Require SSL".
- The healthcheck is bash's `/dev/tcp` against `/health/ready` on the
  management port, 9000. The image is UBI micro with a JVM in it: no curl, no
  wget, and `CMD-SHELL` would run under `sh`, which has no `/dev/tcp`.
- `KC_BOOTSTRAP_ADMIN_USERNAME` / `_PASSWORD`, not `KEYCLOAK_ADMIN` /
  `KEYCLOAK_ADMIN_PASSWORD`. The old names were deprecated in 26.0 and are
  ignored, silently, leaving a server with no way in.
- `spin cli keycloak` is `kcadm.sh`, which needs to log in first:
  `spin shell keycloak`, then
  `./bin/kcadm.sh config credentials --server http://127.0.0.1:8080 --realm master --user spinup --password spinup`.
- Keycloak takes around 35 seconds to become healthy — a JVM, a database
  migration and a realm import on the first start.
