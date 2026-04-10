# EKS Test Cluster

Terraform configuration for provisioning an EKS cluster to test Operator v2 deployments.

## Prerequisites

- AWS CLI installed and authenticated (`aws sso login --profile <profile>` or `aws configure`)
- Terraform >= 1.5
- `kubectl`
- Docker (for ECR image push)

### Required IAM Permissions

The caller needs permissions to create: VPC, subnets, internet gateway, route tables, IAM roles/policies, OIDC provider, EKS cluster, EKS node groups, EKS add-ons, ECR repositories (if `create_ecr = true`), S3 buckets (if `create_bucket = true`), and Helm releases (if `install_cloud_lb_controller = true`).

## Usage

```bash
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars:
#   - set aws_profile if using SSO or named profiles
#   - pick a region close to you to avoid image push timeouts

terraform init
terraform apply
```

After apply completes, configure kubectl:

```bash
eval "$(terraform output -raw kubeconfig_command)"
kubectl get nodes  # verify all nodes are Ready
```

If using ECR (`create_ecr = true`), authenticate Docker:

```bash
eval "$(terraform output -raw ecr_login_command)"
```

Then configure your `tilt-settings.star` with the terraform outputs:

```bash
terraform output -raw kube_context_name
terraform output -raw ecr_registry_host  # if create_ecr = true
terraform output -raw ecr_repo_name      # if create_ecr = true
```

```python
SETTINGS = {
    "allowedContexts": ["<paste kube_context_name>"],
    "defaultRegistry": "<paste ecr_registry_host>",      # if create_ecr = true
    "registrySingleName": "<paste ecr_repo_name>",       # if create_ecr = true
    ...
}
```

Run `tilt up` — Tilt handles cert-manager, ingress/gateway controllers, operators, and the W&B CR.

Note: ECR login tokens expire after 12 hours. Re-run the login command if pushes start failing.

## Container Registry (ECR)

Remote clusters need a container registry for Tilt to push operator images to. Set `create_ecr = true` to create an ECR repository. EKS nodes can pull from it automatically (via the `AmazonEC2ContainerRegistryReadOnly` policy on the node role).

ECR does not support nested repositories, so Tilt must be configured with `registrySingleName` to push all images to a single repo with different tags.

## Networking Scenarios

| Scenario | `install_cloud_lb_controller` | W&B CR `networking.mode` | Class name |
|----------|------------------------------|--------------------------|------------|
| W&B nginx ingress | `false` | `ingress` | `nginx` |
| W&B nginx-gateway-fabric | `false` | `gateway` | `nginx` |
| AWS ALB ingress | `true` | `ingress` | `alb` |
| AWS Gateway API | `true` | `gateway` | `amazon-vpc-lattice` |

The W&B controllers (nginx ingress and nginx-gateway-fabric) are installed by Tilt. The AWS Load Balancer Controller requires IAM integration and is installed by Terraform when `install_cloud_lb_controller = true`.

## Node Sizing

- `node_count = 1`: Single node for `dev` size deployments
- `node_count = 3`: Cross-AZ for `small` and above

Default instance type `m5.2xlarge` (8 vCPU, 32 GB RAM) provides sufficient resources per node.

## External Object Store

Set `create_bucket = true` to create an S3 bucket with a dedicated IAM user and access key. The outputs map directly to the `wandb-objectstore-connection` / `external-objectstore-connection` secret keys:

| Output | Secret Key |
|--------|------------|
| `objectstore_endpoint` | `Host` |
| `objectstore_port` | `Port` |
| `objectstore_bucket` | `Bucket` |
| `objectstore_region` | `Region` |
| `objectstore_access_key` | `AccessKey` |
| `objectstore_secret_key` | `SecretKey` |
| `objectstore_url` | `url` |

Retrieve sensitive values:

```bash
terraform output -raw objectstore_access_key
terraform output -raw objectstore_secret_key
terraform output -raw objectstore_url
```

## Teardown

```bash
terraform destroy
```
