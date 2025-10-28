# Redis API Types (v1beta2)

## Source
- **Package**: `github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2`
- **Version**: v0.22.1
- **Commit**: Corresponds to v0.22.1 release tag
- **Cloned From**: `/Users/j7m4/go/pkg/mod/github.com/!o!t-!c!o!n!t!a!i!n!e!r-!k!i!t/redis-operator@v0.22.1/api/redis/v1beta2/`

## Contents
CRD types for standalone Redis (single instance):
- `Redis` - Main CRD type
- `RedisSpec` - Specification structure
- `RedisStatus` - Status structure
- Webhook setup (no compilation issues)

## Modifications

### redis_types.go
**Line 20**: Updated import path to use vendored common package
```go
common "github.com/wandb/operator/api/redis-operator-vendored/common/v1beta2"
```

### zz_generated.deepcopy.go
**Line 24**: Updated import path to use vendored common package
```go
commonv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/common/v1beta2"
```

**Reason**: This package doesn't have the webhook.Validator bug, but it needs to be vendored for type consistency since it uses the `common/v1beta2` types which are also vendored.
