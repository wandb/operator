# Console integration

Use [the user Console guide](../user/console.md) to enable and administer Console.
The public v2 switch is `spec.adminConsoleEnabled`, and the route is `/console`.
Older `spec.watchtower` examples are historical.

## Packaging

The Operator image contains `/manager`, `/crd-installer`, and `/watchtower`.
Console uses the same image with `command: ["/watchtower"]` and passes
`--port 8080` explicitly so the container matches its Service and probes.
The Watchtower binary embeds its frontend, so it is downloaded from a reviewed
Watchtower GitHub release rather than built from this repository's Go sources.

The [Makefile](../../Makefile) pins `WATCHTOWER_VERSION` and supports an explicit
architecture. Authenticate GitHub CLI with read access to `wandb/watchtower`, then
prepare a production image build:

```bash
GH_TOKEN="$(gh auth token)" make download-watchtower
make download-grafana-plugins
make docker-build
```

Keep the Watchtower binary architecture consistent with the image build. The
[Dockerfile](../../Dockerfile) copies the downloaded binary; it does not download
it. Tilt uses its `Watchtower-Download` resource when Console is enabled and
matches the local manager architecture. The image also carries Grafana plugins;
`make download-grafana-plugins` prepares them before a Docker build. Tilt has a
`Grafana-Plugins-Download` resource for its telemetry setup.

The chart supplies `OPERATOR_IMAGE` as `wandb-operator.image.repository:tag`, and
`watchtowerImage()` uses it verbatim. An image digest or `global.repositoryPrefix`
applied to the Operator container is not reflected, and the CR's image registry
is not applied. An unset value is an error. The frontend's base path is compiled into its assets and must match
the runtime `/console` path.

## Reconciliation and ownership

[watchtower.go](../../internal/controller/reconciler/watchtower.go) synthesizes an
Application, ServiceAccount, password Secret, and namespace/cluster RBAC. Long
names are hashed by `common.FitDefaultInfraName` rather than truncated:

| Resources | Name | Limit |
| --- | --- | --- |
| Application, Service, ServiceAccount, Role, RoleBinding | `<cr>-watchtower` | 63 |
| Password Secret | `<cr>-watchtower-auth` | 63 |
| ClusterRole, ClusterRoleBinding | `<namespace>.<cr>-watchtower` | 253 |

The Application's component label protects it from manifest-driven pruning.
Console has one replica because deployment jobs and event streams are held in
the serving process. Probes use `/console/healthz` and `/console/ready`.

Networking and Console reconcile before the infrastructure readiness gate, so
[resolveInfraRoutes](../../internal/controller/reconciler/infra_routes.go) skips
any infra route whose Service does not exist yet; a later pass publishes it.
Console errors produce a retry without blocking application reconciliation.
Its status is published in `status.watchtowerStatus`, independently of W&B
readiness. Disabling Console removes its resources and password Secret.

## Authentication and permissions

The authentication service is derived from the manifest application declaring
the `/oidc` route. Missing that application is an error. Console validates a W&B
administrator session through `/oidc/auth`, or uses its generated fallback
password. The shared W&B hostname allows the browser to send its session cookie.

Read current RBAC in `reconcileWatchtowerRBAC` before expanding capabilities.
Default grants include W&B CR modification, namespace Secret reads, and
namespace ActionRun creation/deletion for administrator-published actions.
Cluster-wide Pod metrics reads support the Metrics tab when metrics-server is
available. [Support-bundle grants](support-bundles.md#permissions-and-console)
are conditional on the expected admission policy and binding being installed.
The generated ClusterRole omits CRD discovery and pod port-forwarding. Operator
already holds CRD `get/list` for support bundles, so only `pods/portforward`
needs Operator widening. Operator can delegate only what it holds: widen both its
kubebuilder RBAC markers and the hand-maintained chart roles in
`deploy/operator/templates/wandb-operator-*-role.yaml`.

Two deployment-level environment switches on Operator add separate capabilities:

| Variable | Effect |
| --- | --- |
| `WATCHTOWER_ENABLE_SECRET_WRITES` | Any nonempty value grants writes to all Secrets in the W&B namespace |
| `WATCHTOWER_ENABLE_DB_ADMIN` | Enables the Watchtower database administration capability |

Unset these variables to disable them; a string such as `false` is still nonempty.
Treat these as deployment policy, since they affect credentials or application
data beyond editing a W&B CR. Keep the user guide aligned with authentication,
routing, or lifecycle changes. Historical reasoning is retained in
[Watchtower research](../design/watchtower-history.md).
