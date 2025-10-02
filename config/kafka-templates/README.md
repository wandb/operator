# Kafka Operator Templates

This directory contains Kubernetes manifest templates for deploying the Strimzi Kafka operator.

## Structure

The manifest files are organized by resource type and name:
- `{NN}-{kind}-{name}.yaml` - Individual Kubernetes resources

Each file contains a single Kubernetes manifest that has been extracted from the original `kafka-operator.yaml`.

## Namespace Parameterization

All occurrences of `namespace: kafka` have been replaced with `namespace: {{.Namespace}}` to allow Go template interpolation at runtime.

## Usage

### Generating a ConfigMap

To generate a ConfigMap containing all these templates:

```bash
kubectl create configmap kafka-operator-templates \
  --from-file=config/kafka-templates/ \
  --namespace=wandb-system \
  --dry-run=client -o yaml > config/samples/kafka-operator-configmap.yaml
```

Or use kustomize:

```bash
kubectl kustomize config/kafka-templates > config/samples/kafka-operator-configmap.yaml
```

### Using with WeightsAndBiases CR

Reference the ConfigMap in your WeightsAndBiases custom resource:

```yaml
apiVersion: wandb.ai/v2
kind: WeightsAndBiases
metadata:
  name: my-instance
  namespace: wandb-system
spec:
  infra:
    streaming:
      enabled: true
      type: kafka
      configMapRef:
        name: kafka-operator-templates
        namespace: wandb-system
```

The operator will:
1. Read all templates from the ConfigMap
2. Execute Go template substitution for `{{.Namespace}}`
3. Apply the manifests to the cluster using server-side apply

## Template Variables

Currently supported template variables:
- `{{.Namespace}}` - The namespace where Kafka resources should be deployed