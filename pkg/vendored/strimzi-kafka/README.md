# Strimzi Kafka Operator API Vendored Code

This directory contains vendored API types from the [Strimzi Kafka Operator](https://github.com/strimzi/strimzi-kafka-operator) project.

## Source

- **Repository**: https://github.com/strimzi/strimzi-kafka-operator
- **Go API types version**: 0.49.1 (vendored 2025-12-15)
- **CRD YAML version**: 0.50.0 — matches the chart's `strimzi-kafka-operator` subchart dependency

The Go types and CRD YAMLs are intentionally pinned to slightly different versions because the CRD YAMLs are installed by the `crd-installer` binary and must match what the deployed Strimzi controller expects, while the Go types are used only for the operator's own scheme registration / typed client access (where 0.49.1 ↔ 0.50.0 is API-compatible at v1).

## Reason for Vendoring

To have full control over the CRD API types and avoid unexpected breaking changes when the upstream operator updates. This allows the W&B operator to manage Kafka clusters while controlling when and how we adopt upstream changes.

## Changes Made

### Removed Code Generation Annotations

All code generation annotations have been removed from vendored code:
- `+kubebuilder:*` annotations (e.g., `+kubebuilder:validation:*`, `+kubebuilder:object:*`)
- `+k8s:*` annotations (e.g., `+k8s:deepcopy-gen:*`, `+k8s:openapi-gen=true`)

These annotations are only needed when running code generators like controller-gen or deepcopy-gen, which we explicitly do not do for vendored APIs. The Makefile's `generate` target only processes our own `./api/v1` and `./api/v2` types.

Files affected:
- `v1/groupversion_info.go`
- `v1/kafka_types.go`
- `v1/kafkanodepool_types.go`
- `v1/kafkatopic_types.go`

## What Was Vendored

We vendored the API type definitions needed for our operator:

### CRDs

The full Strimzi 0.50.0 CRD set is vendored under [`crds/`](./crds/). These files are installed at chart install/upgrade time by the `crd-installer` binary, which `go:embed`s them at build time.

| File | CRD |
| --- | --- |
| `kafka.strimzi.io_kafkas.yaml` | `kafkas.kafka.strimzi.io` |
| `kafka.strimzi.io_kafkaconnects.yaml` | `kafkaconnects.kafka.strimzi.io` |
| `core.strimzi.io_strimzipodsets.yaml` | `strimzipodsets.core.strimzi.io` |
| `kafka.strimzi.io_kafkatopics.yaml` | `kafkatopics.kafka.strimzi.io` |
| `kafka.strimzi.io_kafkausers.yaml` | `kafkausers.kafka.strimzi.io` |
| `kafka.strimzi.io_kafkanodepools.yaml` | `kafkanodepools.kafka.strimzi.io` |
| `kafka.strimzi.io_kafkabridges.yaml` | `kafkabridges.kafka.strimzi.io` |
| `kafka.strimzi.io_kafkaconnectors.yaml` | `kafkaconnectors.kafka.strimzi.io` |
| `kafka.strimzi.io_kafkamirrormaker2s.yaml` | `kafkamirrormaker2s.kafka.strimzi.io` |
| `kafka.strimzi.io_kafkarebalances.yaml` | `kafkarebalances.kafka.strimzi.io` |

**Refreshing CRDs for a new Strimzi version**: download each `0XX-Crd-*.yaml` file from `https://raw.githubusercontent.com/strimzi/strimzi-kafka-operator/<version>/packaging/install/cluster-operator/` and rename it using the `<group>_<plural>.yaml` convention shown above.

### API Types

- `v1/` - Kafka, KafkaNodePool, and KafkaTopic CRD types (v1 API)
  - `groupversion_info.go` - API group registration and scheme builder
  - `kafka_types.go` - Kafka CRD type definitions
  - `kafkanodepool_types.go` - KafkaNodePool CRD type definitions
  - `kafkatopic_types.go` - KafkaTopic CRD type definitions
  - `zz_generated.deepcopy.go` - Generated DeepCopy methods

### Removed Content
- All test files
- Unused CRD types (KafkaConnect, KafkaMirrorMaker, KafkaUser, etc.)
- Internal operator logic and controllers

## License

The vendored code maintains its original Apache 2.0 license from the Strimzi Kafka Operator project.

## Removal

This vendored copy should be reviewed when upgrading to newer versions of the Strimzi Kafka operator to determine if any important changes or fixes need to be incorporated.
