# minio

[MinIO](https://min.io/) is S3-compatible object storage: the same API as
Amazon S3, on your machine, with a web console for poking at buckets.

```
spinup up minio               # S3 on :9000, console on http://localhost:8086
spinup open minio             # the console
spinup cli minio              # mc ls local — the buckets, from inside
```

## Ports

| Service | Host port | Container | Env var              |
|---------|-----------|-----------|----------------------|
| S3 API  | `9000`    | 9000      | `MINIO_PORT`         |
| Console | `8086`    | 9001      | `MINIO_CONSOLE_PORT` |

## Credentials

| What     | Default         |
|----------|-----------------|
| User     | `spinup`        |
| Password | `spinup-secret` |

Not `spinup`: MinIO refuses to start with a root password shorter than eight
characters. The same pair is the console login and the S3 access key/secret.

## Connecting an SDK

```
AWS_ACCESS_KEY_ID=spinup
AWS_SECRET_ACCESS_KEY=spinup-secret
AWS_ENDPOINT_URL=http://localhost:9000
AWS_REGION=us-east-1
```

Almost every client needs **path-style addressing** — `boto3` with
`s3={'addressing_style': 'path'}`, the AWS CLI with `--endpoint-url`, or
`forcePathStyle: true` in the JS SDK. Virtual-host style would resolve
`bucket.localhost`, which nothing does.

```sh
aws --endpoint-url http://localhost:9000 s3 mb s3://dev
aws --endpoint-url http://localhost:9000 s3 cp ./file s3://dev/
```

From another container the endpoint is `http://host.docker.internal:9000`.

## Storage

Objects live in the `minio-data` volume, so `spinup down minio` keeps them and
`spinup destroy minio` deletes them. There is no bucket by default — make one
in the console, with `mc`, or with your SDK's create-bucket call.

## Notes

- The console is a second listener inside the same container, so this stack has
  no `gui` profile: there is nothing separate to start.
- The image tag is a MinIO release date rather than a version number; that is
  how MinIO ships. Upgrading means moving to a newer `RELEASE.…` tag.
- `mc` inside the container has a `local` alias pointing at the server. The
  image leaves it anonymous — enough for the healthcheck, not enough to list a
  bucket — so `compose.yaml` gives it credentials through `MC_HOST_local`. If
  you change the root password to something containing `@`, `:`, `/` or `?`,
  percent-encode it there: that variable is a URL.
