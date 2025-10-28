# Strimzi Kafka Operator API Types

This directory contains Go type definitions for interacting with Strimzi Kafka Operator Custom Resources.

## Generation Process

These types were generated to enable the W&B operator to manage Kafka clusters deployed via the Strimzi Kafka Operator.

### Source

- **Strimzi Kafka Operator Version**: 0.48.0
- **API Group**: `kafka.strimzi.io`
- **API Version**: `v1beta2`
- **Source CRD**: `https://github.com/strimzi/strimzi-kafka-operator/blob/0.48.0/install/cluster-operator/040-Crd-kafka.yaml`

### Generation Steps

1. **Downloaded Source CRD**:
   ```
   https://raw.githubusercontent.com/strimzi/strimzi-kafka-operator/0.48.0/install/cluster-operator/040-Crd-kafka.yaml
   ```

2. **Created Type Definitions**:
   - `groupversion_info.go` - API group registration and scheme builder
   - `kafka_types.go` - Go type definitions for Kafka CR

   The types were manually created based on the OpenAPI v3 schema in the CRD using Claude Code.

3. **Generated DeepCopy Methods**:
   ```bash
   make generate
   ```
   This creates `zz_generated.deepcopy.go` with required DeepCopy methods for all types.

4. **Registered with Operator Scheme**:
   Updated `cmd/controller/main.go` to include:
   ```go
   import strimziv1beta2 "github.com/wandb/operator/api/strimzi/v1beta2"

   // In main():
   if err := strimziv1beta2.AddToScheme(mgr.GetScheme()); err != nil {
       setupLog.Error(err, "unable to add Strimzi scheme")
       os.Exit(1)
   }
   ```
