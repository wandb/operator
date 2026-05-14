# OpenShift Deployment Limitations

## Limitations that cannot be resolved inside this repository

### 1. `megabinary` image exceeds cri-o image config size limit
The W&B application image (`us-docker.pkg.dev/wandb-production/public/wandb/megabinary`) has an image config JSON that exceeds the 4MB hard limit in the `containers/image` library used by cri-o. This causes `ImagePullBackOff` on OpenShift/CRC for the `wandb-gorilla` and `wandb-internal-signer` pods. The limit is not configurable. The image must be restructured upstream (fewer layers or multi-stage build squashing) to work on OpenShift.

### 2. mysql-operator is not compatible with OpenShift
The MySQL operator used by the W&B operator does not support OpenShift's restricted-v2 SCC. An external MySQL deployment is required. See `hack/testing-manifests/openshift-mysql.yaml` for a reference deployment using `registry.redhat.io/rhel9/mysql-80:latest`.

### 3. ClickHouse operator security context propagation
The Altinity ClickHouse operator applies its own security contexts to ClickHouse pods. The `podSecurityContext` and `securityContext` fields in `managedClickhouse` are defined in the CRD but the ClickHouse operator controller does not currently read them. On OpenShift, the restricted-v2 SCC is applied automatically by the cluster, so ClickHouse pods run correctly without explicit configuration. If the ClickHouse operator is deployed outside OpenShift with a custom PodSecurityPolicy, these fields would need to be wired through the ClickHouse operator's CR spec.

### 4. Grafana operator on OpenShift
The `grafana-operator` subchart has an `isOpenShift: true` flag in the openshift profile that adjusts its deployment for OpenShift. No additional security context changes are needed for the grafana-operator itself.

## What works

- **Redis** (opstree operator): `podSecurityContext` and `securityContext` from the WandB CR are passed through to Redis standalone, sentinel, and replication CRs. Hardcoded `fsGroup: 1000` has been removed.
- **Minio** (tenant operator): `podSecurityContext` → Pool `SecurityContext`, `securityContext` → Pool `ContainerSecurityContext`.
- **Kafka** (Strimzi): `podSecurityContext` and `securityContext` are applied to Kafka pod templates, entity operator pod templates, and KafkaNodePool pod/container templates.
- **Operator pod**: The openshift profile sets `runAsNonRoot`, `seccompProfile: RuntimeDefault`, and container-level `allowPrivilegeEscalation: false` with `capabilities.drop: [ALL]`.
- **redis-operator and minio-operator pods**: The openshift profile nullifies hardcoded UIDs/fsGroups and sets restricted-v2 compliant security contexts.
