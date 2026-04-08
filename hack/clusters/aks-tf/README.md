# AKS Test Cluster

Terraform configuration for provisioning an AKS cluster to test Operator v2 deployments.

## Prerequisites

- Azure CLI authenticated (`az login`)
- Terraform >= 1.5
- `kubectl`

## Usage

```bash
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars as needed

terraform init
terraform apply
```

After apply completes, configure kubectl:

```bash
$(terraform output -raw kubeconfig_command)
```

Then add the context to your `tilt-settings.star`:

```python
SETTINGS = {
    "allowedContexts": ["<paste kube_context_name output here>"],
    ...
}
```

Run `tilt up` — Tilt handles cert-manager, ingress/gateway controllers, operators, and the W&B CR.

## Networking Scenarios

| Scenario | `install_cloud_lb_controller` | W&B CR `networking.mode` | Class name |
|----------|------------------------------|--------------------------|------------|
| W&B nginx ingress | `false` | `ingress` | `nginx` |
| W&B nginx-gateway-fabric | `false` | `gateway` | `nginx` |
| Azure AppGW ingress | `true` | `ingress` | `azure-application-gateway` |

Azure does not yet have a mature native Gateway API controller comparable to GKE's. For cloud-native Gateway API testing, use GKE instead. On AKS, use Tilt's nginx-gateway-fabric for Gateway API scenarios.

## Node Sizing

- `node_count = 1`: Single node for `dev` size deployments
- `node_count = 3`: Cross-AZ (zones 1, 2, 3) for `small` and above

Default instance type `Standard_D8s_v5` (8 vCPU, 32 GB RAM) provides sufficient resources per node.

## External Object Store

Set `create_bucket = true` to create an Azure Blob Storage account and container. The outputs map directly to the `wandb-objectstore-connection` / `external-objectstore-connection` secret keys:

| Output | Secret Key |
|--------|------------|
| `objectstore_endpoint` | `Host` |
| `objectstore_port` | `Port` |
| `objectstore_bucket` | `Bucket` |
| `objectstore_region` | `Region` |
| `objectstore_access_key` | `AccessKey` |
| `objectstore_secret_key` | `SecretKey` |
| `objectstore_url` | `url` |

Retrieve sensitive values with `terraform output -raw objectstore_access_key`.

## Teardown

```bash
terraform destroy
```
