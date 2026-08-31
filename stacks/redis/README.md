# redis

Redis 7 with [RedisInsight](https://redis.io/insight/) as the optional web GUI.

```
spin up redis               # redis only
spin up redis --gui         # redis + RedisInsight
spin url redis              # print the connection string
spin cli redis              # redis-cli, inside the container
```

## Ports

| Service      | Host port              | Container | Env var             |
|--------------|------------------------|-----------|---------------------|
| redis        | `6379`                 | 6379      | `REDIS_PORT`        |
| redisinsight | `8083` (`gui` profile) | 5540      | `REDISINSIGHT_PORT` |

## Credentials

| What     | Default  |
|----------|----------|
| Password | `spinup` |

```
redis://:spinup@localhost:6379
```

There is no username — Redis's classic `requirepass` auth is password-only, so
the connection string has an empty user before the colon.

RedisInsight needs no login of its own, and the `redis` service is pre-registered
in it, so the UI opens straight onto a live connection.

## Persistence

The stock Redis image keeps everything in memory and the old stack declared no
volume at all, so every stop lost the whole dataset. This stack runs with
`--appendonly yes` and a `redis-data` volume, so writes survive restarts.

`spin down` keeps your data; only `spin destroy` deletes it.

If you would rather have a pure in-memory cache — faster, and closer to how
Redis is often used in production behind a real database — drop `--appendonly yes`
from the `command:` in `compose.yaml`.

## Gotchas

- Unlike the SQL stacks, `REDIS_PASSWORD` is re-read on **every** start, because
  it is a server flag rather than first-boot initialisation. Changing it takes
  effect immediately with no need to destroy the volume.
- Requiring a password is a deliberate default here. Redis with no auth on
  `0.0.0.0` is one of the most commonly compromised dev services on the
  internet. To opt out, remove `--requirepass` from the `command:` — but only
  do that if you are certain the port is not reachable from your network.
- `redis-cli` prints a warning when given `-a` on the command line;
  `--no-auth-warning` (already in the `spin cli` command) silences it.
- The plan named `redis/redisinsight:2`, which does not exist. RedisInsight is
  now on 3.x and the image exposes port **5540**, not the 8001 used by the
  discontinued v1 in the old stack.
