# Configure backing infrastructure

MySQL, Redis, object storage, and ClickHouse use maps keyed by instance name.
Include a `default` entry when supplying a map; it is the fallback for manifest
applications that do not select another configured instance. Kafka has a single
managed configuration in this API.

| Service | Managed field | External field |
| --- | --- | --- |
| MySQL | `spec.mysql.default.managedMysql` | `spec.mysql.default.externalMysql` |
| Redis | `spec.redis.default.managedRedis` | `spec.redis.default.externalRedis` |
| Object storage | `spec.objectStore.default.managedObjectStore` | `spec.objectStore.default.externalObjectStore` |
| ClickHouse | `spec.clickhouse.default.managedClickhouse` | `spec.clickhouse.default.externalClickhouse` |
| Kafka | `spec.kafka.managedKafka` | Not exposed; Kafka is managed through Bufstream |

Choose either managed or external configuration for each instance. Omitting a
service defaults it to managed. Connection changes do not copy data between
services. When adapting an existing live spec from managed to external, remove
the corresponding managed block explicitly and plan data ownership before the
change. See [retention](retention.md) and [migration](migrating-v1-to-v2.md).

## Managed services

Operator supplies managed defaults from the size profile. Override only the
settings needed for your environment, for example:

```yaml
spec:
  mysql:
    default:
      managedMysql:
        storageSize: 100Gi
        replicas: 3
```

Moco replicas must be a positive odd number. The supported storage and replica
changes depend on the backing controller and storage provider. Managed
ClickHouse stores table data in object storage; its local `storageSize` covers
metadata, system tables, and read cache. Include the object store in backup and
capacity planning.

## External MySQL and Redis

Create Secrets `wandb-mysql` and `wandb-redis` with a `password` key in the W&B
namespace, then merge this fragment into the deployment resource:

```yaml
spec:
  mysql:
    default:
      externalMysql:
        host: {value: mysql.example.com}
        port: {value: "3306"}
        database: {value: wandb}
        username: {value: wandb}
        password:
          valueFrom:
            secretKeyRef: {name: wandb-mysql, key: password}
        tls: {value: "true"}
  redis:
    default:
      externalRedis:
        host: {value: redis.example.com}
        port: {value: "6379"}
        password:
          valueFrom:
            secretKeyRef: {name: wandb-redis, key: password}
        tls: {value: "true"}
```

The endpoints in this example must support TLS. Both connections expose `sslCa`
for custom CA material; MySQL also exposes `sslCert` and `sslKey`. Use Secret
references for private keys and configure the server's corresponding TLS policy.

## External object storage

For an S3 bucket, create a Secret named `wandb-s3` with `accessKey` and `secretKey`
keys and supply the bucket and region:

```yaml
spec:
  objectStore:
    default:
      externalObjectStore:
        provider: {value: s3}
        bucket: {value: my-wandb-bucket}
        region: {value: us-east-1}
        accessKey:
          valueFrom:
            secretKeyRef: {name: wandb-s3, key: accessKey}
        secretKey:
          valueFrom:
            secretKeyRef: {name: wandb-s3, key: secretKey}
        tlsEnabled: {value: "true"}
        forcePathStyle: {value: "false"}
```

For an S3-compatible endpoint, also set `endpoint` and, when needed, `port`;
set `forcePathStyle` and `tlsEnabled` to match that service. An optional `path`
sets a prefix within the bucket. The API also accepts provider values `gcs` and
`azure`; credential and workload-identity setup must match the provider and the
consuming W&B components. Provider-specific deployment recipes remain in the
[documentation backlog](../developer/documentation.md#remaining-coverage).

When W&B returns direct presigned URLs, browsers must resolve and reach the
storage hostname, trust its certificate, and satisfy the bucket's CORS policy.
Verify both SDK transfers and a browser download. An internal Kubernetes Service
address alone is insufficient for external browser access.

## External ClickHouse

Create a Secret named `wandb-clickhouse` with a `password` key and a `url` key
containing the complete connection URL. Supply both the URL and individual
connection fields; Operator does not derive the external URL from those fields.
Keep its credentials, host, port, database, and TLS options consistent with the
individual fields and the service configuration.

```yaml
spec:
  clickhouse:
    default:
      externalClickhouse:
        url:
          valueFrom:
            secretKeyRef: {name: wandb-clickhouse, key: url}
        host: {value: clickhouse.example.com}
        tcpPort: {value: "9000"}
        httpPort: {value: "8123"}
        database: {value: wandb}
        username: {value: wandb}
        password:
          valueFrom:
            secretKeyRef: {name: wandb-clickhouse, key: password}
        replicated: {value: "false"}
```

Match the ports and replication settings to the service; when replication is
enabled, configure `clusterName`
as required by that ClickHouse deployment. Port numbers alone do not configure
server-side TLS or replication.

## Component operators and verification

The Helm values `moco.enabled`, `redis-operator.enabled`,
`seaweedfs-operator.enabled`, and `altinity-clickhouse-operator.enabled` control
the bundled controllers. Disabling one also excludes its CRDs from the bundled
installer; it does not turn the W&B service into an external connection. Configure
the CR accordingly, or provide an existing compatible controller and CRDs.

```bash
kubectl get weightsandbiases wandb -n wandb -o yaml
kubectl get pods,pvc -n wandb
```

Inspect `mysqlStatus`, `redisStatus`, `objectStoreStatus`, and `clickhouseStatus`
by instance name, plus `kafkaStatus`. Confirm readiness and connection resolution
before diagnosing dependent migration Jobs and applications.
