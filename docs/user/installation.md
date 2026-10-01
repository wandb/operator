# Install Operator and deploy W&B

Complete [deployment planning](planning.md) first. This example uses managed
infrastructure and an existing Ingress controller. Substitute your versions,
hostname, and Ingress class before running commands.

## Install cert-manager

Skip this step when cert-manager is already installed and ready. Set
`CERT_MANAGER_VERSION` to the cert-manager chart version selected for your cluster.

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install cert-manager jetstack/cert-manager \
  --version "$CERT_MANAGER_VERSION" \
  --namespace cert-manager --create-namespace \
  --set crds.enabled=true --wait --timeout 10m
```

The webhook certificate is separate from the public W&B TLS certificate.

## Install Operator

Set `OPERATOR_VERSION` to your chosen published v2 chart version. Create an
`operator-values.yaml` file for any chart overrides; an empty configuration is
valid for the managed default:

```yaml
{}
```

```bash
kubectl create namespace wandb
helm install wandb-operator \
  oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator \
  --version "$OPERATOR_VERSION" \
  --namespace wandb-operators --create-namespace \
  -f operator-values.yaml --wait --timeout 10m
kubectl wait -n wandb-operators --for=condition=Ready \
  certificate/wandb-operator-serving-cert --timeout=300s
kubectl rollout status -n wandb-operators deployment/wandb-operator --timeout=300s
```

Skip namespace creation if it already exists. Use the OCI chart above: the
`wandb/operator` chart in the legacy `charts.wandb.ai` repository is Operator v1.
Save the values file and chart version for future upgrades.

## Create the W&B resource

Provide a TLS Secret named `wandb-tls` in the `wandb` namespace, or adapt the
[networking configuration](networking.md) to your certificate setup. Save this
as `wandb.yaml`, replacing `<wandb-version>`, the hostname, and `nginx` with your
chosen W&B version, public URL, and Ingress class:

```yaml
apiVersion: apps.wandb.com/v2
kind: WeightsAndBiases
metadata:
  name: wandb
  namespace: wandb
spec:
  size: small
  retentionPolicy:
    onDelete: detach
  wandb:
    version: <wandb-version>
    hostname: https://wandb.example.com
  networking:
    mode: ingress
    ingress:
      ingressClassName: nginx
      managed: true
    tls:
      secretName: wandb-tls
```

Add your [license and application configuration](configuration.md) and any
[external infrastructure](infrastructure.md) before applying it. Omitted backing
services use managed defaults.

```bash
kubectl apply --dry-run=server -f wandb.yaml
kubectl apply -f wandb.yaml
kubectl get weightsandbiases wandb -n wandb -w
```

Server dry-run checks admission; it does not test registry access or provision
infrastructure.

## Verify the deployment

```bash
kubectl wait -n wandb --for=condition=Ready \
  weightsandbiases/wandb --timeout=30m
kubectl get weightsandbiases wandb -n wandb -o yaml
kubectl get deployments,statefulsets,jobs,pvc,ingress -n wandb
```

Check the `Ready` condition's `observedGeneration` against the resource's
`metadata.generation`, infrastructure statuses, migration status, and application
rollouts. Point DNS at the address assigned by your routing controller, then open
the configured W&B URL and complete sign-in/setup. Verify a run and an artifact
upload/download, including a browser download when using direct object-store
URLs. A ready controller pod alone does not establish a working W&B deployment.

If the wait times out, follow [troubleshooting](troubleshooting.md). You can also
enable [Console](console.md) and [monitoring](monitoring.md).
