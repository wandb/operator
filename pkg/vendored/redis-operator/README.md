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

### Removed Code Generation Annotations

All code generation annotations have been removed from vendored code:
- `+kubebuilder:*` annotations (e.g., `+kubebuilder:validation:*`, `+kubebuilder:default:*`, `+kubebuilder:object:*`)
- `+k8s:*` annotations (e.g., `+k8s:deepcopy-gen:*`, `+k8s:openapi-gen=true`)

These annotations are only needed when running code generators like controller-gen or deepcopy-gen, which we explicitly do not do for vendored APIs. The Makefile's `generate` target only processes our own `./api/v1` and `./api/v2` types.

Files affected:
- `common/v1beta2/common_types.go`
- `common/v1beta2/groupversion_info.go`
- `redis/v1beta2/redis_types.go`
- `redis/v1beta2/groupversion_info.go`
- `redisreplication/v1beta2/redisreplication_types.go`
- `redisreplication/v1beta2/groupversion_info.go`
- `redissentinel/v1beta2/redissentinel_types.go`
- `redissentinel/v1beta2/redissentinel_webhook.go`
- `redissentinel/v1beta2/groupversion_info.go`

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

### CRDs

**NOTE: CRD files must be downloaded separately and are not currently vendored.**

To download the CRDs for integration testing:

1. Create the `crds/` directory:
   ```bash
   mkdir -p internal/vendored/redis-operator/crds
   ```

2. Download the Redis Operator CRDs from the upstream repository at v0.22.1:
   ```bash
   # Redis (standalone)
   curl -L https://raw.githubusercontent.com/OT-CONTAINER-KIT/redis-operator/v0.22.1/config/crd/bases/redis.redis.opstreelabs.in_redis.yaml \
     -o internal/vendored/redis-operator/crds/redis.redis.opstreelabs.in_redis.yaml

   # RedisReplication
   curl -L https://raw.githubusercontent.com/OT-CONTAINER-KIT/redis-operator/v0.22.1/config/crd/bases/redis.redis.opstreelabs.in_redisreplications.yaml \
     -o internal/vendored/redis-operator/crds/redis.redis.opstreelabs.in_redisreplications.yaml

   # RedisSentinel
   curl -L https://raw.githubusercontent.com/OT-CONTAINER-KIT/redis-operator/v0.22.1/config/crd/bases/redis.redis.opstreelabs.in_redissentinels.yaml \
     -o internal/vendored/redis-operator/crds/redis.redis.opstreelabs.in_redissentinels.yaml
   ```

3. **When updating to a new version**: Update the version tag (v0.22.1) in the URLs above to match the new vendored version.

**Purpose**: Integration testing with real Kubernetes API server (envtest)

### API Types

- `common/v1beta2/` - Common types and configurations
- `redis/v1beta2/` - Redis CRD types (standalone Redis)
- `redisreplication/v1beta2/` - RedisReplication CRD types (HA replication)
- `redissentinel/v1beta2/` - RedisSentinel CRD types (HA monitoring)

### Removed Content
- All test files (`*_test.go`)
- Unused CRD types (RedisCluster)
- Internal operator logic and controllers

## License

The vendored code maintains its original Apache 2.0 license from the redis-operator project.

## Removal

This vendored copy should be removed once redis-operator releases a fixed version (likely v0.22.2 or later) that is compatible with controller-runtime.
