# Cloud Bootstrap Plan

Deploy the W&B operator on EKS, AKS, and GKE clusters using Terraform and a set of
composable shell scripts. The scripts mirror the deployment steps in the `Tiltfile` but
are designed for one-shot cloud use (no live-reload, no port-forwarding, no Kind-specific
resources).

## Scripts Overview

```
hack/clusters/
├── cluster_state.sh      # Check TF state: EMPTY/PENDING/READY/ERROR
├── cluster_kubeconfig.sh  # Fetch kubeconfig and rename context → cluster_name
├── cluster_setup.sh       # Install operators, CRDs, controller, webhooks
├── cluster_deploy.sh      # Apply W&B CR, external secrets, telemetry
├── cluster_status.sh      # Post-deploy health checklist
├── eks-tf/
├── aks-tf/
└── gke-tf/
```

All scripts accept `<tf-dir>` as their first positional argument.

---

## 1. `cluster_state.sh <tf-dir>`

**Purpose:** Determine the lifecycle state of the Terraform-managed cluster.

**States:**

| State     | Condition |
|-----------|-----------|
| `EMPTY`   | No `terraform.tfstate` file, or the state has zero resources |
| `PENDING` | State exists and has some resources, but the cluster resource itself is not yet present or nodes are not ready |
| `READY`   | The cluster resource and node group/pool exist in state and show no tainted/errored resources |
| `ERROR`   | State exists but `terraform output` fails, or resources are tainted/failed |

When the state is not `READY`, the script prints actionable guidance:
- `EMPTY` → `"Run: cd <tf-dir> && terraform init && terraform apply"`
- `PENDING` → `"Terraform apply may still be running or was interrupted. Run: cd <tf-dir> && terraform apply"`
- `ERROR` → `"Resources are in a bad state. Run: cd <tf-dir> && terraform plan to diagnose"`

**Exit codes:** `0` = READY, `1` = ERROR, `2` = PENDING, `3` = EMPTY

**Dependencies:** `terraform`, `jq`

---

## 2. `cluster_kubeconfig.sh <tf-dir>`

**Purpose:** Fetch the kubeconfig for the cluster and rename the context to match `cluster_name`.

**Steps:**
1. Read `cluster_name`, `kubeconfig_command`, and `kube_context_name` from TF outputs.
2. Run the `kubeconfig_command` (cloud-provider-specific).
3. Rename the context from `kube_context_name` → `cluster_name` (skip if already named correctly).
4. Set the current context to `cluster_name`.
5. Verify connectivity with `kubectl get nodes`.

**Key invariant:** The tf-directory parameter fully determines which context to use. Every
script that operates on a cluster reads `cluster_name` from the TF outputs and uses it as
the kubectl context name. `cluster_setup.sh` always calls this script first.

**Dependencies:** `terraform`, `kubectl`, cloud CLI (`aws`/`az`/`gcloud` depending on provider)

---

## 3. `cluster_setup.sh <tf-dir> [options]`

**Purpose:** Install the operator infrastructure on a READY cluster. After this runs,
the cluster has all CRDs, operators, and the W&B controller ready to accept CRs.

Calls `cluster_state.sh` and `cluster_kubeconfig.sh` internally.

### Steps

| Step | What | Source |
|------|------|--------|
| 1 | Validate cluster READY + configure context | `cluster_state.sh` + `cluster_kubeconfig.sh` |
| 2 | cert-manager | Tiltfile `deploy_cert_manager()` |
| 3 | Pre-apply CRDs (server-side apply) + Helm labels | Tiltfile `WandB-CRDs-Apply` |
| 4 | Third-party operators (helm install) | Tiltfile `ThirdParty-Operators` |
| 5 | Operator manifests (RBAC, webhooks, certs) | Tiltfile `Operator-Manifests` + `Operator-Generate` |
| 6 | Build/deploy operator controller | Tiltfile `Operator-Build` |
| 7 | Wait for webhook CA bundle injection | Tiltfile `Operator-Webhook-Ready` |

### Options

```
--skip-build      Skip building and pushing the operator image
--registry <url>  Override the container registry URL
--image <ref>     Use a pre-built operator image instead of building
```

---

## 4. `cluster_deploy.sh <tf-dir> [options]`

**Purpose:** Deploy the W&B CR, external secrets, and telemetry stack. Requires
`cluster_setup.sh` to have been run first (verifies webhook readiness).

### Steps

| Step | What |
|------|------|
| 1 | Verify operator webhook is ready |
| 2 | Create external object store secret (if TF has `objectstore_url`) |
| 3 | Build and apply W&B CR from kustomize base + overlays |
| 4 | Install telemetry stack (helm) |

### Options

```
--skip-wandb      Skip deploying the W&B CR
--skip-telemetry  Skip the telemetry stack
--overlay <name>  Add a kustomize overlay (repeatable)
```

### W&B CR construction (known issue)

The current base CR (`hack/testing-manifests/wandb/kustomize/base/wandb.yaml`) contains
Tilt-specific fields like `manifestRepository: "file:///server-manifest"`. This needs to
be revisited for cloud deployments — either a separate cloud base or an overlay that
strips the Tilt-only fields.

---

## 5. `cluster_status.sh <tf-dir>`

**Purpose:** Post-deployment health checklist. Shows the context name and the status
of every component installed by `cluster_setup.sh` and `cluster_deploy.sh`.

**Checklist items (in dependency order):**

| # | Component | Check |
|---|-----------|-------|
| 1 | Cluster connectivity | Nodes are Ready |
| 2 | cert-manager | Deployments available |
| 3 | W&B CRDs | Established |
| 4 | Third-party operators | Deployments available in `wandb-operator` namespace |
| 5 | Operator namespace | `operator-system` exists |
| 6 | Operator RBAC | Key roles present |
| 7 | Operator controller | Deployment available |
| 8 | Webhooks | CA bundle injected |
| 9 | W&B CR | Exists, report phase |
| 10 | External secrets | Present if object store configured |
| 11 | W&B pods | Running in default namespace |
| 12 | Telemetry | VM/Grafana pods if installed |

**Short-circuit logic:** If item N fails and N+1 depends on it, skip N+1.

---

## Execution Order

```bash
# 1. Apply terraform (user does this manually for each cloud)
cd hack/clusters/eks-tf && terraform init && terraform apply
cd hack/clusters/aks-tf && terraform init && terraform apply
cd hack/clusters/gke-tf && terraform init && terraform apply

# 2. (Optional) Check cluster readiness
hack/clusters/cluster_state.sh hack/clusters/eks-tf

# 3. Install operator infrastructure
hack/clusters/cluster_setup.sh hack/clusters/eks-tf --skip-build
hack/clusters/cluster_setup.sh hack/clusters/aks-tf --skip-build
hack/clusters/cluster_setup.sh hack/clusters/gke-tf --skip-build

# 4. Deploy W&B CR and telemetry
hack/clusters/cluster_deploy.sh hack/clusters/eks-tf --overlay size-small
hack/clusters/cluster_deploy.sh hack/clusters/aks-tf --overlay size-small
hack/clusters/cluster_deploy.sh hack/clusters/gke-tf --overlay size-small

# 5. Verify deployments
hack/clusters/cluster_status.sh hack/clusters/eks-tf
hack/clusters/cluster_status.sh hack/clusters/aks-tf
hack/clusters/cluster_status.sh hack/clusters/gke-tf

# Individual scripts can also be used standalone:
hack/clusters/cluster_kubeconfig.sh hack/clusters/eks-tf  # just configure kubectl
hack/clusters/cluster_state.sh hack/clusters/aks-tf       # just check TF state
```

**Note:** Scripts must be run sequentially per cloud (they share the kubeconfig file).
Each script pins `--context` on all kubectl/helm calls, but the kubeconfig write from
cloud CLIs cannot be parallelized.

---

## Resolved Decisions

1. **Context management** — `cluster_setup.sh` always calls `cluster_kubeconfig.sh` first. The tf-directory parameter fully determines which cluster context is active.

2. **Terraform is read-only** — Scripts never run `terraform apply`. When `cluster_state.sh` returns a non-READY state, it prints the exact command the user should run.

3. **Hostname** — Internal testing only. `hostname: http://localhost` is fine.

4. **Storage classes and tfvars** — Out of scope. User's responsibility via `terraform.tfvars`.
