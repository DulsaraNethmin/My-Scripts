# nginx-static

nginx serving a static site or a single-page-app build, built locally from the
`Dockerfile` in this folder.

```
spinup up nginx-static
spinup open nginx-static
```

## Ports

| Service | Host port | Container | Env var      |
|---------|-----------|-----------|--------------|
| nginx   | `8090`    | 80        | `NGINX_PORT` |

## Serving your own site

By default the stack serves a placeholder page from a named volume seeded out
of the image. Point `NGINX_SITE` at a folder on your machine to serve that
instead:

```
spinup env nginx-static --edit
# NGINX_SITE=/Users/you/project/dist
spinup restart nginx-static
```

Compose treats any value starting with `/` or `.` as a bind mount and anything
else as a named volume, so this one variable switches between the two with no
edit to `compose.yaml`.

## SPA routing

`nginx.conf` uses `try_files $uri /index.html`, so unknown paths return your
`index.html` rather than a 404. React Router, Vue Router and friends work
without extra configuration. Static assets (`css`, `js`, images, fonts) are
served with a six-month cache header and no access logging.

## Gotchas

- The old `nginxServer/run_server.sh` ran `nethmindulsara/nginx-server:v1` from
  Docker Hub — a personal image nobody else could pull or rebuild — while
  ignoring the perfectly good `Dockerfile` sitting next to it. This stack builds
  locally, so what runs is what is in the repo.
- Editing files in a bind-mounted folder shows up immediately; no restart is
  needed. Changing `NGINX_SITE` itself does need a restart, because it changes
  the mount.
- The six-month asset cache is aggressive for local development. If you are
  iterating on CSS and see stale files, hard-reload, or drop the `expires`
  block from `nginx.conf`.
- This stack takes 8090, not 80, so it can run next to everything else. The
  `nginx-proxy-manager` stack is the one that wants port 80.
