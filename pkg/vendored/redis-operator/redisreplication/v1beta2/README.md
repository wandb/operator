# RedisReplication API Types (v1beta2)

## Source
- **Package**: `github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2`
- **Version**: v0.22.1
- **Commit**: Corresponds to v0.22.1 release tag
- **Cloned From**: `/Users/j7m4/go/pkg/mod/github.com/!o!t-!c!o!n!t!a!i!n!e!r-!k!i!t/redis-operator@v0.22.1/api/redisreplication/v1beta2/`

## Contents
CRD types for Redis replication setup (primary + replicas):
- `RedisReplication` - Main CRD type
- `RedisReplicationSpec` - Specification structure
- `RedisReplicationStatus` - Status structure
- Webhook validation (not used by our operator)

## Modifications

### redisreplication_types.go
**Line 4**: Updated import path to use vendored common package
```go
common "github.com/wandb/operator/api/redis-operator/model/v1beta2"
```
