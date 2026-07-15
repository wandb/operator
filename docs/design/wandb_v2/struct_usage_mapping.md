# Struct Usage Mapping

This document maps struct definitions and their usage (reads/writes) across the wandb operator codebase.

## Legend
- 📦 **Package**: Where structs are defined
- 📝 **Defined**: Struct definition location
- 📖 **Read**: Package/file that reads struct fields
- ✍️ **Write**: Package/file that writes/modifies struct fields or creates instances

---

## 1. API Structs (api/v2/)

### Core CR Types

#### WeightsAndBiases
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:41`
- 📖 **Read by**:
  - `internal/controller/wandb_v2/weightsandbiases_v2_controller.go` - Reads entire resource in reconciler
  - `internal/controller/wandb_v2/*.go` - All infrastructure reconcilers read spec fields
  - `internal/controller/translator/v2/*.go` - Translators read spec to build defaults
  - `internal/model/*.go` - Model layer reads spec for configuration
- ✍️ **Written by**:
  - `internal/controller/wandb_v2/weightsandbiases_v2_controller.go` - Updates status fields

#### WeightsAndBiasesSpec
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:70`
- 📖 **Read by**:
  - `internal/controller/wandb_v2/weightsandbiases_v2_controller.go:111-115` - Reads Redis, Kafka, MySQL, Minio, ClickHouse, Size
  - `internal/controller/translator/v2/*.go` - All translators read from this
  - `internal/model/config.go` - InfraConfigBuilder uses spec fields
- ✍️ **Written by**:
  - User-provided manifests

#### WeightsAndBiasesStatus
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:225`
- 📖 **Read by**:
  - `internal/controller/wandb_v2/weightsandbiases_v2_controller.go:185` - Reads RedisStatus
- ✍️ **Written by**:
  - `internal/controller/wandb_v2/weightsandbiases_v2_controller.go:187` - Writes State
  - `internal/controller/wandb_v2/redis.go:43` - Writes RedisStatus
  - `internal/controller/wandb_v2/kafka.go:43` - Writes KafkaStatus
  - `internal/controller/wandb_v2/mysql.go:43` - Writes MySQLStatus
  - `internal/controller/wandb_v2/minio.go:43` - Writes MinioStatus
  - `internal/controller/wandb_v2/clickhouse.go:43` - Writes ClickHouseStatus

### MySQL Structs

#### MySQLSpec
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:84`
- 📖 **Read by**:
  - `internal/controller/translator/v2/mysql.go` - BuildMySQLSpec(), BuildMySQLDefaults()
- ✍️ **Written by**:
  - `internal/controller/translator/v2/mysql.go` - Creates MySQLSpec{} literals with defaults

#### WBMySQLConfig
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:95`
- 📖 **Read by**:
  - `internal/controller/translator/v2/mysql.go` - Reads for merging with defaults
- ✍️ **Written by**:
  - `internal/controller/translator/v2/mysql.go` - Creates WBMySQLConfig{} literals with defaults

#### WBMySQLStatus
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:279`
- 📖 **Read by**:
  - `internal/controller/wandb_v2/weightsandbiases_v2_controller.go` - Reads status for state aggregation
- ✍️ **Written by**:
  - `internal/controller/wandb_v2/mysql.go:43` - Writes via results.ExtractMySQLStatus()
  - `internal/model/mysql.go` - Creates status from MySQLStatusDetail

#### WBMySQLConnection
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:289`
- 📖 **Read by**:
  - Downstream consumers of status
- ✍️ **Written by**:
  - `internal/model/mysql.go` - Creates from MySQLConnInfo

### Redis Structs

#### WBRedisSpec
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:99`
- 📖 **Read by**:
  - `internal/controller/translator/v2/redis.go` - BuildRedisSpec(), BuildRedisDefaults()
- ✍️ **Written by**:
  - `internal/controller/translator/v2/redis.go` - Creates WBRedisSpec{} literals with defaults

#### WBRedisConfig
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:109`
- 📖 **Read by**:
  - `internal/controller/translator/v2/redis.go` - Reads for merging with defaults
- ✍️ **Written by**:
  - `internal/controller/translator/v2/redis.go` - Creates WBRedisConfig{} literals with defaults

#### WBRedisSentinelSpec
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:113`
- 📖 **Read by**:
  - `internal/controller/translator/v2/redis.go` - BuildRedisSpec()
- ✍️ **Written by**:
  - `internal/controller/translator/v2/redis.go` - Creates WBRedisSentinelSpec{} literals with defaults

#### WBRedisSentinelConfig
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:118`
- 📖 **Read by**:
  - `internal/controller/translator/v2/redis.go` - Reads for merging with defaults
- ✍️ **Written by**:
  - `internal/controller/translator/v2/redis.go` - Creates WBRedisSentinelConfig{} literals with defaults

#### WBRedisStatus
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:305`
- 📖 **Read by**:
  - `internal/controller/wandb_v2/weightsandbiases_v2_controller.go:185` - Reads for state aggregation
- ✍️ **Written by**:
  - `internal/controller/wandb_v2/redis.go:43` - Writes via results.ExtractRedisStatus()
  - `internal/model/redis.go` - Creates status from RedisStatusDetail

#### WBRedisConnection
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:313`
- 📖 **Read by**:
  - Downstream consumers of status
- ✍️ **Written by**:
  - `internal/model/redis.go` - Creates from RedisSentinelConnInfo or RedisStandaloneConnInfo

### Kafka Structs

#### WBKafkaSpec
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:123`
- 📖 **Read by**:
  - `internal/controller/translator/v2/kafka.go` - BuildKafkaSpec(), BuildKafkaDefaults()
- ✍️ **Written by**:
  - `internal/controller/translator/v2/kafka.go` - Creates WBKafkaSpec{} literals with defaults

#### WBKafkaConfig
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:134`
- 📖 **Read by**:
  - `internal/controller/translator/v2/kafka.go` - Reads for merging with defaults
- ✍️ **Written by**:
  - `internal/controller/translator/v2/kafka.go` - Creates WBKafkaConfig{} literals with defaults

#### WBKafkaBackupSpec
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:138`
- 📖 **Read by**:
  - `internal/controller/translator/v2/kafka.go` - Reads backup configuration
- ✍️ **Written by**:
  - `internal/controller/translator/v2/kafka.go` - Creates WBKafkaBackupSpec{} literals with defaults

#### WBKafkaStatus
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:326`
- 📖 **Read by**:
  - `internal/controller/wandb_v2/weightsandbiases_v2_controller.go` - Reads for state aggregation
- ✍️ **Written by**:
  - `internal/controller/wandb_v2/kafka.go:43` - Writes via results.ExtractKafkaStatus()
  - `internal/model/kafka.go` - Creates status from KafkaStatusDetail

#### WBKafkaConnection
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:321`
- 📖 **Read by**:
  - Downstream consumers of status
- ✍️ **Written by**:
  - `internal/model/kafka.go` - Creates from KafkaConnInfo

### Minio Structs

#### WBMinioSpec
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:146`
- 📖 **Read by**:
  - `internal/controller/translator/v2/minio.go` - BuildMinioSpec(), BuildMinioDefaults()
- ✍️ **Written by**:
  - `internal/controller/translator/v2/minio.go` - Creates WBMinioSpec{} literals with defaults

#### WBMinioConfig
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:158`
- 📖 **Read by**:
  - `internal/controller/translator/v2/minio.go` - Reads for merging with defaults
- ✍️ **Written by**:
  - `internal/controller/translator/v2/minio.go` - Creates WBMinioConfig{} literals with defaults

#### WBMinioBackupSpec
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:162`
- 📖 **Read by**:
  - `internal/controller/translator/v2/minio.go` - Reads backup configuration
- ✍️ **Written by**:
  - `internal/controller/translator/v2/minio.go` - Creates WBMinioBackupSpec{} literals with defaults

#### WBMinioStatus
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:335`
- 📖 **Read by**:
  - `internal/controller/wandb_v2/weightsandbiases_v2_controller.go` - Reads for state aggregation
- ✍️ **Written by**:
  - `internal/controller/wandb_v2/minio.go:43` - Writes via results.ExtractMinioStatus()
  - `internal/model/minio.go` - Creates status from MinioStatusDetail

#### WBMinioConnection
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:345`
- 📖 **Read by**:
  - Downstream consumers of status
- ✍️ **Written by**:
  - `internal/model/minio.go` - Creates from MinioConnInfo

### ClickHouse Structs

#### WBClickHouseSpec
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:170`
- 📖 **Read by**:
  - `internal/controller/translator/v2/clickhouse.go` - BuildClickHouseSpec(), BuildClickHouseDefaults()
- ✍️ **Written by**:
  - `internal/controller/translator/v2/clickhouse.go` - Creates WBClickHouseSpec{} literals with defaults

#### WBClickHouseConfig
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:183`
- 📖 **Read by**:
  - `internal/controller/translator/v2/clickhouse.go` - Reads for merging with defaults
- ✍️ **Written by**:
  - `internal/controller/translator/v2/clickhouse.go` - Creates WBClickHouseConfig{} literals with defaults

#### WBClickHouseBackupSpec
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:187`
- 📖 **Read by**:
  - `internal/controller/translator/v2/clickhouse.go` - Reads backup configuration
- ✍️ **Written by**:
  - `internal/controller/translator/v2/clickhouse.go` - Creates WBClickHouseBackupSpec{} literals with defaults

#### WBClickHouseStatus
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:351`
- 📖 **Read by**:
  - `internal/controller/wandb_v2/weightsandbiases_v2_controller.go` - Reads for state aggregation
- ✍️ **Written by**:
  - `internal/controller/wandb_v2/clickhouse.go:43` - Writes via results.ExtractClickHouseStatus()
  - `internal/model/clickhouse.go` - Creates status from ClickHouseStatusDetail

#### WBClickHouseConnection
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:361`
- 📖 **Read by**:
  - Downstream consumers of status
- ✍️ **Written by**:
  - `internal/model/clickhouse.go` - Creates from ClickHouseConnInfo

### Backup Structs

#### WBBackupSpec
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:195`
- 📖 **Read by**:
  - Infrastructure model layers for backup configuration
- ✍️ **Written by**:
  - User-provided manifests

#### WBBackupS3Spec
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:211`
- 📖 **Read by**:
  - Infrastructure model layers for S3 backup configuration
- ✍️ **Written by**:
  - User-provided manifests

#### WBBackupFilesystemSpec
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:218`
- 📖 **Read by**:
  - Infrastructure model layers for filesystem backup configuration
- ✍️ **Written by**:
  - User-provided manifests

#### WBBackupStatus
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:295`
- 📖 **Read by**:
  - Controllers for backup status monitoring
- ✍️ **Written by**:
  - Infrastructure controllers during backup operations

### Common Structs

#### WBStatusDetail
- 📝 **Defined**: `api/v2/weightsandbiases_types.go:273`
- 📖 **Read by**:
  - All status aggregation logic
- ✍️ **Written by**:
  - `internal/model/*.go` - All model layers create StatusDetail instances
  - Embedded in MySQL, Redis, Kafka, Minio, ClickHouse status structs

### Application CR Types

#### Application
- 📝 **Defined**: `api/v2/application_types.go:45`
- 📖 **Read by**:
  - Application controllers (if implemented)
- ✍️ **Written by**:
  - Application controllers

#### ApplicationSpec
- 📝 **Defined**: `api/v2/application_types.go:27`
- 📖 **Read by**:
  - Application controllers
- ✍️ **Written by**:
  - User-provided manifests

#### ApplicationStatus
- 📝 **Defined**: `api/v2/application_types.go:36`
- 📖 **Read by**:
  - Application controllers
- ✍️ **Written by**:
  - Application controllers

---

## 2. Internal Model Structs (internal/model/)

### InfraConfigBuilder
- 📝 **Defined**: `internal/model/config.go:21`
- 📖 **Read by**:
  - `internal/controller/wandb_v2/weightsandbiases_v2_controller.go` - Reads built configuration
  - All infrastructure reconcilers read their respective config fields
- ✍️ **Written by**:
  - `internal/controller/translator/v2/*.go` - Translators populate builder with api/v2 specs that have defaults merged

### MySQL Model Structs

#### MySQLConfig
- 📝 **Defined**: `internal/model/mysql.go:32`
- 📖 **Read by**:
  - `internal/controller/infra/mysql/percona/*.go` - Reads config to build Percona resources
- ✍️ **Written by**:
  - `internal/model/mysql.go` - NewMySQLConfig() creates defaults based on Size, independent of api/v2 spec

#### MySQLConnInfo
- 📝 **Defined**: `internal/model/mysql.go:262`
- 📖 **Read by**:
  - Status reporting logic
- ✍️ **Written by**:
  - `internal/model/mysql.go` - ToConnInfo() creates from MySQLConfig

#### MySQLConnDetail
- 📝 **Defined**: `internal/model/mysql.go:268`
- 📖 **Read by**:
  - Status reporting logic
- ✍️ **Written by**:
  - `internal/model/mysql.go` - ToConnInfo() creates connection details

#### MySQLStatusDetail
- 📝 **Defined**: `internal/model/mysql.go:232`
- 📖 **Read by**:
  - `internal/controller/wandb_v2/mysql.go` - Reads to extract status
- ✍️ **Written by**:
  - `internal/model/mysql.go` - Creates from infrastructure results

#### MySQLSizeConfig
- 📝 **Defined**: `internal/model/mysql.go:124`
- 📖 **Read by**:
  - `internal/model/mysql.go` - Reads for resource sizing
- ✍️ **Written by**:
  - `internal/model/mysql.go` - Created based on Size enum (Small, Medium, Large)

#### MySQLInfraError
- 📝 **Defined**: `internal/model/mysql.go:188`
- 📖 **Read by**:
  - Error handling and status reporting
- ✍️ **Written by**:
  - `internal/model/mysql.go` - Created on configuration errors

### Redis Model Structs

#### RedisConfig
- 📝 **Defined**: `internal/model/redis.go:20`
- 📖 **Read by**:
  - `internal/controller/infra/redis/opstree/*.go` - Reads config to build Redis resources
- ✍️ **Written by**:
  - `internal/model/redis.go` - NewRedisConfig() creates defaults based on Size, independent of api/v2 spec

#### sentinelConfig
- 📝 **Defined**: `internal/model/redis.go:29`
- 📖 **Read by**:
  - `internal/model/redis.go` - Internal sentinel configuration
- ✍️ **Written by**:
  - `internal/model/redis.go` - Created during RedisConfig initialization

#### RedisSentinelConnInfo
- 📝 **Defined**: `internal/model/redis.go:208`
- 📖 **Read by**:
  - Status reporting for sentinel mode
- ✍️ **Written by**:
  - `internal/model/redis.go` - ToConnInfo() creates for sentinel

#### RedisSentinelConnDetail
- 📝 **Defined**: `internal/model/redis.go:223`
- 📖 **Read by**:
  - Status reporting logic
- ✍️ **Written by**:
  - `internal/model/redis.go` - ToConnInfo() creates connection details

#### RedisStandaloneConnInfo
- 📝 **Defined**: `internal/model/redis.go:228`
- 📖 **Read by**:
  - Status reporting for standalone mode
- ✍️ **Written by**:
  - `internal/model/redis.go` - ToConnInfo() creates for standalone

#### RedisStandaloneConnDetail
- 📝 **Defined**: `internal/model/redis.go:233`
- 📖 **Read by**:
  - Status reporting logic
- ✍️ **Written by**:
  - `internal/model/redis.go` - ToConnInfo() creates connection details

#### RedisStatusDetail
- 📝 **Defined**: `internal/model/redis.go:156`
- 📖 **Read by**:
  - `internal/controller/wandb_v2/redis.go` - Reads to extract status
- ✍️ **Written by**:
  - `internal/model/redis.go` - Creates from infrastructure results

#### RedisInfraError
- 📝 **Defined**: `internal/model/redis.go:98`
- 📖 **Read by**:
  - Error handling and status reporting
- ✍️ **Written by**:
  - `internal/model/redis.go` - Created on configuration errors

### Kafka Model Structs

#### KafkaConfig
- 📝 **Defined**: `internal/model/kafka.go:18`
- 📖 **Read by**:
  - `internal/controller/infra/kafka/strimzi/*.go` - Reads config to build Kafka resources
- ✍️ **Written by**:
  - `internal/model/kafka.go` - NewKafkaConfig() creates defaults based on Size, independent of api/v2 spec

#### KafkaReplicationConfig
- 📝 **Defined**: `internal/model/kafka.go:27`
- 📖 **Read by**:
  - `internal/model/kafka.go` - Reads for replication configuration
- ✍️ **Written by**:
  - `internal/model/kafka.go` - Created during KafkaConfig initialization

#### KafkaConnInfo
- 📝 **Defined**: `internal/model/kafka.go:196`
- 📖 **Read by**:
  - Status reporting logic
- ✍️ **Written by**:
  - `internal/model/kafka.go` - ToConnInfo() creates from KafkaConfig

#### KafkaConnDetail
- 📝 **Defined**: `internal/model/kafka.go:201`
- 📖 **Read by**:
  - Status reporting logic
- ✍️ **Written by**:
  - `internal/model/kafka.go` - ToConnInfo() creates connection details

#### KafkaStatusDetail
- 📝 **Defined**: `internal/model/kafka.go:188`
- 📖 **Read by**:
  - `internal/controller/wandb_v2/kafka.go` - Reads to extract status
- ✍️ **Written by**:
  - `internal/model/kafka.go` - Creates from infrastructure results

#### KafkaInfraError
- 📝 **Defined**: `internal/model/kafka.go:136`
- 📖 **Read by**:
  - Error handling and status reporting
- ✍️ **Written by**:
  - `internal/model/kafka.go` - Created on configuration errors

### Minio Model Structs

#### MinioConfig
- 📝 **Defined**: `internal/model/minio.go:26`
- 📖 **Read by**:
  - `internal/controller/infra/minio/tenant/*.go` - Reads config to build Minio Tenant resources
- ✍️ **Written by**:
  - `internal/model/minio.go` - NewMinioConfig() creates defaults based on Size, independent of api/v2 spec

#### MinioSizeConfig
- 📝 **Defined**: `internal/model/minio.go:88`
- 📖 **Read by**:
  - `internal/model/minio.go` - Reads for resource sizing
- ✍️ **Written by**:
  - `internal/model/minio.go` - Created based on Size enum (Small, Medium, Large)

#### MinioConnInfo
- 📝 **Defined**: `internal/model/minio.go:208`
- 📖 **Read by**:
  - Status reporting logic
- ✍️ **Written by**:
  - `internal/model/minio.go` - ToConnInfo() creates from MinioConfig

#### MinioConnDetail
- 📝 **Defined**: `internal/model/minio.go:214`
- 📖 **Read by**:
  - Status reporting logic
- ✍️ **Written by**:
  - `internal/model/minio.go` - ToConnInfo() creates connection details

#### MinioStatusDetail
- 📝 **Defined**: `internal/model/minio.go:178`
- 📖 **Read by**:
  - `internal/controller/wandb_v2/minio.go` - Reads to extract status
- ✍️ **Written by**:
  - `internal/model/minio.go` - Creates from infrastructure results

#### MinioInfraError
- 📝 **Defined**: `internal/model/minio.go:134`
- 📖 **Read by**:
  - Error handling and status reporting
- ✍️ **Written by**:
  - `internal/model/minio.go` - Created on configuration errors

### ClickHouse Model Structs

#### ClickHouseConfig
- 📝 **Defined**: `internal/model/clickhouse.go:18`
- 📖 **Read by**:
  - `internal/controller/infra/clickhouse/altinity/*.go` - Reads config to build ClickHouse resources
- ✍️ **Written by**:
  - `internal/model/clickhouse.go` - NewClickHouseConfig() creates defaults based on Size, independent of api/v2 spec

#### ClickHouseConnInfo
- 📝 **Defined**: `internal/model/clickhouse.go:163`
- 📖 **Read by**:
  - Status reporting logic
- ✍️ **Written by**:
  - `internal/model/clickhouse.go` - ToConnInfo() creates from ClickHouseConfig

#### ClickHouseConnDetail
- 📝 **Defined**: `internal/model/clickhouse.go:169`
- 📖 **Read by**:
  - Status reporting logic
- ✍️ **Written by**:
  - `internal/model/clickhouse.go` - ToConnInfo() creates connection details

#### ClickHouseStatusDetail
- 📝 **Defined**: `internal/model/clickhouse.go:133`
- 📖 **Read by**:
  - `internal/controller/wandb_v2/clickhouse.go` - Reads to extract status
- ✍️ **Written by**:
  - `internal/model/clickhouse.go` - Creates from infrastructure results

#### ClickHouseInfraError
- 📝 **Defined**: `internal/model/clickhouse.go:89`
- 📖 **Read by**:
  - Error handling and status reporting
- ✍️ **Written by**:
  - `internal/model/clickhouse.go` - Created on configuration errors

### Common Model Structs

#### InfraError
- 📝 **Defined**: `internal/model/interface.go:26`
- 📖 **Read by**:
  - All infrastructure error handling
- ✍️ **Written by**:
  - Implemented by MySQL, Redis, Kafka, Minio, ClickHouse InfraError types

#### InfraStatus
- 📝 **Defined**: `internal/model/interface.go:78`
- 📖 **Read by**:
  - Status aggregation logic
- ✍️ **Written by**:
  - Implemented by all infrastructure StatusDetail types

#### Results
- 📝 **Defined**: `internal/model/interface.go:132`
- 📖 **Read by**:
  - `internal/controller/wandb_v2/*.go` - All infrastructure reconcilers read Results
- ✍️ **Written by**:
  - Infrastructure reconciliation logic creates Results

---

## 3. Vendored Operator Structs

### Redis Operator (internal/vendored/redis-operator/)

#### Redis
- 📝 **Defined**: `internal/vendored/redis-operator/redis/v1beta2/redis_types.go:56`
- 📖 **Read by**:
  - `internal/controller/infra/redis/opstree/actual.go` - Reads actual Redis resources
- ✍️ **Written by**:
  - `internal/controller/infra/redis/opstree/desired.go:37` - Creates &redisv1beta2.Redis{}
  - Kubernetes API (persisted resources)

#### RedisSpec
- 📝 **Defined**: `internal/vendored/redis-operator/redis/v1beta2/redis_types.go:29`
- 📖 **Read by**:
  - `internal/controller/infra/redis/opstree/actual.go` - Reads spec fields
- ✍️ **Written by**:
  - `internal/controller/infra/redis/opstree/desired.go:42` - Creates redisv1beta2.RedisSpec{}

#### RedisStatus
- 📝 **Defined**: `internal/vendored/redis-operator/redis/v1beta2/redis_types.go:53`
- 📖 **Read by**:
  - Status monitoring and reporting
- ✍️ **Written by**:
  - Redis operator controller

#### RedisSentinel
- 📝 **Defined**: `internal/vendored/redis-operator/redissentinel/v1beta2/redissentinel_types.go:46`
- 📖 **Read by**:
  - `internal/controller/infra/redis/opstree/actual.go` - Reads actual RedisSentinel resources
- ✍️ **Written by**:
  - `internal/controller/infra/redis/opstree/desired.go:89` - Creates redissentinelv1beta2.RedisSentinel{}

#### RedisSentinelSpec
- 📝 **Defined**: `internal/vendored/redis-operator/redissentinel/v1beta2/redissentinel_types.go:9`
- 📖 **Read by**:
  - `internal/controller/infra/redis/opstree/actual.go` - Reads spec fields
- ✍️ **Written by**:
  - `internal/controller/infra/redis/opstree/desired.go:94` - Creates redissentinelv1beta2.RedisSentinelSpec{}

#### RedisSentinelStatus
- 📝 **Defined**: `internal/vendored/redis-operator/redissentinel/v1beta2/redissentinel_types.go:43`
- 📖 **Read by**:
  - Status monitoring and reporting
- ✍️ **Written by**:
  - Redis operator controller

#### RedisReplication
- 📝 **Defined**: `internal/vendored/redis-operator/redisreplication/v1beta2/redisreplication_types.go:46`
- 📖 **Read by**:
  - Replication mode controllers (if used)
- ✍️ **Written by**:
  - Controllers managing replication

#### KubernetesConfig
- 📝 **Defined**: `internal/vendored/redis-operator/common/v1beta2/common_types.go:9`
- 📖 **Read by**:
  - `internal/controller/infra/redis/opstree/desired.go` - Embeds in Redis specs
- ✍️ **Written by**:
  - `internal/controller/infra/redis/opstree/desired.go` - Creates KubernetesConfig fields

#### Storage
- 📝 **Defined**: `internal/vendored/redis-operator/common/v1beta2/common_types.go:148`
- 📖 **Read by**:
  - Redis resource creation
- ✍️ **Written by**:
  - `internal/controller/infra/redis/opstree/desired.go` - Creates Storage config

#### RedisExporter
- 📝 **Defined**: `internal/vendored/redis-operator/common/v1beta2/common_types.go:129`
- 📖 **Read by**:
  - Monitoring configuration
- ✍️ **Written by**:
  - `internal/controller/infra/redis/opstree/desired.go` - Creates exporter config

#### RedisConfig
- 📝 **Defined**: `internal/vendored/redis-operator/common/v1beta2/common_types.go:140`
- 📖 **Read by**:
  - Redis configuration management
- ✍️ **Written by**:
  - `internal/controller/infra/redis/opstree/desired.go` - Creates Redis config

#### RedisSentinelConfig (common)
- 📝 **Defined**: `internal/vendored/redis-operator/common/v1beta2/common_types.go:217`
- 📖 **Read by**:
  - Sentinel configuration management
- ✍️ **Written by**:
  - `internal/controller/infra/redis/opstree/desired.go` - Creates sentinel config

#### Additional Redis Common Types
- **ACLConfig**, **AdditionalVolume**, **ExistingPasswordSecret**, **InitContainer**, **Service**, **ServiceConfig**, **Sidecar**, **TLSConfig**, **RedisFollower**, **RedisLeader**, **RedisPodDisruptionBudget**
- 📖 **Read by**: Redis infrastructure controllers
- ✍️ **Written by**: `internal/controller/infra/redis/opstree/desired.go`

### Minio Operator (internal/vendored/minio-operator/)

#### Tenant
- 📝 **Defined**: `internal/vendored/minio-operator/minio.min.io/v2/types.go:25`
- 📖 **Read by**:
  - `internal/controller/infra/minio/tenant/actual.go` - Reads actual Tenant resources
- ✍️ **Written by**:
  - `internal/controller/infra/minio/tenant/desired.go:36` - Creates &miniov2.Tenant{}
  - Kubernetes API (persisted resources)

#### TenantSpec
- 📝 **Defined**: `internal/vendored/minio-operator/minio.min.io/v2/types.go:89`
- 📖 **Read by**:
  - `internal/controller/infra/minio/tenant/actual.go` - Reads spec fields
- ✍️ **Written by**:
  - `internal/controller/infra/minio/tenant/desired.go:44` - Creates miniov2.TenantSpec{}

#### TenantStatus
- 📝 **Defined**: `internal/vendored/minio-operator/minio.min.io/v2/types.go:526`
- 📖 **Read by**:
  - Status monitoring and reporting
- ✍️ **Written by**:
  - Minio operator controller

#### Pool
- 📝 **Defined**: `internal/vendored/minio-operator/minio.min.io/v2/types.go:640`
- 📖 **Read by**:
  - Pool configuration for storage
- ✍️ **Written by**:
  - `internal/controller/infra/minio/tenant/desired.go:49` - Creates []miniov2.Pool{}

#### TenantDomains
- 📝 **Defined**: `internal/vendored/minio-operator/minio.min.io/v2/types.go:57`
- 📖 **Read by**:
  - Domain configuration
- ✍️ **Written by**:
  - `internal/controller/infra/minio/tenant/desired.go` - Sets domain config

#### Features
- 📝 **Defined**: `internal/vendored/minio-operator/minio.min.io/v2/types.go:67`
- 📖 **Read by**:
  - Feature flag configuration
- ✍️ **Written by**:
  - `internal/controller/infra/minio/tenant/desired.go` - Sets features

#### Additional Minio Types
- **Bucket**, **CertificateConfig**, **CertificateStatus**, **CustomCertificateConfig**, **CustomCertificates**, **ExposeServices**, **KESConfig**, **Logging**, **PoolsMetadata**, **PoolStatus**, **ServiceMetadata**, **SideCars**, **TenantScheduler**, **TenantUsage**, **TierUsage**, **AuditConfig**
- 📖 **Read by**: Minio infrastructure controllers
- ✍️ **Written by**: `internal/controller/infra/minio/tenant/desired.go`

### Kafka Operator (internal/vendored/strimzi-kafka/)

#### Kafka
- 📝 **Defined**: `internal/vendored/strimzi-kafka/v1beta2/kafka_types.go:255`
- 📖 **Read by**:
  - `internal/controller/infra/kafka/strimzi/actual.go` - Reads actual Kafka resources
- ✍️ **Written by**:
  - `internal/controller/infra/kafka/strimzi/desired.go:26` - Creates &v1beta3.Kafka{}
  - Kubernetes API (persisted resources)

#### KafkaSpec
- 📝 **Defined**: `internal/vendored/strimzi-kafka/v1beta2/kafka_types.go:25`
- 📖 **Read by**:
  - `internal/controller/infra/kafka/strimzi/actual.go` - Reads spec fields
- ✍️ **Written by**:
  - `internal/controller/infra/kafka/strimzi/desired.go:37` - Creates v1beta3.KafkaSpec{}

#### KafkaStatus
- 📝 **Defined**: `internal/vendored/strimzi-kafka/v1beta2/kafka_types.go:229`
- 📖 **Read by**:
  - Status monitoring and reporting
- ✍️ **Written by**:
  - Strimzi Kafka operator

#### KafkaClusterSpec
- 📝 **Defined**: `internal/vendored/strimzi-kafka/v1beta2/kafka_types.go:32`
- 📖 **Read by**:
  - Cluster configuration
- ✍️ **Written by**:
  - `internal/controller/infra/kafka/strimzi/desired.go:38` - Creates v1beta3.KafkaClusterSpec{}

#### KafkaNodePool
- 📝 **Defined**: `internal/vendored/strimzi-kafka/v1beta2/kafkanodepool_types.go:76`
- 📖 **Read by**:
  - Node pool configuration
- ✍️ **Written by**:
  - `internal/controller/infra/kafka/strimzi/desired.go:93` - Creates &v1beta3.KafkaNodePool{}

#### KafkaNodePoolSpec
- 📝 **Defined**: `internal/vendored/strimzi-kafka/v1beta2/kafkanodepool_types.go:25`
- 📖 **Read by**:
  - Node pool spec configuration
- ✍️ **Written by**:
  - `internal/controller/infra/kafka/strimzi/desired.go` - Creates node pool specs

#### GenericKafkaListener
- 📝 **Defined**: `internal/vendored/strimzi-kafka/v1beta2/kafka_types.go:45`
- 📖 **Read by**:
  - Listener configuration
- ✍️ **Written by**:
  - `internal/controller/infra/kafka/strimzi/desired.go` - Creates listeners

#### Additional Kafka Types
- **EntityOperatorSpec**, **EntityTopicOperatorSpec**, **EntityUserOperatorSpec**, **GenericKafkaListenerConfiguration**, **KafkaListenerAuthentication**, **KafkaListenerConfigurationBootstrap**, **KafkaListenerConfigurationBroker**, **KafkaStorage**, **KRaftMetadataStorage**, **StorageVolume**, **ZooKeeperSpec**, **ContainerTemplate**, **EntityOperatorLogging**, **EntityOperatorTemplate**, **KafkaClusterTemplate**, **MetadataTemplate**, **PodTemplate**, **ResourceTemplate**, **StatefulSetTemplate**, **Rack**, **ListenerAddress**, **ListenerStatus**, **PodSetTemplate**, **JvmOptions**, **SystemProperty**, **ZooKeeperClusterTemplate**
- 📖 **Read by**: Kafka infrastructure controllers
- ✍️ **Written by**: `internal/controller/infra/kafka/strimzi/desired.go`

### MySQL Operator (internal/vendored/percona-operator/)

#### PerconaXtraDBCluster
- 📝 **Defined**: `internal/vendored/percona-operator/pxc/v1/pxc_types.go:323`
- 📖 **Read by**:
  - `internal/controller/infra/mysql/percona/actual.go` - Reads actual PXC resources
- ✍️ **Written by**:
  - `internal/controller/infra/mysql/percona/desired.go:36` - Creates &pxcv1.PerconaXtraDBCluster{}
  - Kubernetes API (persisted resources)

#### PerconaXtraDBClusterSpec
- 📝 **Defined**: `internal/vendored/percona-operator/pxc/v1/pxc_types.go:30`
- 📖 **Read by**:
  - `internal/controller/infra/mysql/percona/actual.go` - Reads spec fields
- ✍️ **Written by**:
  - `internal/controller/infra/mysql/percona/desired.go:44` - Creates pxcv1.PerconaXtraDBClusterSpec{}

#### PerconaXtraDBClusterStatus
- 📝 **Defined**: `internal/vendored/percona-operator/pxc/v1/pxc_types.go:266`
- 📖 **Read by**:
  - Status monitoring and reporting
- ✍️ **Written by**:
  - Percona operator controller

#### PXCSpec
- 📝 **Defined**: `internal/vendored/percona-operator/pxc/v1/pxc_types.go:90`
- 📖 **Read by**:
  - PXC node configuration
- ✍️ **Written by**:
  - `internal/controller/infra/mysql/percona/desired.go:51` - Creates &pxcv1.PXCSpec{}

#### ProxySQLSpec
- 📝 **Defined**: `internal/vendored/percona-operator/pxc/v1/pxc_types.go:574`
- 📖 **Read by**:
  - ProxySQL configuration
- ✍️ **Written by**:
  - `internal/controller/infra/mysql/percona/desired.go:82` - Creates &pxcv1.ProxySQLSpec{}

#### HAProxySpec
- 📝 **Defined**: `internal/vendored/percona-operator/pxc/v1/pxc_types.go:579`
- 📖 **Read by**:
  - HAProxy configuration
- ✍️ **Written by**:
  - `internal/controller/infra/mysql/percona/desired.go` - Creates HAProxy config

#### BackupStorageSpec
- 📝 **Defined**: `internal/vendored/percona-operator/pxc/v1/pxc_types.go:657`
- 📖 **Read by**:
  - Backup configuration
- ✍️ **Written by**:
  - `internal/controller/infra/mysql/percona/desired.go` - Creates backup storage config

#### Additional MySQL Types
- **AppStatus**, **BackupContainerArgs**, **BackupContainerOptions**, **BackupStorageAzureSpec**, **BackupStorageS3Spec**, **ClusterCondition**, **ComponentStatus**, **InitContainerSpec**, **LogCollectorSpec**, **PerconaXtraDBClusterBackup**, **PerconaXtraDBClusterRestore**, **PITR**, **PITRSpec**, **PMMSpec**, **PodAffinity**, **PodDisruptionBudgetSpec**, **PodSpec**, **PXCScheduledBackup**, **ReplicasServiceExpose**, **ReplicationChannel**, **ReplicationChannelConfig**, **ReplicationChannelStatus**, **ReplicationSource**, **ReplicationStatus**, **SecretKeySelector**, **ServiceExpose**, **TLSSpec**, **UnsafeFlags**, **UpgradeOptions**, **User**, **Volume**, **VolumeSpec**, **MySQLConfig**, **MySQLSizeConfig**
- 📖 **Read by**: MySQL infrastructure controllers
- ✍️ **Written by**: `internal/controller/infra/mysql/percona/desired.go`

### ClickHouse Operator (internal/vendored/altinity-clickhouse/)

#### ClickHouseInstallation
- 📝 **Defined**: `internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1/types.go:36`
- 📖 **Read by**:
  - `internal/controller/infra/clickhouse/altinity/actual.go` - Reads actual CHI resources
- ✍️ **Written by**:
  - `internal/controller/infra/clickhouse/altinity/desired.go:49` - Creates &v2.ClickHouseInstallation{}
  - Kubernetes API (persisted resources)

#### ChiSpec
- 📝 **Defined**: `internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1/type_spec.go:22`
- 📖 **Read by**:
  - `internal/controller/infra/clickhouse/altinity/actual.go` - Reads spec fields
- ✍️ **Written by**:
  - `internal/controller/infra/clickhouse/altinity/desired.go:57` - Creates v2.ChiSpec{}

#### Status
- 📝 **Defined**: `internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1/type_status.go:46`
- 📖 **Read by**:
  - Status monitoring and reporting
- ✍️ **Written by**:
  - ClickHouse operator controller

#### Configuration
- 📝 **Defined**: `internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1/type_configuration_chi.go:46`
- 📖 **Read by**:
  - Configuration management
- ✍️ **Written by**:
  - `internal/controller/infra/clickhouse/altinity/desired.go:58` - Creates &v2.Configuration{}

#### Cluster
- 📝 **Defined**: `internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1/type_cluster.go:22`
- 📖 **Read by**:
  - Cluster configuration
- ✍️ **Written by**:
  - `internal/controller/infra/clickhouse/altinity/desired.go` - Creates cluster config

#### Templates
- 📝 **Defined**: `internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1/type_templates.go:25`
- 📖 **Read by**:
  - Template configuration
- ✍️ **Written by**:
  - `internal/controller/infra/clickhouse/altinity/desired.go:76` - Creates &v2.Templates{}

#### Additional ClickHouse Types (extensive list)
- **ActionPlan**, **AddonConfiguration**, **AddonSpec**, **ChiClusterAddress**, **ChiClusterLayout**, **ChiClusterRuntime**, **ChiShardAddress**, **ChiShardRuntime**, **ChiShard**, **ClickHouseInstallationRuntime**, **ClickHouseOperatorConfiguration**, **Cleanup**, **ClusterSecret**, **ConfigCRSource**, **Defaults**, **FillStatusParams**, **Host**, **HostPorts**, **HostRuntime**, **HostSecure**, **HostSettings**, **MacrosSection**, **MacrosSections**, **ObjectsCleanup**, **OperatorConfig**, **OperatorConfigAddons**, **OperatorConfigAddonRule**, **OperatorConfigAnnotation**, **OperatorConfigCHI**, **OperatorConfigCHIRuntime**, **OperatorConfigClickHouse**, **OperatorConfigConfig**, **OperatorConfigDefault**, **OperatorConfigFile**, **OperatorConfigFileRuntime**, **OperatorConfigKeeper**, **OperatorConfigLabel**, **OperatorConfigLabelRuntime**, **OperatorConfigMetrics**, **OperatorConfigMetricsLabels**, **OperatorConfigReconcile**, **OperatorConfigReconcileRuntime**, **OperatorConfigRestartPolicy**, **OperatorConfigRestartPolicyRule**, **OperatorConfigRuntime**, **OperatorConfigStatus**, **OperatorConfigStatusFields**, **OperatorConfigTemplate**, **OperatorConfigUser**, **OperatorConfigWatch**, **OperatorConfigWatchNamespaces**, **PodDistribution**, **PodTemplateZone**, **ReconcileHost**, **ReconcileHostDrop**, **ReconcileHostDropReplicas**, **ReconcileHostWait**, **ReconcileHostWaitProbes**, **ReconcileHostWaitReplicas**, **ReconcileMacros**, **SchemaPolicy**, **Setting**, **SettingSource**, **Settings**, **SettingsNormalizerOptions**, **TemplatesList**, **VolumeClaimTemplate**, **ZookeeperConfig**, **ZookeeperNode**
- 📖 **Read by**: ClickHouse infrastructure controllers
- ✍️ **Written by**: `internal/controller/infra/clickhouse/altinity/desired.go`

---

## 4. Usage Summary by Package

### internal/controller/wandb_v2/
**Primary Role**: Orchestrates infrastructure reconciliation and status updates

**Reads from**:
- `api/v2.WeightsAndBiases` - Main CR resource
- `api/v2.WeightsAndBiasesSpec` - Reads Redis, Kafka, MySQL, Minio, ClickHouse, Size specs
- `api/v2.*Status` - Reads all infrastructure status fields

**Writes to**:
- `api/v2.WeightsAndBiasesStatus` - Updates State field
- `api/v2.WBRedisStatus`, `api/v2.WBKafkaStatus`, `api/v2.WBMySQLStatus`, `api/v2.WBMinioStatus`, `api/v2.WBClickHouseStatus` - Updates all infrastructure status

**Files**:
- `weightsandbiases_v2_controller.go` - Main orchestration
- `redis.go`, `kafka.go`, `mysql.go`, `minio.go`, `clickhouse.go` - Infrastructure-specific reconcilers

### internal/controller/translator/v2/
**Primary Role**: Builds defaults and merges user specs with defaults

**Reads from**:
- `api/v2.WB*Spec` types - All infrastructure spec types
- `api/v2.WB*Config` types - All infrastructure config types

**Writes to**:
- `api/v2.WB*Spec` types - Creates struct literals with defaults
- `api/v2.WB*Config` types - Creates struct literals with defaults

**Files**:
- `redis.go`, `kafka.go`, `mysql.go`, `minio.go`, `clickhouse.go` - Infrastructure-specific translators

### internal/model/
**Primary Role**: Business logic layer for configuration and status

**Reads from**:
- `internal/model.*Config` types - Reads its own config structs for transformations
- `internal/model.*StatusDetail` types - Reads its own status structs for conversions
- `internal/model.*ConnInfo` types - Reads its own connection info structs for conversions

**Writes to**:
- `internal/model.*Config` types - Creates internal config structs based on Size defaults
- `internal/model.*StatusDetail` types - Creates status detail structs from infra results
- `internal/model.*ConnInfo` types - Creates connection info structs from configs
- `api/v2.WB*Status` types - Converts StatusDetail to API status structs
- `api/v2.WB*Connection` types - Converts ConnInfo to API connection structs

**Files**:
- `config.go` - InfraConfigBuilder (holds api/v2 specs populated by translator)
- `redis.go`, `kafka.go`, `mysql.go`, `minio.go`, `clickhouse.go` - Infrastructure-specific models
- `interface.go` - Common interfaces

**Key Note**: internal/model does NOT import or read from api/v2 types. It creates default configs based on Size enums and converts its own internal types to api/v2 status types.

### internal/controller/infra/*/
**Primary Role**: Creates actual Kubernetes resources for infrastructure operators

#### internal/controller/infra/redis/opstree/
**Reads from**:
- `internal/model.RedisConfig` - Reads Redis configuration
- `internal/vendored/redis-operator/v1beta2.Redis` - Reads actual resources
- `internal/vendored/redis-operator/v1beta2.RedisSentinel` - Reads actual sentinel resources

**Writes to**:
- `internal/vendored/redis-operator/v1beta2.Redis` - Creates Redis{} instances
- `internal/vendored/redis-operator/v1beta2.RedisSpec` - Creates RedisSpec{} instances
- `internal/vendored/redis-operator/v1beta2.RedisSentinel` - Creates RedisSentinel{} instances
- `internal/vendored/redis-operator/v1beta2.RedisSentinelSpec` - Creates RedisSentinelSpec{} instances
- `internal/vendored/redis-operator/common/v1beta2.*` - Creates all common types

**Files**:
- `desired.go` - Creates desired resources
- `actual.go` - Reads actual resources

#### internal/controller/infra/kafka/strimzi/
**Reads from**:
- `internal/model.KafkaConfig` - Reads Kafka configuration
- `internal/vendored/strimzi-kafka/v1beta2.Kafka` - Reads actual resources

**Writes to**:
- `internal/vendored/strimzi-kafka/v1beta2.Kafka` - Creates Kafka{} instances
- `internal/vendored/strimzi-kafka/v1beta2.KafkaSpec` - Creates KafkaSpec{} instances
- `internal/vendored/strimzi-kafka/v1beta2.KafkaClusterSpec` - Creates KafkaClusterSpec{} instances
- `internal/vendored/strimzi-kafka/v1beta2.KafkaNodePool` - Creates KafkaNodePool{} instances
- All Kafka template and configuration types

**Files**:
- `desired.go` - Creates desired resources
- `actual.go` - Reads actual resources

#### internal/controller/infra/mysql/percona/
**Reads from**:
- `internal/model.MySQLConfig` - Reads MySQL configuration
- `internal/vendored/percona-operator/pxc/v1.PerconaXtraDBCluster` - Reads actual resources

**Writes to**:
- `internal/vendored/percona-operator/pxc/v1.PerconaXtraDBCluster` - Creates PerconaXtraDBCluster{} instances
- `internal/vendored/percona-operator/pxc/v1.PerconaXtraDBClusterSpec` - Creates spec instances
- `internal/vendored/percona-operator/pxc/v1.PXCSpec` - Creates PXC node spec instances
- `internal/vendored/percona-operator/pxc/v1.ProxySQLSpec` - Creates ProxySQL spec instances
- All PXC configuration types

**Files**:
- `desired.go` - Creates desired resources
- `actual.go` - Reads actual resources

#### internal/controller/infra/minio/tenant/
**Reads from**:
- `internal/model.MinioConfig` - Reads Minio configuration
- `internal/vendored/minio-operator/minio.min.io/v2.Tenant` - Reads actual resources

**Writes to**:
- `internal/vendored/minio-operator/minio.min.io/v2.Tenant` - Creates Tenant{} instances
- `internal/vendored/minio-operator/minio.min.io/v2.TenantSpec` - Creates TenantSpec{} instances
- `internal/vendored/minio-operator/minio.min.io/v2.Pool` - Creates Pool{} instances
- All Minio tenant configuration types

**Files**:
- `desired.go` - Creates desired resources
- `actual.go` - Reads actual resources

#### internal/controller/infra/clickhouse/altinity/
**Reads from**:
- `internal/model.ClickHouseConfig` - Reads ClickHouse configuration
- `internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1.ClickHouseInstallation` - Reads actual resources

**Writes to**:
- `internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1.ClickHouseInstallation` - Creates CHI instances
- `internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1.ChiSpec` - Creates ChiSpec{} instances
- `internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1.Configuration` - Creates Configuration{} instances
- `internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1.Templates` - Creates Templates{} instances
- All ClickHouse configuration types

**Files**:
- `desired.go` - Creates desired resources
- `actual.go` - Reads actual resources

---

## 5. Data Flow Diagram

```
User Manifest (api/v2.WeightsAndBiasesSpec)
        ↓
internal/controller/wandb_v2/weightsandbiases_v2_controller.go
        ↓ (reads WeightsAndBiasesSpec)
        ↓
internal/controller/translator/v2/*.go (TRANSLATOR LAYER - THE BRIDGE)
        ↓ (reads api/v2.WB*Spec from user)
        ↓ (builds defaults for missing fields)
        ↓ (merges user input with defaults)
        ↓ (writes back to api/v2.WB*Spec with complete defaults)
        ↓ (populates InfraConfigBuilder with merged specs)
        ↓
internal/model/config.go (InfraConfigBuilder)
        ↓ (holds api/v2 specs with defaults - NO direct api/v2 import in model)
        ↓ (passed to model constructors as plain structs)
        ↓
internal/model/*.go (RedisConfig, KafkaConfig, MySQLConfig, MinioConfig, ClickHouseConfig)
        ↓ (creates internal config structs using Size-based defaults)
        ↓ (NO direct reading of api/v2 types)
        ↓
internal/controller/infra/*/desired.go
        ↓ (reads internal/model.*Config)
        ↓ (creates vendored operator structs)
        ↓
Vendored Operator CRs (Redis, Kafka, MySQL, Minio, ClickHouse)
        ↓
Kubernetes API
        ↓
Operator Controllers (external)
        ↓ (updates vendored operator status)
        ↓
internal/controller/infra/*/actual.go
        ↓ (reads vendored operator status)
        ↓ (creates internal/model.*StatusDetail)
        ↓
internal/model/*.go
        ↓ (converts internal StatusDetail → api/v2.WB*Status)
        ↓ (converts internal ConnInfo → api/v2.WB*Connection)
        ↓
internal/controller/wandb_v2/*.go
        ↓ (writes status back to api/v2)
        ↓
api/v2.WeightsAndBiasesStatus
```

**Key Points**:
- **Translator is the bridge** between api/v2 and model
- **internal/model does NOT import api/v2** - it creates Size-based defaults
- **Data flows**: api/v2 → translator → model → infra → vendored operators
- **Status flows**: vendored operators → infra → model → translator → api/v2

---

## 6. Cross-Package Struct Dependencies

### internal/controller/translator/v2 ↔ api/v2 (THE BRIDGE)
- **Translator reads**: All `WB*Spec` and `WB*Config` types from user input
- **Translator writes**: All `WB*Spec` and `WB*Config` types with defaults merged
- **Role**: The ONLY layer that directly manipulates api/v2 infrastructure spec types

### internal/controller/translator/v2 → internal/model
- **Translator writes**: Populates `InfraConfigBuilder` with api/v2 specs that have defaults
- **Model reads**: Accesses specs from builder (NOT direct api/v2 import)

### internal/model → internal/model (internal conversions)
- `*Config` types → `*ConnInfo` types (via ToConnInfo() methods)
- `*StatusDetail` types → `api/v2.WB*Status` types (via ToStatus() methods)
- `*ConnInfo` types → `api/v2.WB*Connection` types (via conversion methods)

### internal/model → internal/controller/infra/*
- All `*Config` types (RedisConfig, KafkaConfig, MySQLConfig, MinioConfig, ClickHouseConfig) are read by infrastructure controllers

### internal/controller/infra/* ↔ vendored operators
- **Infra writes**: Creates all vendored operator struct types (Redis, Kafka, MySQL, Minio, ClickHouse CRs)
- **Infra reads**: Reads actual vendored operator resources and status

### internal/controller/infra/* → internal/model
- **Infra writes**: Creates `*StatusDetail` types from vendored operator status

### internal/controller/wandb_v2 → api/v2
- **Main controller reads**: All spec types (WeightsAndBiasesSpec)
- **Main controller writes**: All status types (WeightsAndBiasesStatus)

### Key Architectural Principle
**internal/model does NOT import api/v2**. The translator layer acts as the bridge:
- User input (api/v2) → Translator → InfraConfigBuilder → Model
- Model status → Translator (optional) → api/v2 status
- This separation keeps the model layer decoupled from the API layer

---

## 7. Method Receivers Summary

### api/v2 Types
All API types have generated DeepCopy methods (method receivers):
- `func (in *WeightsAndBiases) DeepCopy() *WeightsAndBiases`
- `func (in *WeightsAndBiasesSpec) DeepCopy() *WeightsAndBiasesSpec`
- Similar for all API struct types

### internal/model Types
Model types have business logic method receivers:
- `func (c *RedisConfig) ToConnInfo() RedisConnInfo`
- `func (c *MySQLConfig) ToConnInfo() MySQLConnInfo`
- `func (c *KafkaConfig) ToConnInfo() KafkaConnInfo`
- `func (c *MinioConfig) ToConnInfo() MinioConnInfo`
- `func (c *ClickHouseConfig) ToConnInfo() ClickHouseConnInfo`
- `func (s *RedisStatusDetail) ToStatus() WBRedisStatus`
- Similar patterns for all infrastructure types

### Vendored Operator Types
Vendored operator types have their own generated DeepCopy methods and operator-specific methods.

---

## Notes

1. **api/v2** defines the user-facing API (CRD types)
2. **internal/controller/translator/v2** is THE BRIDGE - reads api/v2, builds defaults, merges, writes back to api/v2
3. **internal/model** does NOT import api/v2 - creates Size-based defaults and converts internal types to api/v2 status
4. **internal/controller/infra** creates actual operator CRs from internal/model configs
5. **internal/controller/wandb_v2** orchestrates the entire flow and updates status

All struct usage follows a clear pattern:
- **Config Flow**: User spec (api/v2) → Translator (bridge) → InfraConfigBuilder → Model (no api/v2 import) → Infrastructure controller → Vendored operators
- **Status Flow**: Vendored operators → Infrastructure controller → Model StatusDetail → Model conversion → api/v2 Status

**Critical Architectural Point**: internal/model is decoupled from api/v2. The translator acts as the adapter/bridge layer.
