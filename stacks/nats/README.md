# nats

[NATS](https://nats.io/) 2 with JetStream on: subjects for plain pub/sub, and
streams, key-value and object stores that survive a restart.

```
spinup up nats                # nats://localhost:4222
spinup url nats               # the connection string
curl localhost:8088/varz      # server stats
curl localhost:8088/healthz   # what the healthcheck asks
```

## Ports

| Service    | Host port | Container | Env var             |
|------------|-----------|-----------|---------------------|
| Client     | `4222`    | 4222      | `NATS_PORT`         |
| Monitoring | `8088`    | 8222      | `NATS_MONITOR_PORT` |

## Credentials

None. The server accepts any client, which is what you want locally and never
what you want anywhere else. If you need auth, add `--user`/`--pass` or an
`--auth` token to the `command:` in `compose.yaml`.

## Connecting

```
nats://localhost:4222
```

From another container, `nats://host.docker.internal:4222`.

The `nats` CLI is not in this image — it is a separate download — so there is
no `spinup cli nats`. Once you have it:

```
nats --server localhost:4222 pub greet.joe 'hello'
nats --server localhost:4222 sub 'greet.*'
nats --server localhost:4222 stream ls
```

## Storage

JetStream writes to the `nats-data` volume (`--store_dir=/data`), so streams
and key-value buckets survive `spinup down nats` and are deleted by
`spinup destroy nats`. Plain pub/sub keeps nothing by design: a message with
no subscriber is gone.

## Notes

- The monitoring port serves JSON, not a web UI, so this stack declares no GUI
  and `spinup open nats` has nothing to open. `/varz`, `/connz`, `/subsz` and
  `/jsz` are the interesting ones.
- The `nats:2-alpine` tag rather than `nats:2`: the default image is built on
  scratch and has no shell or wget, which leaves nothing to run a healthcheck
  with.
