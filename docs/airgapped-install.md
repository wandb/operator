# Air-gapped install: `spec.global.imageRegistry`

This document describes how the operator supports a **fully air-gapped install** — one
where the cluster has no egress to public registries and every container image is pulled
from a customer-controlled mirror — and the `spec.global.imageRegistry` field that makes
the managed data-plane images participate in it.

## The problem

A complete W&B install pulls images from four upstreams, in three tiers:

| Tier | Images | Upstream |
|------|--------|----------|
| **1** | operator chart + image, cert-manager, nginx-gateway, and every **application** image listed in the server manifest (weave, megabinary, frontend, …) | `us-docker.pkg.dev`, `quay.io/jetstack`, `ghcr.io/nginx` |
| **2** | the bundled managed-service **operators** (moco, strimzi, altinity-clickhouse, opstree redis, seaweedfs) and the **Kafka broker** | `docker.io`, `quay.io`, `ghcr.io` |
| **3** | the managed **data-plane** pods: ClickHouse, MySQL, Redis, SeaweedFS **servers** | `docker.io`, `quay.io`, `ghcr.io` |

Tiers 1 and 2 already had retarget paths (the server-manifest rewrite, and Helm image
values on the chart). **Tier 3 did not.** The data-plane image references are hardcoded
Go constants written into the vendored CRs at reconcile time
(`internal/controller/infra/managed/<svc>/<vendor>/spec.go`), with no override. The only
workaround was a node-level containerd registry mirror — impossible on managed platforms
with no node access (GKE Autopilot, EKS Fargate).

## The change

A new top-level CR field:

```go
// api/v2/weightsandbiases_types.go
type WeightsAndBiasesSpec struct {
    ...
    // Global holds settings shared across all managed components.
    Global GlobalSpec `json:"global,omitempty"`
    ...
}

type GlobalSpec struct {
    // ImageRegistry retargets the managed data-plane images to this registry.
    ImageRegistry string `json:"imageRegistry,omitempty"`
}
```

When set, each managed data-plane image has its **registry host replaced** by this value
— a host-strip + prefix that matches where `wsm registry mirror` pushes the mirrored copy.
The repository path and tag are preserved.

```
quay.io/opstree/redis:v7.0.15          ->  <registry>/opstree/redis:v7.0.15
altinity/clickhouse-server:25.8.…      ->  <registry>/altinity/clickhouse-server:25.8.…
ghcr.io/cybozu-go/moco/mysql:8.4.8     ->  <registry>/cybozu-go/moco/mysql:8.4.8
```

The transformation lives in one helper, `common.ApplyImageRegistry(image, registry)`
(`internal/controller/common/image.go`), applied at each image const in the managed specs
for **ClickHouse, MySQL (moco), Redis (standalone/sentinel/replication + exporter), and
SeaweedFS**. An empty `ImageRegistry` is a no-op, so existing installs are unaffected.

**Kafka is intentionally excluded.** The Strimzi *operator* supplies the broker image (via
`STRIMZI_KAFKA_IMAGES`), so the Kafka data plane is retargeted at the chart level (Strimzi's
`defaultImageRegistry`), not through this CR field.


## Full air-gapped install via `wsm`

`wsm` drives the whole flow in two phases. Phase 1 installs the operator stack; phase 2
creates the W&B instance. `--mirror-registry` on each phase wires the matching layer.
```
  ┌────────────────────────── CONNECTED (bastion / CI) ──────────────────────────┐
  │                                                                               │
  │   public registries                                                           │
  │   us-docker.pkg.dev  quay.io  ghcr.io  docker.io                              │
  │            │  pull                                                            │
  │            ▼                                                                  │
  │   wsm registry mirror --to $REG --wandb-version <v>                           │
  │            │  push  (host-stripped: quay.io/strimzi/operator → $REG/strimzi/operator,
  │            │         and the server manifest rewritten → $REG/wandb/*)        │
  │            ▼                                                                  │
  │   ┌───────────────────────────────┐                                          │
  │   │     your mirror registry $REG  │  ◄── now holds tiers 1, 2, 3 + manifest  │
  │   └───────────────────────────────┘                                          │
  └───────────────────────────────────────┬───────────────────────────────────────┘
                                           │   (carry $REG into the air-gap)
  ┌────────────────────────────── AIR-GAPPED ▼ ──────────────────────────────────┐
  │                                                                               │
  │   Phase 1:  wsm deploy-v2 operator      --mirror-registry $REG                │
  │     • operator / cert-manager / nginx charts + images  → $REG  (Helm values)  │
  │     • tier-2 operators + Kafka broker                  → $REG  (per-subchart  │
  │                                                                 Helm values)  │
  │                                                                               │
  │   Phase 2:  wsm deploy-v2 wandb deploy  --mirror-registry $REG --wandb-version <v>
  │     • --manifest-repository defaults to oci://$REG/wandb/server-manifest       │
  │       → operator pulls the rewritten manifest + all tier-1 app images from $REG│
  │     • sets spec.global.imageRegistry = $REG on the CR                         │
  │       → operator host-replaces the tier-3 data-plane image refs               │
  │                                           │                                   │
  │                                           ▼                                   │
  │   ┌────────────────────────────────────────────────────────────────────┐    │
  │   │                          your cluster                                │    │
  │   │                                                                      │    │
  │   │   operator (reconciles the CR) creates:                              │    │
  │   │     app pods (weave, megabinary, …) ─pull─► $REG/wandb/*    (tier 1)  │    │
  │   │     ClickHouse / MySQL / Redis /    ─pull─► $REG/<stripped> (tier 3,  │    │
  │   │       SeaweedFS server pods                  via spec.global.         │    │
  │   │                                              imageRegistry)           │    │
  │   │     Kafka broker pods               ─pull─► $REG/strimzi/* (tier 2)   │    │
  │   └────────────────────────────────────────────────────────────────────┘    │
  │                                                                               │
  │   Result: every pod pulls from $REG. No node containerd config required.      │
  └───────────────────────────────────────────────────────────────────────────────┘
```

### Commands

Two independent version flags are in play:

- **`--operator-chart-version`** — the operator chart + operator image. **This is the
  build that must include `spec.global.imageRegistry`.** Pass it to `registry mirror` *and*
  `deploy-v2 operator`, and keep the two identical.
- **`--wandb-version`** — the server manifest (the W&B application images). Pass it to
  `registry mirror` *and* `wandb deploy`.

```bash
OP_VERSION=2.0.0-alpha.3   # the operator release that includes spec.global.imageRegistry
WB_VERSION=<server-version>

# --- CONNECTED: mirror everything (tiers 1-3 + server manifest + app images) ---
wsm registry mirror --to $REG \
  --operator-chart-version $OP_VERSION \
  --wandb-version $WB_VERSION
wsm registry check --registry $REG \
  --operator-chart-version $OP_VERSION \
  --wandb-version $WB_VERSION --fail-on-missing

# --- AIR-GAPPED: phase 1, operator stack (selects the operator build here) ---
wsm deploy-v2 operator \
  --context <kube-context> \
  --mirror-registry $REG \
  --operator-chart-version $OP_VERSION \
  --registry-ca-file ./ca.crt        # if $REG uses a self-signed / internal CA

# --- AIR-GAPPED: phase 2, W&B instance ---
wsm deploy-v2 wandb deploy \
  --context <kube-context> \
  --mirror-registry $REG \
  --wandb-version $WB_VERSION
```

> `--operator-chart-version` is where you pin the operator build. It defaults to the value
> baked into `wsm` (`2.0.0-alpha.2` at time of writing) — override it until that default is
> bumped to a release containing `spec.global.imageRegistry`. `--wandb-version` does **not**
> control the operator; it only selects the server manifest / app images.

The resulting CR carries the field directly:

```yaml
apiVersion: apps.wandb.com/v2
kind: WeightsAndBiases
spec:
  global:
    imageRegistry: registry.corp.internal:5000   # set by --mirror-registry
  # ...
```

> `$REG` must be served over **HTTPS** (the operator fetches the server manifest over HTTPS
> from inside the cluster). A self-signed cert is fine — pass `--registry-ca-file` on phase 1
> so `wsm` trusts it for chart pulls and mounts it into the operator. See the wsm on-prem
> deployment guide for the registry/TLS setup.

## Requirements

- An operator build that includes `spec.global.imageRegistry` (this branch onward),
  selected with `--operator-chart-version` on `wsm registry mirror` and
  `wsm deploy-v2 operator`.
- `wsm` with two-phase support and `--mirror-registry` on `wandb deploy`.
- A mirror pre-populated by `wsm registry mirror` (which pushes managed images to the
  host-stripped paths the host-replacement produces).
