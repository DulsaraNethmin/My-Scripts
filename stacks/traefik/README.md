# traefik

[Traefik](https://traefik.io/) v3 — a reverse proxy that builds its routing
table from Docker labels, so a container becomes a hostname by adding two
lines to its compose file.

```
spin up traefik                       # proxy on 8098, dashboard on 8092
spin open traefik                     # the dashboard
curl http://whoami.localhost:8098/      # the example route this stack ships
```

## Ports

| Service          | Host port | Container | Env var                   |
|------------------|-----------|-----------|---------------------------|
| `web` entrypoint | `8098`    | 80        | `TRAEFIK_HTTP_PORT`       |
| Dashboard        | `8092`    | 8080      | `TRAEFIK_DASHBOARD_PORT`  |

`8098`, not `80`: the `nginx-proxy-manager` stack owns port 80 in this catalog,
and on Linux binding it needs root. The cost is that routed URLs carry the
port — `http://whoami.localhost:8098` rather than `http://whoami.localhost`.
If you want the bare hostname and nothing else is on 80, set
`TRAEFIK_HTTP_PORT=80`.

## Credentials

None. `--api.insecure=true` puts the dashboard on its own port with no
authentication, which is the only reason it is one command to open. It is also
why this stack belongs on your machine and nowhere else.

## Routing your own container

Traefik reads labels through the Docker socket. `exposedByDefault=false` means
a container is routed only when it asks:

```yaml
services:
  api:
    image: my-api
    networks: [spinup-traefik_default]
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.api.rule=Host(`api.localhost`)"
      - "traefik.http.routers.api.entrypoints=web"
      # Only needed when the container exposes more than one port.
      - "traefik.http.services.api.loadbalancer.server.port=3000"

networks:
  spinup-traefik_default:
    external: true
```

Two things have to be true or the route silently will not appear: the
container must be on Traefik's network (`spinup-traefik_default`, created by
this stack), and `traefik.enable=true` must be set. The dashboard's HTTP
Routers page shows what Traefik actually saw, which is the fastest way to tell
the two failures apart.

`*.localhost` resolves to `127.0.0.1` without touching `/etc/hosts` on macOS
and on Linux with systemd-resolved. Elsewhere, add the name to `/etc/hosts` or
send the header by hand:

```
curl -H 'Host: api.localhost' http://localhost:8098/
```

## The example route

`whoami` is a 4 MB echo server with the labels above already on it, published
nowhere — the only way to reach it is through the proxy, which is what makes
it a test of the proxy. Delete the service once you have your own; nothing
else refers to it.

## Notes

- The dashboard is served by Traefik itself, not a separate container, so this
  stack has no `gui` profile.
- The socket is mounted read-only. Traefik only reads labels, and a proxy with
  write access to the daemon is a proxy that can start containers.
- The healthcheck is `traefik healthcheck --ping`, the binary's own client
  against the `/ping` endpoint that `--ping=true` turns on.
- No HTTPS entrypoint. Locally it would mean a self-signed certificate and a
  browser warning on every route; Let's Encrypt needs a public name. Add
  `--entryPoints.websecure.address=:443` and a certificate resolver if you
  need it.
- `TRAEFIK_LOG_LEVEL=DEBUG` spells out why a router did not match, which is
  most of what goes wrong here.
