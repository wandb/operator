# Strimzi Kafka Operator API Types (v1)

This directory contains Go type definitions for interacting with Strimzi Kafka Operator v1 Custom Resources.

## Generation Process

These types were generated to enable the W&B operator to manage Kafka clusters, node pools, and topics via the Strimzi Kafka Operator.

### Source

- **Strimzi Kafka Operator Version**: 0.49.1
- **API Group**: `kafka.strimzi.io`
- **API Version**: `v1`
- **Date Vendored**: 2025-12-15

### Source CRDs

- **Kafka**: `https://github.com/strimzi/strimzi-kafka-operator/blob/0.49.1/packaging/install/cluster-operator/040-Crd-kafka.yaml`
- **KafkaNodePool**: `https://github.com/strimzi/strimzi-kafka-operator/blob/0.49.1/packaging/install/cluster-operator/045-Crd-kafkanodepool.yaml`
- **KafkaTopic**: `https://github.com/strimzi/strimzi-kafka-operator/blob/0.49.1/packaging/install/cluster-operator/043-Crd-kafkatopic.yaml`

### Generation Steps

1. **Downloaded CRD YAML files** from Strimzi 0.49.1
2. **Created type definitions** based on OpenAPI v3 schemas in the CRDs
3. **Generated DeepCopy methods** via `make generate`
4. **Manually removed** code generation annotations
5. **Registered** with operator scheme in `cmd/main.go`

### Migration from v1beta2

This replaces the previous v1beta2 types entirely (breaking change). The v1 API is the recommended and stable version in Strimzi 0.49.1.

**Key Changes from v1beta2**:
- KafkaNodePool: Removed deprecated `storage.overrides` field
- All other types: Maintained backward compatibility
- Added: KafkaTopic v1 types for topic management

## What Was Vendored

We vendored the API type definitions needed for our operator:

- `v1/` - Kafka, KafkaNodePool, and KafkaTopic CRD types
  - `groupversion_info.go` - API group registration and scheme builder
  - `kafka_types.go` - Kafka CRD type definitions
  - `kafkanodepool_types.go` - KafkaNodePool CRD type definitions
  - `kafkatopic_types.go` - KafkaTopic CRD type definitions
  - `zz_generated.deepcopy.go` - Generated DeepCopy methods

### Removed Content
- All test files
- Unused CRD types (KafkaConnect, KafkaMirrorMaker, KafkaUser, etc.)
- Internal operator logic and controllers
- Code generation annotations

## License

The vendored code maintains its original Apache 2.0 license from the Strimzi Kafka Operator project.

## Updates

This vendored copy should be reviewed when upgrading to newer versions of the Strimzi Kafka operator to determine if any important changes or fixes need to be incorporated.
