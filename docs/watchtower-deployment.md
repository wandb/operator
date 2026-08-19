# Deploying Watchtower

[Watchtower](https://github.com/wandb/watchtower) is the cluster administration UI
that replaces the deprecated W&B console. The operator deploys it as an
operator-owned component — there is no Watchtower chart, and nothing to install
separately.

## Packaging: one image, two entrypoints

Watchtower is **not** a separate image to pull, mirror and version. Its binary is
copied into the operator image at build time and sits next to `/manager`:

```dockerfile
ARG WATCHTOWER_IMAGE=us-docker.pkg.dev/wandb-production/public/wandb/watchtower
ARG WATCHTOWER_VERSION=0.11.0
...
FROM ${WATCHTOWER_IMAGE}:${WATCHTOWER_VERSION} AS watchtower
...
COPY --from=watchtower /watchtower .
```

The binary is lifted from the published Watchtower image rather than rebuilt from
Go source, because it embeds a Next.js static export — building it here would mean
carrying the Watchtower frontend, a Node toolchain and a circular module
dependency (Watchtower already depends on `github.com/wandb/operator`).

Pin a different release with:

```bash
make docker-build WATCHTOWER_VERSION=0.12.0
```

The Application the operator synthesizes selects the second entrypoint with
`command: ["/watchtower"]` and `args: ["--port", "8080"]` — the binary's own
default port is 9090, which would not match the Service or the probes.

### How the operator knows its own image

Because the binary lives inside the operator image, the reconciler has to name
that image when it builds the Application. The operator chart passes it in:

```gotemplate
{{- define "wandb-operator.operatorImageEnv" -}}
- name: OPERATOR_IMAGE
  value: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
{{- end -}}
```

wired through `wandb-operator.envTpls` in `deploy/operator/values.yaml`, and read
by `watchtowerImage()`. Resolving it at runtime is deliberate: a default tag
compiled into the operator would drift on every release, so a 2.1.0 operator
would quietly deploy a 2.0.0 Watchtower. `spec.watchtower.image` overrides it when
set; leaving it empty is the normal case.

If `OPERATOR_IMAGE` is unset the reconciler fails loudly rather than guessing.

## Configuration

```yaml
apiVersion: apps.wandb.com/v2
kind: WeightsAndBiases
spec:
  watchtower:
    install: true                 # opt-in; defaults to false
    basePath: /watchtower         # published route and container base path
    authService: ""               # empty = derive from the server manifest
    resources: {}                 # defaults to 100m / 256Mi requests
    image: {}                     # empty = the operator's own image
    serviceAccount:
      create: true
```

`install` defaults to **false**. Watchtower grants cluster-wide access to anyone
holding an admin credential, so turning it on is an explicit decision.

## What the operator creates

| Resource | Name | Notes |
|----------|------|-------|
| `Application` | `<cr>-watchtower` | `Kind: Deployment`, `replicas: 1`, labelled `weightsandbiases.apps.wandb.com/component=watchtower` so manifest-driven pruning skips it |
| `Service` | `<cr>-watchtower` | ClusterIP `8080`, derived from the Application by the application controller |
| `ServiceAccount` | `<cr>-watchtower` | Token automounted — unlike the W&B app pods, Watchtower calls the Kubernetes API |
| `Secret` | `<cr>-watchtower-auth` | The fallback admin password (see Authentication) |
| `Role` / `RoleBinding` | `<cr>-watchtower` | Namespaced reads |
| `ClusterRole` / `ClusterRoleBinding` | `<namespace>-<cr>-watchtower` | Cluster-wide reads plus `weightsandbiases` `update`/`patch` |
| Ingress path | `<basePath>` on the consolidated Ingress | Ingress mode |
| `HTTPRoute` | via `Application.spec.httpRouteTemplate` | Gateway API mode, same hostnames as the app |

`replicas` is deliberately **not** configurable: in-flight deploy jobs and their
SSE streams live in the serving pod's memory, so a reconnect landing on a second
pod would see no history.

Status lands in `status.watchtowerStatus` (`ready`, `url`, `image`,
`authService`). Watchtower never contributes to the CR's `Ready` condition.

### Naming and multiple installs

Every namespaced object is named from the CR, and cluster-scoped RBAC is
additionally qualified with the namespace, so two `WeightsAndBiases` CRs can
coexist in one namespace *and* in one cluster without sharing an Application,
RBAC binding or Secret.

Names go through `common.FitDefaultInfraName`, which hashes the CR name when the
derived name would exceed the 63-character DNS-1123 label budget. The budget is a
label rather than a subdomain because the application controller derives a Service
from the Application name. Plain truncation would be wrong here: two CR names
differing only past the cutoff would collapse onto one object.

## Reconcile ordering

Watchtower is reconciled inside `ReconcileWandbManifest`, in this order:

```
cleanupNetworkingModeResources + resetInactiveNetworkingStatus
gateway block   (NetworkingModeGatewayAPI)
ingress block   (NetworkingModeIngress)
reconcileWatchtower
─── infrastructure readiness gate ──────────────
migrations, applications, …
```

Two properties this buys, both deliberate:

**Networking and Watchtower sit above the infrastructure gate.** Watchtower exists
to diagnose a broken install, so it has to come up when MySQL, Redis, Kafka, the
object store or ClickHouse are *not* ready — which is when the reconcile returns
early. Its route has to be published for the same reason, so the networking
reconcile moved up with it.

Moving the Ingress reconcile above the gate means infra Services may not exist
yet. `resolveInfraRoutes` treats a missing Service as "skip this route" rather
than an error — a route to a Service that is not there is not publishable, and it
gets picked up on a later pass.

**A Watchtower failure never blocks the W&B install.** `reconcileWatchtower`'s
error is logged and stepped over, not returned. A bad image or an RBAC mistake
must not stop infra, migrations and applications from reconciling. Nothing
downstream reads a Watchtower readiness signal.

One cosmetic consequence: on the first pass after enabling Watchtower, the Ingress
publishes `<basePath>` before the Service exists, so the route 503s for one
reconcile cycle.

## Routing

Watchtower is served from the W&B app's own hostname at `<basePath>`, not on a
separate port or hostname. That is what makes the browser send the app's session
cookie to it, which is what the OIDC auth path depends on.

`basePath` defaults to `/watchtower` and is validated to be non-root — `/` is the
W&B frontend's own path, and mounting Watchtower there would shadow the app it
manages.

### The base path is a build-time value

`basePath` cannot be changed on its own. Next.js bakes `basePath` into every asset
URL and router href at build time, and the Watchtower binary refuses to start when
its runtime base path disagrees with the compiled-in one. Changing it requires a
Watchtower image built with a matching `NEXT_PUBLIC_BASE_PATH`; the published
image is built with `/watchtower`.

The health probes carry the prefix for the same reason: `{basePath}/healthz` and
`{basePath}/ready`, which sit outside the auth gate — the kubelet holds no
credential — but inside the base path.

## Authentication

Two independent credentials, either of which grants access. This mirrors what
console did on-prem: an app session *or* a root password.

**The W&B app session.** Watchtower forwards the caller's `Cookie` and
`Authorization` headers to `GET http://$WATCHTOWER_AUTH_SERVICE/oidc/auth` and
allows the request when gorilla confirms an admin. It implements no OIDC of its
own.

`spec.watchtower.authService` is normally left empty. The operator finds the
manifest application that owns the `/oidc` ingress path — `api` in current
manifests — and uses `<application name>:<ingress service port>`, since the
application controller names each Service after its Application. That keeps
working across manifest renames and port changes. If no application declares
`/oidc`, reconciliation **fails** rather than deploying an unauthenticated
Watchtower.

**The generated admin password.** The operator creates
`<cr>-watchtower-auth` on first reconcile and injects it as
`WATCHTOWER_PASSWORD` via `secretKeyRef` — never inlined in the pod spec, where
anyone with `kubectl get deployment` could read it. Retrieve it with:

```bash
kubectl get secret -n wandb <cr>-watchtower-auth \
  -o jsonpath='{.data.password}' | base64 -d
```

The password is generated once and never rewritten: regenerating on upgrade would
lock the operator's user out of a working install. This is the path that keeps
Watchtower usable when the W&B app itself is down, which is exactly when it is
needed — so the two credentials are not redundant.

`secretKeyRef` env vars are resolved at pod creation and never refreshed, so
rotation is two steps:

```bash
kubectl delete secret -n wandb <cr>-watchtower-auth   # reconcile regenerates it
kubectl rollout restart deployment/<cr>-watchtower -n wandb
```

## RBAC

The apiserver refuses to grant permissions the operator does not itself hold, so
two rules are **missing** from the ClusterRole the operator creates:

- `apiextensions.k8s.io/customresourcedefinitions` `get`/`list` — used to detect
  whether v2 is served
- `pods/portforward` — telemetry port-forward

Both need the operator's own ClusterRole widened first (kubebuilder markers in
`internal/controller/weightsandbiases_controller.go`, then `make manifests`).
Installing or upgrading the operator from inside the pod needs more again, and is
not granted today.

This constraint is specific to operator-created RBAC. It did not apply when a Helm
chart created these objects, because Helm acts as the installing user.

## Verifying a deployment

The pod reaching `1/1 Ready` already confirms a lot: the readiness probe is
`{basePath}/ready`, which only answers 200 once the Kubernetes client has
initialized from the ServiceAccount token. Base path, RBAC and in-cluster
credentials are all proven before you load a page.

To confirm the running binary is the one you built, compare digests rather than
tags:

```bash
kubectl exec -n wandb deploy/<cr>-watchtower -- sha256sum /watchtower
docker run --rm --entrypoint sha256sum <watchtower-image> /watchtower
```

Nodes default to `imagePullPolicy: IfNotPresent` for any tag but `latest`, so a
rebuilt floating tag can leave a node serving the old layer — which looks exactly
like a change that did not land.
