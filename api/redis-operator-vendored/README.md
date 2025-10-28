# Redis Operator API Vendored Code

This directory contains vendored API types from the [OT-CONTAINER-KIT/redis-operator](https://github.com/OT-CONTAINER-KIT/redis-operator) project.

## Source

- **Repository**: https://github.com/OT-CONTAINER-KIT/redis-operator
- **Version**: v0.22.1
- **Date Vendored**: 2025-01-28

## Reason for Vendoring

The redis-operator v0.22.1 has a compilation error in its webhook code that is incompatible with controller-runtime versions used by this project:

```
redissentinel_webhook.go:46:15: undefined: webhook.Validator
```

The webhook code references `webhook.Validator` which doesn't exist in any controller-runtime version - it should be `admission.Validator` or `admission.CustomValidator`.

## Changes Made

### api/redissentinel/v1beta2/redissentinel_webhook.go
- **Line 26-28**: Commented out unused webhook import
- **Line 46-47**: Commented out the problematic `var _ webhook.Validator = &RedisSentinel{}` interface check
- Added comments explaining the patches

### Import Path Updates
All vendored files have been updated to reference the vendored `common/v1beta2` package instead of the upstream package:
- `redis/v1beta2/redis_types.go`
- `redissentinel/v1beta2/redissentinel_types.go`
- `redisreplication/v1beta2/redisreplication_types.go`
- Generated deepcopy files

## What Was Vendored

We vendored the API type definitions needed for our operator:

- `common/v1beta2/` - Common types and configurations
- `redis/v1beta2/` - Redis CRD types (standalone Redis)
- `redisreplication/v1beta2/` - RedisReplication CRD types (HA replication)
- `redissentinel/v1beta2/` - RedisSentinel CRD types (HA monitoring)

## License

The vendored code maintains its original Apache 2.0 license from the redis-operator project.

## Removal

This vendored copy should be removed once redis-operator releases a fixed version (likely v0.22.2 or later) that is compatible with controller-runtime.
