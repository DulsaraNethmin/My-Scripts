# kafka

[Apache Kafka](https://kafka.apache.org/) 4 in KRaft mode — one container, no
ZooKeeper — with [kafka-ui](https://github.com/kafbat/kafka-ui) as the optional
web GUI.

```
spinup up kafka                  # broker on localhost:9092
spinup up kafka --gui            # + kafka-ui on http://localhost:8093
spinup cli kafka                 # lists the topics
spinup shell kafka               # the container, with bin/ on PATH
```

## Ports

| Service  | Host port              | Container | Env var         |
|----------|------------------------|-----------|-----------------|
| Broker   | `9092`                 | 29092     | `KAFKA_PORT`    |
| kafka-ui | `8093` (`gui` profile) | 8080      | `KAFKA_UI_PORT` |

## Credentials

None. `PLAINTEXT` on both listeners, no SASL, no TLS — which is what you want
locally and never what you want anywhere else.

## Two listeners, and why

A Kafka client does not keep talking to the address it bootstrapped against.
It asks the cluster for metadata and gets back the address each broker
*advertises*, then reconnects there. So the advertised address has to be
correct from where the client is standing, and "where the client is standing"
has two answers here:

| Client                          | Bootstrap with  | Listener         |
|---------------------------------|-----------------|------------------|
| On your machine                 | `localhost:9092`| `PLAINTEXT_HOST` |
| In a container on this network  | `kafka:9092`    | `PLAINTEXT`      |

That is the whole reason for the two-listener setup, and the single most
common way a local Kafka goes wrong: bootstrap succeeds, the client is handed
an address it cannot reach, and everything then times out with no obvious
cause. If you override `KAFKA_PORT`, the advertised host address follows it.

## Using it

```
spinup cli kafka                                          # list topics
spinup shell kafka
  ./bin/kafka-topics.sh --bootstrap-server 127.0.0.1:9092 --create --topic demo
  ./bin/kafka-console-producer.sh --bootstrap-server 127.0.0.1:9092 --topic demo
  ./bin/kafka-console-consumer.sh --bootstrap-server 127.0.0.1:9092 --topic demo --from-beginning
```

From your own code, `bootstrap.servers=localhost:9092`.

Producing to a topic that does not exist creates it, which is convenient and
is also why a typo in a topic name is silently a new topic. Set
`KAFKA_AUTO_CREATE_TOPICS=false` to have the broker refuse instead.

## Storage

`kafka-data` holds the log segments and the KRaft metadata log. `spinup down
kafka` keeps them, `spinup destroy kafka` deletes them — and deleting them
resets the cluster id, so a consumer group's committed offsets go with it.

## Notes

- KRaft, not ZooKeeper: this node is both broker and controller, which is what
  makes a single-node Kafka one container. ZooKeeper was removed outright in
  Kafka 4.
- Every replication factor is pinned to 1. One node cannot replicate anywhere,
  and the built-in topics default to asking for three copies, which leaves the
  broker up but unable to create a consumer group.
- `group.initial.rebalance.delay.ms` is 0 rather than the three-second
  default. That delay exists to batch up a fleet of consumers starting at
  once; locally it is three seconds of nothing on every consumer start.
- The healthcheck runs the broker's own `kafka-broker-api-versions.sh`, which
  proves it answers the protocol rather than merely holding the port open. It
  starts a JVM, so the interval is 15s rather than the catalog's usual 10s.
- kafka-ui polls the cluster for metadata rather than watching it, so a topic
  you have just created takes a few seconds to appear in the sidebar. It is
  not broken; refresh once.
- kafka-ui is [kafbat's fork](https://github.com/kafbat/kafka-ui), which is
  the maintained one. The `provectuslabs/kafka-ui` image most search results
  still point at has not had a release since 2023.
