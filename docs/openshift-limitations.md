# OpenShift Deployment Limitations

## Architecture

OpenShift support is driven by a single `--openshift` flag (or `OPENSHIFT=true` env var) on the operator binary. The openshift Helm profile (`deploy/operator/profiles/openshift.yaml`) sets this env var and adjusts the operator pod's own security context to be restricted-v2 compliant. When enabled, the controller automatically applies appropriate `podSecurityContext` and container `securityContext` to all managed infrastructure (Redis, Minio, Kafka) without any CR-level configuration.

See `pkg/utils/openshift.go` for the implementation.

## Limitations that cannot be resolved inside this repository

### 1. `megabinary` image exceeds cri-o image config size limit
The W&B application image (`us-docker.pkg.dev/wandb-production/public/wandb/megabinary`) has an image config JSON that exceeds the 4MB hard limit in the `containers/image` library used by cri-o. This causes `ImagePullBackOff` on OpenShift/CRC for the `wandb-gorilla` and `wandb-internal-signer` pods. The limit is not configurable. The image must be restructured upstream (fewer layers or multi-stage build squashing) to work on OpenShift.

### 2. mysql-operator is not compatible with OpenShift
The MySQL operator used by the W&B operator does not support OpenShift's restricted-v2 SCC. The openshift profile disables it. An external MySQL deployment is required. See `hack/testing-manifests/openshift-mysql.yaml` for a reference deployment using `registry.redhat.io/rhel9/mysql-80:latest`.

### 3. ClickHouse operator security context propagation
The Altinity ClickHouse operator applies its own security contexts to ClickHouse pods. On OpenShift, the restricted-v2 SCC is applied automatically by the cluster, so ClickHouse pods run correctly without explicit configuration.

## What works

- **Operator pod**: The openshift profile nullifies hardcoded UIDs/fsGroup from the wandb-base chart defaults so SCC admission can assign namespace-scoped IDs. Container security context (`allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`) comes from the parent chart defaults.
- **Redis** (opstree operator): When `IsOpenShift()` is true, the controller sets restricted-v2 compliant `podSecurityContext` and `securityContext` on Redis standalone/sentinel/replication CRs.
- **Minio** (tenant operator): When `IsOpenShift()` is true, the controller sets `SecurityContext` (pod-level) and `ContainerSecurityContext` on Minio tenant pool specs.
- **Kafka** (Strimzi): When `IsOpenShift()` is true, the controller applies security contexts to Kafka pod templates, entity operator pod templates, and KafkaNodePool pod/container templates.
- **redis-operator pod**: SCC admission handles security context — no profile overrides needed.
- **minio-operator pod**: The openshift profile nullifies hardcoded UIDs/fsGroup and sets restricted-v2 compliant container security context.
- **Grafana operator**: The openshift profile sets `isOpenShift: true` which adjusts its deployment for OpenShift.
- **Frontend nginx**: When `IsOpenShift()` is true, the controller adds writable volume mounts for paths that nginx needs to write to (temp, cache, pid).
