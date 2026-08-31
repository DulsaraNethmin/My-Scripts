# monitoring

[Prometheus](https://prometheus.io/) and [Grafana](https://grafana.com/), with
[node-exporter](https://github.com/prometheus/node_exporter) for the host and
[cAdvisor](https://github.com/google/cadvisor) for every container on the
daemon — including the other spinup stacks. A dashboard is provisioned, so
`--gui` opens on graphs rather than on a setup wizard.

```
spinup up monitoring              # Prometheus on http://localhost:9090
spinup up monitoring --gui        # + Grafana on http://localhost:8095
spinup open monitoring            # Grafana, already pointed at Prometheus
```

## Ports

| Service       | Host port              | Container | Env var           |
|---------------|------------------------|-----------|-------------------|
| Prometheus    | `9090`                 | 9090      | `PROMETHEUS_PORT` |
| Grafana       | `8095` (`gui` profile) | 3000      | `GRAFANA_PORT`    |
| node-exporter | *not published*        | 9100      | —                 |
| cAdvisor      | *not published*        | 8080      | —                 |

The two exporters are scraped by Prometheus over the stack's own network. A
host port for each would be two more numbers to collide with and nothing to
look at — their output is a page of Prometheus text.

## Credentials

| What            | Default  |
|-----------------|----------|
| Grafana user    | `spinup` |
| Grafana password| `spinup` |

Prometheus has no authentication, which is how it ships.

The Grafana account is created on the first start and then lives in the
`grafana-data` volume; changing `GRAFANA_PASSWORD` afterwards does nothing.
Change it in the UI, or `spinup destroy monitoring`.

## What is provisioned

- **Data source** — Prometheus at `http://prometheus:9090`, set as the
  default, with the fixed uid `spinup-prometheus`.
- **Dashboard** — *Docker & host*: host CPU, memory and disk from
  node-exporter; per-container CPU, memory and network from cAdvisor.

Both come from files, not from Grafana's database:

```
grafana/provisioning/datasources/prometheus.yml
grafana/provisioning/dashboards/spinup.yml     # the provider
grafana/dashboards/*.json                      # every file here is a dashboard
```

Drop another dashboard JSON into `grafana/dashboards/` and it appears within
thirty seconds — no restart. That is also how to import one from
[grafana.com/dashboards](https://grafana.com/grafana/dashboards/): download
the JSON, save it there, and set its data source uid to `spinup-prometheus`.

## Scrape targets

`prometheus/prometheus.yml` holds them, addressed by service name:

```yaml
- job_name: node
  static_configs: [{ targets: ["node-exporter:9100"] }]
```

To scrape your own app, add a job and put the app's container on this stack's
network (`spinup-monitoring_default`). `spinup restart monitoring` reloads the
file; `--web.enable-lifecycle` is on, so `curl -XPOST
localhost:9090/-/reload` works too.

Prometheus's own UI at `http://localhost:9090` is the fastest way to see
whether a target is up — Status → Target health.

## Container names, and Docker Desktop

On **Linux**, cAdvisor reaches containerd and labels every series with the
container's `name` and `image`, so the dashboard's legends read
`spinup-postgres-postgres-1`.

On **Docker Desktop** (macOS and Windows) it cannot: containerd's socket lives
inside the VM and is not on the host to be mounted. cAdvisor still reports
every container's CPU, memory and network — it just knows them by cgroup id
rather than by name. The provisioned dashboard runs both queries in every
container panel, so it has data either way; the legends are just container ids
rather than names on Docker Desktop.

This is a limitation of what the platform exposes, not something the stack can
configure around. If names matter to you on a Mac, `spinup ps` and Portainer
both have them.

## Storage

`prometheus-data` holds the time series, `grafana-data` the accounts and
anything edited in the UI. `spinup down monitoring` keeps both;
`spinup destroy monitoring` deletes them.

`PROMETHEUS_RETENTION` defaults to 15 days, which for these two exporters is a
few hundred megabytes.

## Notes

- Grafana is a container of its own, so it lives behind the `gui` profile and
  waits for Prometheus to be healthy. Prometheus, node-exporter and cAdvisor
  start either way — the stack keeps collecting whether or not anything is
  looking.
- Prometheus's image is distroless: no shell, no curl, no wget. The
  healthcheck is `promtool check healthy`, the binary that ships beside it.
- cAdvisor runs privileged and mounts `/sys`, `/var/lib/docker` and the Docker
  socket read-only. That is what reading every container's cgroups costs, and
  it is a good reason to keep this stack on your own machine.
- The dashboard is `editable: true` and the provider allows UI updates, so you
  can change it in Grafana — but the file wins on the next reload. Save
  anything you want to keep back into `grafana/dashboards/`.
