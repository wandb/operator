# Pod Labeling Standards

This document defines the labeling standards for all pods (and their controlling
workloads) that the W&B operator creates or manages, with a dedicated section for
non-pod resources such as ServiceAccounts. It is the contract that NetworkPolicies,
dashboards, metrics, and `kubectl` selectors depend on, so treat these labels as a
stable, public API.

## Goals

- Every operator-managed pod is identifiable by a **single, consistent** set of labels.
- Standard ecosystem tooling (kube-state-metrics, Grafana, Lens, k9s, ArgoCD,
  `kubectl`) works out of the box.
- The operator has **stable, immutable, collision-free** selectors for its own
  ownership, pruning, and retention logic.
- Users can write portable NetworkPolicies and selectors against a documented label.

## The two label families

We deliberately maintain **two** label families, each with a distinct job. Do not
collapse them into one.

| Family | Prefix | Purpose | Mutable? |
|--------|--------|---------|----------|
| **Standard / descriptive** | `app.kubernetes.io/*` | Interop with ecosystem tooling; human legibility | Yes — informational only |
| **Operator / ownership** | `weightsandbiases.apps.wandb.com/*` | Workload `spec.selector` and the operator's own list/match/retention logic | No — immutable selector anchor |

**Rule of thumb:** if a human or third-party tool reads it, it's `app.kubernetes.io/*`.
If the operator matches on it or it backs an immutable `spec.selector`, it's
`weightsandbiases.apps.wandb.com/*`. Both families appear on every managed pod.

## Standard labels (`app.kubernetes.io/*`)

Apply all of the following to every operator-managed pod template.

| Label | Value | Example |
|-------|-------|---------|
| `app.kubernetes.io/name` | The **software/service** that runs in the pod | `api`, `executor`, `mysql`, `weave-trace` |
| `app.kubernetes.io/instance` | The **owning `WeightsAndBiases` CR name** (the release) | `wandb` |
| `app.kubernetes.io/component` | The **architectural role** the workload plays | `server`, `worker`, `proxy`, `database`, `cache` |
| `app.kubernetes.io/part-of` | Always `wandb` | `wandb` |
| `app.kubernetes.io/managed-by` | Always `wandb-operator` | `wandb-operator` |
| `app.kubernetes.io/version` | W&B server version (optional but recommended) | `0.79.0` |

### `name` vs `component`

These are different axes and MUST NOT be treated as synonyms:

- `name` answers *"what software is this?"* — the service/binary/image (`api`,
  `mysql`, `redis`).
- `component` answers *"what role does it play?"* — its place in the architecture
  (`server`, `database`, `cache`, `worker`).

They coincide only in the degenerate case of a standalone app that is its own single
role. They diverge whenever the software name differs from its role (`mysql` →
`database`), when one binary runs in multiple roles (`weave-trace` server vs worker),
or when you want tier-level grouping (`component: server` matches every stateless web
service at once). Use `name` for per-service targeting and `component` for per-tier
targeting.

### Rules

- `app.kubernetes.io/part-of: wandb` MUST be present on **every** managed pod. It
  is the single anchor that matches the entire deployment and the documented
  NetworkPolicy selector.
- `app.kubernetes.io/instance` MUST be the CR/release name, **not** the namespace.
  (This corrects the current app-pod behavior where `instance` is set to the
  namespace.)
- `app.kubernetes.io/name` MUST come from the service vocabulary and
  `app.kubernetes.io/component` from the role vocabulary (see the Vocabularies
  section). No free-form values.
- These labels are **descriptive**. Never use them as a `Deployment`/`StatefulSet`
  `spec.selector`, because Helm and users routinely override them and selectors are
  immutable.

## Operator labels (`weightsandbiases.apps.wandb.com/*`)

These back the immutable `spec.selector` and the operator's ownership queries. Keep
the set **minimal and low-cardinality**.

| Label | Value |
|-------|-------|
| `weightsandbiases.apps.wandb.com/name` | The owning CR name |
| `weightsandbiases.apps.wandb.com/namespace` | The owning CR namespace |
| `weightsandbiases.apps.wandb.com/component` | The **service identity** (equivalent to `app.kubernetes.io/name`, e.g. `mysql`) |

### Naming caveat

The operator family's `/component` key does **not** hold the role-based value from
`app.kubernetes.io/component`. For historical reasons (`common.BuildWandbLabels`
populates it from the module/service name), it carries the **service identity** —
i.e. it lines up with `app.kubernetes.io/name`, not the role. This is intentional:
the selector needs to be unique *per service*, and the service name is the stable,
collision-free key for that. Do not "fix" this to match the role vocabulary; doing
so would change immutable selectors.

### Rules

- These are produced by `common.BuildWandbLabels(wandb, component)` — use that
  helper, do not hand-roll the keys.
- The subset used in a workload `spec.selector` is **immutable**. Once a workload
  exists you cannot change its selector; altering it requires deleting and
  recreating the workload. Treat any change here as a breaking migration.
- Never put high-cardinality or user-mutable values here.

## Non-pod resources

This standard is written for **workload pods**, but the operator also creates
ServiceAccounts, Services, Roles/RoleBindings, Secrets, and ConfigMaps. Apply the
labels to those as follows.

- **Descriptive identity labels — apply everywhere.** Every operator-created object
  MUST carry `app.kubernetes.io/part-of: wandb`,
  `app.kubernetes.io/managed-by: wandb-operator`,
  `app.kubernetes.io/instance: <CR name>`, and (where known)
  `app.kubernetes.io/version`. This keeps the whole release queryable and
  attributable regardless of resource kind.

- **`name` / `component` role model — pods only, with judgement for others.** The
  service/role split is meaningful for a workload that runs a specific service in a
  specific role. For a resource dedicated to one service (e.g. that service's own
  `Service` object), set `name`/`component` to match its pods. For shared or
  role-less resources, omit them rather than inventing a value.

- **Operator / selector family — pods and their workloads only.** The
  `weightsandbiases.apps.wandb.com/*` family exists to back immutable `spec.selector`
  fields and pod-ownership queries. Non-pod resources have no `spec.selector`, so it
  is not required on them (the operator already tracks them via owner references).

### The shared ServiceAccount

There is a single ServiceAccount (default `wandb`) **shared by every application
pod**, so it has no single service identity or architectural role. It is explicitly
**exempt from `name` and `component`**. It MUST still carry the descriptive identity
labels (`part-of`, `managed-by`, `instance`, and `version` when available), and user
annotations continue to flow from `spec.wandb.serviceAccount.annotations`.

## Vocabularies

Two closed vocabularies feed the labels above. If a new workload type is introduced,
extend both lists in the same PR.

### Service names (`app.kubernetes.io/name`)

The service/software identity. Sourced from the manifest application name or infra
module name.

- Applications: `api`, `executor`, `filestream`, `filemeta`, `glue`, `parquet`,
  `weave`, `weave-trace`, `weave-trace-worker`, `nginx-proxy`,
  `flat-run-fields-updater`, `metric-observer`.
- Infrastructure: `mysql`, `redis`, `clickhouse`, `kafka`, `seaweedfs`.
- Operational: `migration`.

### Component roles (`app.kubernetes.io/component`)

The architectural role. Keep this list small and generic.

- `server` — stateless request-serving apps.
- `worker` — async/background processors.
- `proxy` — ingress/edge proxies.
- `database` — relational stores.
- `cache` — in-memory caches.
- `analytics-db` — columnar/analytics stores.
- `queue` — message/streaming brokers.
- `object-storage` — blob/object stores.
- `migration` — one-shot migration/init jobs.

### Mapping

| Workload | `name` | `component` |
|----------|--------|-------------|
| api | `api` | `server` |
| executor | `executor` | `worker` |
| filestream | `filestream` | `server` |
| parquet | `parquet` | `worker` |
| weave-trace | `weave-trace` | `server` |
| weave-trace-worker | `weave-trace` | `worker` |
| nginx-proxy | `nginx-proxy` | `proxy` |
| MySQL | `mysql` | `database` |
| Redis | `redis` | `cache` |
| ClickHouse | `clickhouse` | `analytics-db` |
| Kafka | `kafka` | `queue` |
| SeaweedFS | `seaweedfs` | `object-storage` |
| migration/init job | `migration` | `migration` |

Note that `name` and `component` differ for most workloads. They coincide only where
the service *is* its own single role (e.g. the `migration` job).

## Worked example

A pod for the `api` application in a CR named `wandb`, running server version
`0.79.0`, should carry:

```yaml
metadata:
  labels:
    # Standard / descriptive
    app.kubernetes.io/name: api          # the service
    app.kubernetes.io/instance: wandb
    app.kubernetes.io/component: server  # the role
    app.kubernetes.io/part-of: wandb
    app.kubernetes.io/managed-by: wandb-operator
    app.kubernetes.io/version: 0.79.0
    # Operator / ownership (selector anchor)
    weightsandbiases.apps.wandb.com/name: wandb
    weightsandbiases.apps.wandb.com/namespace: wandb-system
    weightsandbiases.apps.wandb.com/component: api  # service identity (see Naming caveat)
```

The workload `spec.selector` matches only on the operator family, e.g.:

```yaml
spec:
  selector:
    matchLabels:
      weightsandbiases.apps.wandb.com/name: wandb
      weightsandbiases.apps.wandb.com/component: api
```

## Using the labels

### NetworkPolicies

Document `app.kubernetes.io/part-of: wandb` as the anchor for the whole deployment.
Use `app.kubernetes.io/name` to target a **specific service** and
`app.kubernetes.io/component` to target a **whole tier** (e.g. every `database`). A
`NetworkPolicy` `podSelector` is independent of the workload's immutable
`spec.selector`, so it is safe to select on the descriptive labels here.

```yaml
# Restrict the MySQL service's ingress to W&B pods only
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: wandb-mysql-restrict
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/part-of: wandb
      app.kubernetes.io/name: mysql       # this specific service
  policyTypes: [Ingress]
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app.kubernetes.io/part-of: wandb
      ports:
        - { protocol: TCP, port: 3306 }
```

To apply a rule to every storage tier at once, select on the role instead — e.g.
`app.kubernetes.io/component: database` for all relational stores.

### Metrics and dashboards

kube-state-metrics exposes `app.kubernetes.io/*` as metric labels. Scope to a release
with `app.kubernetes.io/instance`, break down a single service with
`app.kubernetes.io/name`, and roll up a tier with `app.kubernetes.io/component`.

### Ad-hoc queries

```bash
# Everything in a release
kubectl get pods -l app.kubernetes.io/part-of=wandb,app.kubernetes.io/instance=wandb

# One specific service
kubectl get pods -l app.kubernetes.io/name=clickhouse

# A whole tier (all databases)
kubectl get pods -l app.kubernetes.io/component=database
```
