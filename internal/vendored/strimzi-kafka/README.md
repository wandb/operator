# Strimzi Kafka Operator API Vendored Code

This directory contains vendored API types from the [Strimzi Kafka Operator](https://github.com/strimzi/strimzi-kafka-operator) project.

## Source

- **Repository**: https://github.com/strimzi/strimzi-kafka-operator
- **Version**: 0.48.0
- **Date Vendored**: 2025-10-28

## Reason for Vendoring

To have full control over the CRD API types and avoid unexpected breaking changes when the upstream operator updates. This allows the W&B operator to manage Kafka clusters while controlling when and how we adopt upstream changes.

## Changes Made

### Removed Code Generation Annotations

All code generation annotations have been removed from vendored code:
- `+kubebuilder:*` annotations (e.g., `+kubebuilder:validation:*`, `+kubebuilder:object:*`)
- `+k8s:*` annotations (e.g., `+k8s:deepcopy-gen:*`, `+k8s:openapi-gen=true`)

These annotations are only needed when running code generators like controller-gen or deepcopy-gen, which we explicitly do not do for vendored APIs. The Makefile's `generate` target only processes our own `./api/v1` and `./api/v2` types.

Files affected:
- `v1beta2/groupversion_info.go`
- `v1beta2/kafka_types.go`
- `v1beta2/kafkanodepool_types.go`

## What Was Vendored

We vendored the API type definitions needed for our operator:

- `v1beta2/` - Kafka and KafkaNodePool CRD types
  - `groupversion_info.go` - API group registration and scheme builder
  - `kafka_types.go` - Kafka CRD type definitions
  - `kafkanodepool_types.go` - KafkaNodePool CRD type definitions
  - `zz_generated.deepcopy.go` - Generated DeepCopy methods

### Removed Content
- All test files
- Unused CRD types (KafkaConnect, KafkaMirrorMaker, KafkaTopic, etc.)
- Internal operator logic and controllers

## License

The vendored code maintains its original Apache 2.0 license from the Strimzi Kafka Operator project.

## Removal

This vendored copy should be reviewed when upgrading to newer versions of the Strimzi Kafka operator to determine if any important changes or fixes need to be incorporated.
