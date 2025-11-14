# RedisSentinel API Types (v1beta2)

## Source
- **Package**: `github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2`
- **Version**: v0.22.1
- **Commit**: Corresponds to v0.22.1 release tag
- **Cloned From**: `/Users/j7m4/go/pkg/mod/github.com/!o!t-!c!o!n\!t\!a\!i\!n\!e\!r-\!k\!i\!t/redis-operator@v0.22.1/api/redissentinel/v1beta2/`

## Contents
CRD types for Redis Sentinel (HA monitoring and failover):
- `RedisSentinel` - Main CRD type
- `RedisSentinelSpec` - Specification structure
- `RedisSentinelConfig` - Sentinel-specific configuration
- `RedisSentinelStatus` - Status structure
- Webhook validation (contains patched code)

## Modifications

### redissentinel_webhook.go
**Line 26-28**: Commented out unused webhook import
```go
// PATCHED: Commented out unused import after removing webhook.Validator
// "sigs.k8s.io/controller-runtime/pkg/webhook"
```

**Line 47-48**: Commented out problematic webhook.Validator interface check
```go
// PATCHED: Commented out due to controller-runtime incompatibility
// var _ webhook.Validator = &RedisSentinel{}
```

**Reason**: The original code references `webhook.Validator` which doesn't exist in controller-runtime. This line is only needed for webhook server compilation, which we don't use - we only need the CRD type definitions.

### redissentinel_webhook_test.go
**Deleted**: Test file removed as it references internal redis-operator packages we don't need.

### redissentinel_types.go
**Line 4**: Updated import path to use vendored common package
```go
common "github.com/wandb/operator/api/redis-operator-vendored/model/v1beta2"
```
