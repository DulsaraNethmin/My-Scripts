# mysql

MySQL 8.4 (LTS) with [phpMyAdmin](https://www.phpmyadmin.net/) as the optional
web GUI.

```
spin up mysql               # database only
spin up mysql --gui         # database + phpMyAdmin
spin url mysql              # print the connection string
spin cli mysql              # mysql client, inside the container
```

## Ports

| Service    | Host port              | Container | Env var            |
|------------|------------------------|-----------|--------------------|
| mysql      | `3306`                 | 3306      | `MYSQL_PORT`       |
| phpmyadmin | `8081` (`gui` profile) | 80        | `PHPMYADMIN_PORT`  |

## Credentials

| What          | Default  |
|---------------|----------|
| Root password | `spinup` |
| User          | `spinup` |
| Password      | `spinup` |
| Database      | `spinup` |

```
mysql://spinup:spinup@localhost:3306/spinup
```

Log into phpMyAdmin as either `spinup` or `root`, both with password `spinup`.
Development defaults for a local machine — change them with
`spin env mysql --edit`, and never expose this stack to an untrusted network.

## Connecting from phpMyAdmin

phpMyAdmin is already pointed at the `mysql` service, so you only supply the
username and password. If you configure another client running in Docker, use
host `mysql` and port `3306` — inside the network, not `localhost` and
`MYSQL_PORT`.

## Seeding

Anything in `init/` — `.sql`, `.sql.gz` or `.sh` — is executed once, in
alphabetical order, the first time the data volume is created. It does **not**
re-run on later starts; `spin destroy mysql` then `spin up mysql` to reseed.

For a dump too large to sit in the repo, import it through phpMyAdmin instead —
`PHPMYADMIN_UPLOAD_LIMIT` is raised to 300M for exactly this.

## Gotchas

- MySQL 8.4 **removed** `--default-authentication-plugin`. The old script passed
  it and MySQL refuses to start with it on 8.4+. Modern clients speak
  `caching_sha2_password` natively; if you must support a very old client, set
  `MYSQL_ROOT_HOST` and use `mysql_native_password` explicitly per user instead.
- `MYSQL_USER` cannot be `root` — that account always exists, and MySQL errors
  out on first boot if you try.
- All `MYSQL_*` values are only read when the data volume is empty. Editing them
  later does nothing until you destroy the volume.
- `spin down` keeps your data; only `spin destroy` deletes it. The old
  `mysql-stop.sh` ran `docker-compose down -v` and silently dropped the database
  on every stop — that is fixed here.
- MySQL takes noticeably longer than Postgres to first become healthy (it builds
  the system tablespace), hence the 30 s `start_period`.
