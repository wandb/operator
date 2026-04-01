# Infrastructure Connection Settings

All Go URL spec components are parsed by the shared `connectors.ParseConnectionString()` into `connectors.ConnectionInfo`.
Query params are parsed per-connector via `queryparams.ParseAndValidate()` into typed structs (e.g., `RedisQueryParams`).
Weave-python env vars are defined in `services/weave-python/weave-public/weave/trace_server/environment.py`.

| Infra | Component | Setting | Type | Default | Notes |
|-------|-----------|---------|------|---------|-------|
| **Redis** | `connectors.ConnectionInfo`<br>`weave-python env` | `scheme`<br>`WEAVE_REDIS_URL.Scheme` | string | `redis` | Only `redis://` supported; TLS via query param |
| Redis | `connectors.ConnectionInfo`<br>`weave-python env` | `host`<br>`WEAVE_REDIS_URL.Host` | string | — | Required |
| Redis | `connectors.ConnectionInfo`<br>`weave-python env` | `port`<br>`WEAVE_REDIS_URL.Port` | int | — | Typically 6379; 26379 for Sentinel |
| Redis | `connectors.ConnectionInfo`<br>`weave-python env` | `username`<br>`WEAVE_REDIS_URL.Username` | string | — | Extracted but typically ignored |
| Redis | `connectors.ConnectionInfo`<br>`weave-python env` | `password`<br>`WEAVE_REDIS_URL.Password` | string | — | From userinfo |
| Redis | `connectors.RedisQueryParams` | `tls` | bool | false | Enables TLS (min TLS 1.2) |
| Redis | `connectors.RedisQueryParams` | `caCertPath` | string | — | Path to CA certificate file |
| Redis | `connectors.RedisQueryParams` | `ttlInSeconds` | int64 | 0 | Cache TTL |
| Redis | `connectors.RedisQueryParams` | `master` | string | — | Sentinel master name; enables failover mode |
| Redis | `connectors.RedisQueryParams` | `cluster` | bool | false | Enables Redis Cluster mode |
| Redis | `connectors.RedisQueryParams` | `poolSize` | int | 0 (auto) | Connection pool size |
| Redis | `connectors.RedisQueryParams` | `poolSizeMultiple` | int | — | Multiple of GOMAXPROCS |
| Redis | `connectors.RedisQueryParams` | `minIdleConns` | int | 0 | Minimum idle connections |
| Redis | `connectors.RedisQueryParams` | `dialTimeout` | duration | — | Dial timeout |
| Redis | `connectors.RedisQueryParams` | `readTimeout` | duration | — | Read timeout |
| Redis | `connectors.RedisQueryParams` | `writeTimeout` | duration | — | Write timeout |
| Redis | `connectors.RedisQueryParams` | `contextTimeoutEnabled` | bool | — | Enable context-based timeouts |
| Redis | `connectors.RedisQueryParams` | `poolTimeout` | duration | — | Pool wait timeout |
| Redis | `connectors.RedisQueryParams` | `connMaxIdleTime` | duration | — | Max idle connection lifetime |
| Redis | `connectors.RedisQueryParams` | `connMaxLifetime` | duration | — | Max connection lifetime |
| Redis | `connectors.RedisQueryParams` | `connMaxLifetimeJitter` | duration | — | Jitter for connection lifetime |
| Redis | `connectors.RedisQueryParams` | `maxRetries` | int | — | Max retry attempts |
| Redis | `connectors.RedisQueryParams` | `dialerRetries` | int | — | Dialer-specific retries |
| Redis | `connectors.RedisQueryParams` | `maxConcurrentDials` | int | — | Concurrent dial limit |
| Redis | `connectors.RedisQueryParams` | `dialerRetryTimeout` | duration | — | Timeout for dialer retries |
| Redis | `connectors.RedisQueryParams` | `minRetryBackoff` | duration | — | Min exponential backoff |
| Redis | `connectors.RedisQueryParams` | `maxRetryBackoff` | duration | — | Max exponential backoff |
| Redis | `connectors.RedisQueryParams` | `minDialerRetryBackoff` | duration | — | Min dialer backoff |
| Redis | `connectors.RedisQueryParams` | `maxDialerRetryBackoff` | duration | — | Max dialer backoff |
| **MySQL** | `connectors.ConnectionInfo` | `scheme` | string | `mysql` | Also: `mysql-replica`, `cloudsql`, `cloudsql-replica` |
| MySQL | `connectors.ConnectionInfo` | `host` | string | — | Required |
| MySQL | `connectors.ConnectionInfo` | `port` | int | 3306 | — |
| MySQL | `connectors.ConnectionInfo` | `username` | string | — | DB user |
| MySQL | `connectors.ConnectionInfo` | `password` | string | — | DB password |
| MySQL | `connectors.ConnectionInfo` | `path (database)` | string | — | Database name |
| MySQL | `connectors.MySQLQueryParams` | `tls` | string | — | `true`, `false`, `skip-verify`, `preferred`, `custom` |
| MySQL | `connectors.MySQLQueryParams` | `ssl-ca` | string | — | CA cert path (required for tls=custom) |
| MySQL | `connectors.MySQLQueryParams` | `ssl-cert` | string | — | Client cert path (optional for tls=custom) |
| MySQL | `connectors.MySQLQueryParams` | `ssl-key` | string | — | Client key path (optional for tls=custom) |
| **ClickHouse** | `connectors.ConnectionInfo` | `scheme` | string | `clickhouse` | Also: `clickhouse-sql` |
| ClickHouse | `connectors.ConnectionInfo`<br>`weave-python env` | `host`<br>`WF_CLICKHOUSE_HOST` | string | `localhost` | Required |
| ClickHouse | `connectors.ConnectionInfo`<br>`weave-python env` | `port`<br>`WF_CLICKHOUSE_PORT` | int | 9000 / 8123 | Go: 9000 native, 8123 HTTP; Python default: 8123 |
| ClickHouse | `connectors.ConnectionInfo`<br>`weave-python env` | `username`<br>`WF_CLICKHOUSE_USER` | string | `default` | DB user |
| ClickHouse | `connectors.ConnectionInfo`<br>`weave-python env` | `password`<br>`WF_CLICKHOUSE_PASS` | string | `""` | DB password |
| ClickHouse | `connectors.ConnectionInfo`<br>`weave-python env` | `path (database)`<br>`WF_CLICKHOUSE_DATABASE` | string | `default` | Database name |
| ClickHouse | `connectors.ClickhouseQueryParams` | `tls` | bool | true | Enables TLS (min TLS 1.3); Python auto-detects via port 8443 |
| ClickHouse | `connectors.ClickhouseQueryParams` | `max_idle_conns` | int | 5 | Max idle connections |
| ClickHouse | `connectors.ClickhouseQueryParams` | `max_open_conns` | int | idle+5 | Max open connections |
| ClickHouse | `connectors.ClickhouseQueryParams` | `conn_max_lifetime` | duration | 10m | Max connection lifetime |
| ClickHouse | `connectors.ClickhouseQueryParams` | `dial_timeout` | duration | 30s | Connection timeout |
| ClickHouse | `connectors.ClickhouseQueryParams` | `client_name` | string | `megabinary` | Client identifier |
| ClickHouse | `connectors.ClickhouseQueryParams` | `protocol` | string | `native` | `native` or `http` |
| ClickHouse | `connectors.ClickhouseQueryParams` | `fail-fast` | bool | false | Fail fast on connection errors |
| ClickHouse | `connectors.ClickhouseQueryParams` | `auto-create` | bool | true | Auto-create database if missing |
| ClickHouse | `weave-python env` | `WF_CLICKHOUSE_REPLICATED` | string | `false` | Enable replication |
| ClickHouse | `weave-python env` | `WF_CLICKHOUSE_REPLICATED_PATH` | string | None | ZooKeeper path for replicated tables |
| ClickHouse | `weave-python env` | `WF_CLICKHOUSE_REPLICATED_CLUSTER` | string | None | Cluster name for replicated tables |
| ClickHouse | `weave-python env` | `WF_CLICKHOUSE_USE_DISTRIBUTED_TABLES` | string | `false` | Enable distributed tables |
| ClickHouse | `weave-python env` | `WF_CLICKHOUSE_CALLS_SHARD_KEY` | string | `trace_id` | Shard key (trace_id, id, or project_id) |
| ClickHouse | `weave-python env` | `WF_CLICKHOUSE_MAX_MEMORY_USAGE` | string | None | Max memory per query |
| ClickHouse | `weave-python env` | `WF_CLICKHOUSE_MAX_EXECUTION_TIME` | string | None | Max query execution time |
| ClickHouse | `weave-python env` | `WF_CLICKHOUSE_ASYNC_INSERT_BUSY_TIMEOUT_MIN_MS` | int | 100 | Async insert min busy timeout (ms) |
| ClickHouse | `weave-python env` | `WF_CLICKHOUSE_ASYNC_INSERT_BUSY_TIMEOUT_MAX_MS` | int | 1000 | Async insert max busy timeout (ms) |
| ClickHouse | `weave-python env` | `WEAVE_TRACE_CLICKHOUSE_USE_ASYNC_INSERT` | string | `true` | Enable async inserts |
| ClickHouse | `weave-python env` | `WF_CLICKHOUSE_DISABLE_LIGHTWEIGHT_UPDATE` | string | `false` | Disable lightweight UPDATE/DELETE |
| **S3/Minio** | `connectors.ConnectionInfo` | `scheme` | string | `s3` | Also: `cw` for CoreWeave |
| S3/Minio | `connectors.ConnectionInfo` | `host` | string | — | Empty for AWS default; set for Minio/CW |
| S3/Minio | `connectors.ConnectionInfo` | `port` | int | — | Typically 9000 for Minio |
| S3/Minio | `connectors.ConnectionInfo`<br>`weave-python env` | `username`<br>`WF_FILE_STORAGE_AWS_ACCESS_KEY_ID` | string | None | AWS Access Key ID |
| S3/Minio | `connectors.ConnectionInfo`<br>`weave-python env` | `password`<br>`WF_FILE_STORAGE_AWS_SECRET_ACCESS_KEY` | string | None | AWS Secret Access Key |
| S3/Minio | `connectors.ConnectionInfo` | `path (bucket)` | string | — | Bucket name + optional sub-path |
| S3/Minio | `connectors.S3QueryParams` | `tls` | bool | varies | Depends on service type detection |
| S3/Minio | `connectors.S3QueryParams` | `forcePathStyle` | bool | varies | false for AWS, true for S3-compat |
| S3/Minio | `connectors.S3QueryParams`<br>`weave-python env` | `region`<br>`WF_FILE_STORAGE_AWS_REGION` | string | `us-west-1` | AWS region override |
| S3/Minio | `weave-python env` | `WF_FILE_STORAGE_URI` | string | None | Storage bucket URI |
| S3/Minio | `weave-python env` | `WF_FILE_STORAGE_AWS_SESSION_TOKEN` | string | None | AWS temporary session token |
| S3/Minio | `weave-python env` | `WF_FILE_STORAGE_AWS_KMS_KEY` | string | None | KMS key ID for encryption |
| **Kafka** | `connectors.ConnectionInfo` | `scheme` | string | `kafka` | — |
| Kafka | `connectors.ConnectionInfo`<br>`weave-python env` | `host`<br>`KAFKA_BROKER_HOST` | string | `localhost` | Broker host |
| Kafka | `connectors.ConnectionInfo`<br>`weave-python env` | `port`<br>`KAFKA_BROKER_PORT` | int | 9092 | Broker port |
| Kafka | `connectors.ConnectionInfo`<br>`weave-python env` | `username`<br>`KAFKA_CLIENT_USER` | string | None | SASL_PLAIN username |
| Kafka | `connectors.ConnectionInfo`<br>`weave-python env` | `password`<br>`KAFKA_CLIENT_PASSWORD` | string | None | SASL_PLAIN password |
| Kafka | `connectors.ConnectionInfo` | `path (topic)` | string | — | Topic name (slashes become dots) |
| Kafka | `connectors.KafkaQueryParams` | `consumer_group_id` | string | — | Consumer group ID |
| Kafka | `connectors.KafkaQueryParams` | `min_bytes` | int32 | 1 | Consumer fetch min bytes |
| Kafka | `connectors.KafkaQueryParams` | `max_bytes` | int32 | 10MB | Consumer fetch max bytes |
| Kafka | `connectors.KafkaQueryParams` | `max_wait` | duration | 200ms | Consumer fetch max wait |
| Kafka | `connectors.KafkaQueryParams` | `request_timeout` | duration | 1s | Producer request timeout |
| Kafka | `connectors.KafkaQueryParams` | `linger` | duration | 10ms | Producer linger time |
| Kafka | `connectors.KafkaQueryParams` | `producer_batch_max_bytes` | int32 | 15MB | Producer max batch size |
| Kafka | `connectors.KafkaQueryParams` | `num_partitions` | int | 1 | Topic partition count |
| Kafka | `weave-python env` | `KAFKA_PRODUCER_MAX_BUFFER_SIZE` | string | None | Producer buffer size limit |
| Kafka | `weave-python env` | `KAFKA_PARTITION_BY_PROJECT_ID` | string | `false` | Partition by project ID |
