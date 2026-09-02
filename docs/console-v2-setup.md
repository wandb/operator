# Setting Up Console v2

Console v2 (formerly Watchtower) is the cluster administration UI for W&B
deployments, replacing the deprecated v1 console. It is served from your W&B
hostname at `/console`.

The short version: **install the operator as usual, then set one field on the
`WeightsAndBiases` resource.** There is no second chart, no separate image to
mirror, and no extra Helm release.

```yaml
spec:
  adminConsoleEnabled: true
```

## What you need before you start

| Requirement | Why |
|---|---|
| cert-manager, installed cluster-wide | The operator chart renders `Certificate`/`Issuer` for the webhook serving cert. `helm install` fails without it. |
| A default `StorageClass` | Managed backing services request PersistentVolumes. |
| Helm ≥ 3.8.0, `kubectl` within one minor of the cluster | OCI chart support. |
| `spec.wandb.hostname` set on the CR | Console is served from the app's own hostname; without it there is no route and no status URL. |
| `spec.networking.mode` = `ingress` or `gatewayAPI` | The route is published on the consolidated Ingress or an HTTPRoute. |
| A W&B **admin** user, or the generated admin password | Both auth paths are described under [Logging in](#logging-in). |

## Step 1 — Install cert-manager

Skip if your cluster already runs it.

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace --set crds.enabled=true
```

## Step 2 — Install the operator

Nothing console-specific here — this is the standard install. The operator image
already carries the console binary as a second entrypoint.

```bash
helm install wandb-operator oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator --version <operator-version> --namespace wandb-operators --create-namespace
```

Use the OCI path above. The `wandb/operator` chart in the legacy
`charts.wandb.ai` repository is Operator v1 and cannot run Console v2.

Upgrading an existing v2 operator is the same command with `upgrade --install`:

```bash
helm upgrade --install wandb-operator oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator --version <operator-version> --namespace wandb-operators
```

### Why no image values to set

The console binary lives at `/console` inside the operator image, next to
`/manager`. The operator finds that image by reading the `OPERATOR_IMAGE`
environment variable, which the chart already sets from the values you are
installing with:

```gotemplate
{{- define "wandb-operator.operatorImageEnv" -}}
- name: OPERATOR_IMAGE
  value: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
{{- end -}}
```

So the console version always matches the operator version, and an air-gapped
mirror of the operator image carries the console with it. If you override
`wandb-operator.image.repository`/`tag`, the console follows automatically.

If `OPERATOR_IMAGE` is somehow unset, console reconciliation fails loudly rather
than guessing a tag.

## Step 3 — Enable it on the `WeightsAndBiases` resource

```yaml
apiVersion: apps.wandb.com/v2
kind: WeightsAndBiases
metadata:
  name: wandb
  namespace: wandb
spec:
  size: small
  adminConsoleEnabled: true
  wandb:
    version: <wandb-version>
    hostname: https://wandb.example.com
  networking:
    mode: ingress
  retentionPolicy:
    onDelete: detach
```

```bash
kubectl apply -f wandb.yaml
```

`adminConsoleEnabled` defaults to **false**. Console v2 grants cluster-wide read
access plus write access to the `WeightsAndBiases` resource to anyone who can
authenticate, so enabling it is meant to be an explicit decision.

On an existing install, flipping the flag is enough — no operator restart:

```bash
kubectl patch weightsandbiases wandb -n wandb --type=merge -p '{"spec":{"adminConsoleEnabled":true}}'
```

Setting it back to `false` deletes every console resource, including the
password Secret.

## Step 4 — Verify

```bash
kubectl get pods -n wandb -l weightsandbiases.apps.wandb.com/component=watchtower
kubectl get weightsandbiases wandb -n wandb -o jsonpath='{.status.watchtowerStatus}' | jq
```

Expected status:

```json
{
  "ready": true,
  "url": "https://wandb.example.com/console",
  "image": "us-docker.pkg.dev/wandb-production/public/wandb/operator:<tag>",
  "authService": "api:8080"
}
```

The pod reaching `1/1 Ready` proves a lot on its own: the readiness probe is
`/console/ready`, which only answers 200 once the Kubernetes client has
initialized from the ServiceAccount token. Base path, RBAC and in-cluster
credentials are all confirmed before you load a page.

On the first reconcile after enabling, the Ingress publishes `/console` a moment
before the Service exists, so the route can 503 for one cycle. It resolves
itself.

## Logging in

Browse to `https://<your-wandb-hostname>/console`. Two independent credentials
work, and either alone is sufficient.

**Your W&B admin session.** Console forwards your browser's `Cookie` and
`Authorization` headers to `GET /oidc/auth` on the W&B API service and allows the
request when gorilla confirms the caller is an **admin**. Non-admin W&B users are
rejected. Console implements no OIDC of its own — this is why it must be served
from the app's own hostname, so the browser sends the app's session cookie to it.

**The generated admin password.** The operator generates one on first reconcile
and stores it in a Secret. Retrieve it with:

```bash
kubectl get secret -n wandb wandb-watchtower-auth -o jsonpath='{.data.password}' | base64 -d
```

Then use the `/console/login` form. This path is what keeps the console usable
when the W&B app itself is down — which is exactly when you need it — so the two
credentials are not redundant.

The password is generated once and never rewritten on upgrade. To rotate it,
delete the Secret and restart the pod, since `secretKeyRef` env vars are resolved
at pod creation and never refreshed:

```bash
kubectl delete secret -n wandb wandb-watchtower-auth
kubectl rollout restart deployment/wandb-watchtower -n wandb
```

## Things worth knowing


**Console comes up even when the install is broken.** It is reconciled *above*
the infrastructure readiness gate, so it starts when MySQL, Redis, Kafka, the
object store or ClickHouse are not ready — which is when you most need it.
Conversely, a console failure never blocks your W&B install: the error is logged
and stepped over, and it never contributes to the CR's `Ready` condition.

**`authService` is derived, not configured.** The operator finds the server-manifest
application that owns the `/oidc` ingress path (`api` in current manifests) and
uses `<application>:<port>`. If no application declares `/oidc`, reconciliation
**fails** rather than deploying an unauthenticated console.

**Two RBAC rules are missing by design.** The apiserver refuses to grant
permissions the operator does not itself hold, so the console's ClusterRole lacks
`apiextensions.k8s.io/customresourcedefinitions` `get`/`list` (v2 served-version
detection) and `pods/portforward` (telemetry port-forward). Those features degrade
until the operator's own ClusterRole is widened. Installing or upgrading the
operator from inside the console needs more permission again, and is not granted
today.

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| No `wandb-watchtower` pod | `adminConsoleEnabled` unset or `false`. Check `kubectl get weightsandbiases wandb -n wandb -o jsonpath='{.spec.adminConsoleEnabled}'`. |
| Operator logs `OPERATOR_IMAGE is unset` | The operator was installed by something other than the chart, or with the env removed. Reinstall with the chart. |
| Operator logs `cannot derive ... authService` | The server manifest has no application serving `/oidc`. Check `spec.wandb.version`. |
| Pod runs but never `Ready` | Readiness is `/console/ready`; it fails until the Kubernetes client initializes. Check the ClusterRoleBinding exists and the SA token is mounted. |
| `/console` returns 404 | `spec.networking.mode` is unset, so no Ingress/HTTPRoute is published. |
| `/console` returns 401 for a valid W&B login | The account is not a W&B **admin**. The auth sub-request is admin-only. |
| `status.watchtowerStatus.url` is empty | `spec.wandb.hostname` is not set. |

## Related

- [Deploying Watchtower](watchtower-deployment.md) — implementation detail and design rationale
- [Configuration API](config-api.md)
- [Monitoring and Telemetry Guide](monitoring.md)
