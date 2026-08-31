# rabbitmq

[RabbitMQ](https://www.rabbitmq.com/) 4 with the management plugin: an AMQP
broker on `5672` and its web UI on `8087`.

```
spin up rabbitmq            # broker + management UI
spin open rabbitmq          # the UI
spin url rabbitmq           # amqp://spinup:spinup@localhost:5672/
spin cli rabbitmq           # rabbitmqctl list_queues
```

## Ports

| Service    | Host port | Container | Env var                    |
|------------|-----------|-----------|----------------------------|
| AMQP       | `5672`    | 5672      | `RABBITMQ_PORT`            |
| Management | `8087`    | 15672     | `RABBITMQ_MANAGEMENT_PORT` |

## Credentials

| What     | Default  |
|----------|----------|
| User     | `spinup` |
| Password | `spinup` |
| Vhost    | `/`      |

The same user logs into the management UI and connects over AMQP. RabbitMQ's
own `guest` account is not used: it can only connect from localhost *inside*
the container, which is never where your app is.

## Connecting

```
amqp://spinup:spinup@localhost:5672/
```

pika, amqplib, Bunny and Spring AMQP all take that URL as it stands. From
another container it is `host.docker.internal` rather than `localhost`.

## Storage

Queues, exchanges and messages that were published as persistent live in the
`rabbitmq-data` volume, so `spin down rabbitmq` keeps them and
`spin destroy rabbitmq` deletes them.

`compose.yaml` sets a fixed `hostname:`. RabbitMQ names its node after the
hostname and stores data under that name, so without one every recreate would
get a fresh container id, a fresh node name, and a data directory that looks
empty — the messages would still be on the volume, under a name nothing reads.

## Notes

- The management UI is a plugin inside the same container, so this stack has no
  `gui` profile: there is nothing separate to start.
- The broker takes 15–30 seconds to become healthy on first start, which is
  why `spin up` waits rather than returning the moment the container exists.
