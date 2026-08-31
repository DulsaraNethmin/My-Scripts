# nginx-proxy-manager

[Nginx Proxy Manager](https://nginxproxymanager.com/) — a reverse proxy with a
web UI and one-click Let's Encrypt certificates.

```
spin up nginx-proxy-manager
spin open nginx-proxy-manager      # the admin UI on :81
```

## Ports

| Purpose        | Host port | Env var           |
|----------------|-----------|-------------------|
| HTTP proxy     | `80`      | `NPM_HTTP_PORT`   |
| HTTPS proxy    | `443`     | `NPM_HTTPS_PORT`  |
| Admin UI       | `81`      | `NPM_ADMIN_PORT`  |

**This is the one stack that wants port 80 and 443.** It is an edge proxy, so
that is the point — but it cannot run at the same time as anything else binding
those ports. Every other stack in the catalog deliberately avoids them.

## First run

There are **no default credentials**. On first load the UI asks you to create
the admin account.

Older guides — and older versions of the software — tell you to log in with
`admin@example.com` / `changeme`. That has not been true for some time; on
2.15 those credentials are simply rejected, because no user exists until you
make one. You can confirm the state at any time:

```
curl -s http://localhost:81/api/          # {"status":"OK","setup":false,...}
```

`setup: false` means no admin account exists yet.

## Proxying your other stacks

Containers can only reach each other on a shared Docker network. To put a
service behind this proxy, attach it to the proxy's network and add a proxy
host pointing at the **container name and container port** — not `localhost`
and not the published host port.

```yaml
# in the other stack's compose.yaml
networks:
  default:
    name: spinup-nginx-proxy-manager_default
    external: true
```

Then in the UI: *Hosts → Proxy Hosts → Add*, forwarding to e.g. hostname
`spinup-nginx-static-nginx-1`, port `80`.

## Data

The proxy database and issued certificates live in the `npm-data` and
`npm-letsencrypt` named volumes. The old compose file bind-mounted `./data` and
`./letsencrypt`, which wrote certificates and a SQLite database straight into
the repo folder.

`spin down` keeps them; `spin destroy` deletes them — including every
certificate you have issued.

## Gotchas

- The old file was named `docker-compsoe.yml` (misspelled) and its first line
  was a pasted `GNU nano 7.2 …` header, which made it invalid YAML — Compose
  refused to parse it at all.
- It also ran a second `frontend` service off a personal Docker Hub image. That
  belongs in a user's own project, not in a shared catalog stack, so it is gone;
  the section above shows how to attach your own services instead.
- Let's Encrypt cannot issue certificates for `localhost` or a private IP. Real
  certificates need a public DNS name pointing at a reachable host on port 80.
- On Linux, binding ports 80 and 443 may require extra privileges. Change
  `NPM_HTTP_PORT` / `NPM_HTTPS_PORT` if that is a problem locally.
