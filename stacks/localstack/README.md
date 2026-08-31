# localstack

[LocalStack](https://localstack.cloud/) 4 — S3, SQS, SNS, Lambda, DynamoDB,
Kinesis, IAM and the rest, answering on your machine so tests and local runs
never touch a real AWS account.

```
spinup up localstack                      # http://localhost:4566
spinup cli localstack -- s3 mb s3://demo  # awslocal, inside the container
spinup cli localstack -- s3 ls
curl localhost:4566/_localstack/health    # what is available
```

## Ports

| Service | Host port | Container | Env var           |
|---------|-----------|-----------|-------------------|
| Gateway | `4566`    | 4566      | `LOCALSTACK_PORT` |

Every service is behind that one port — it is the endpoint URL you point an
SDK at, not one port per service.

## Credentials

AWS SDKs refuse to sign a request without credentials, and LocalStack does not
check them. The convention is:

```
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
AWS_DEFAULT_REGION=us-east-1
AWS_ENDPOINT_URL=http://localhost:4566
```

Recent SDKs and the AWS CLI v2 read `AWS_ENDPOINT_URL`, so those four
variables are usually the whole of the setup. Older ones need the endpoint
passed per call (`aws --endpoint-url=http://localhost:4566 s3 ls`).

## Using it

`awslocal` is the AWS CLI with the endpoint already set, and it ships in the
image:

```
spinup cli localstack -- s3 mb s3://demo
spinup cli localstack -- sqs create-queue --queue-name jobs
spinup cli localstack -- dynamodb list-tables
```

From your own shell, with the variables above exported, the plain `aws` CLI
and any SDK work the same way.

## Storage

`localstack-data` holds the working state and the caches LocalStack builds.
Note that it is *not* a save file: in the community edition the resources you
create are gone when the container stops. Persistence across restarts is a Pro
feature (`PERSISTENCE=1`), so treat every `spinup up localstack` as an empty
account and create what a test needs in the test.

## Notes

- The image is pinned to `4`, not `latest`. LocalStack's CalVer tags
  (`latest`, `stable`, `2026.x`) are the Pro image: without a valid
  `LOCALSTACK_AUTH_TOKEN` they print a licence error and quit with exit 55.
  `4` is the last freely usable line.
- The Docker socket is mounted because LocalStack runs each Lambda invocation
  in a container of its own. Without it every service works except Lambda. It
  is also root on the host by another name — run this locally, not on
  anything exposed. On Windows the host side is `//./pipe/docker_engine`; see
  `LOCALSTACK_DOCKER_SOCK`.
- No GUI, so `spinup open localstack` has nothing to open. LocalStack's web
  interface is a hosted service at app.localstack.cloud that needs an account;
  `/_localstack/health` is the local equivalent and answers with JSON.
- Only the gateway port is published. LocalStack can also allocate per-service
  ports in `4510-4559`; publishing fifty ports for a case most setups never
  hit is not worth it, and anything that needs one can add it to
  `compose.yaml`.
