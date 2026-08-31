# mongodb

MongoDB 7 with [mongo-express](https://github.com/mongo-express/mongo-express) as
the optional web GUI.

```
spin up mongodb             # database only
spin up mongodb --gui       # database + mongo-express
spin url mongodb            # print the connection string
spin cli mongodb            # mongosh, inside the container
```

## Ports

| Service       | Host port              | Container | Env var              |
|---------------|------------------------|-----------|----------------------|
| mongodb       | `27017`                | 27017     | `MONGO_PORT`         |
| mongo-express | `8082` (`gui` profile) | 8081      | `MONGO_EXPRESS_PORT` |

## Credentials

There are **two separate logins** here, which trips people up:

| What                     | Default  |
|--------------------------|----------|
| MongoDB root user        | `spinup` |
| MongoDB root password    | `spinup` |
| Database                 | `spinup` |
| mongo-express UI login    | `spinup` |
| mongo-express UI password | `spinup` |

```
mongodb://spinup:spinup@localhost:27017/spinup?authSource=admin
```

`authSource=admin` is required — the root user lives in the `admin` database,
not in `spinup`. Leave it off and authentication fails with a confusing error.

Development defaults for a local machine; change them with
`spin env mongodb --edit`.

## Seeding

Anything in `init/` — `.js` or `.sh` — runs once, in alphabetical order, the
first time the data volume is created. JavaScript files execute against
`MONGO_DB` via `mongosh`:

```js
// stacks/mongodb/init/01-seed.js
db.users.insertMany([{ name: "ada" }, { name: "grace" }]);
```

To reseed, `spin destroy mongodb` then `spin up mongodb`.

## Gotchas

- Mongo creates databases **lazily**. `MONGO_DB` will not show up in
  `show dbs` until something is actually written to it — this is normal, not a
  failed start.
- `MONGO_USER` / `MONGO_PASSWORD` are only read when the data volume is empty.
  Editing them later does nothing until you destroy the volume.
- mongo-express 1.x puts its entire UI behind HTTP basic auth, configured
  separately from the MongoDB credentials. An unauthenticated request correctly
  returns 401.
- The stack keeps two volumes: `mongodb-data` for your data and
  `mongodb-config` for Mongo's own metadata. `spin destroy` removes both.
- `spin down` keeps your data; only `spin destroy` deletes it.
