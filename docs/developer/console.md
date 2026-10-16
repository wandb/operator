# Console integration

Use [the user Console guide](../user/console.md) to enable and administer Console.
The public v2 switch is `spec.adminConsoleEnabled`, and the route is `/console`.
Older `spec.watchtower` examples are historical.

## Packaging

The Operator image contains `/manager`, `/crd-installer`, and `/watchtower`.
Console uses the same image with `command: ["/watchtower"]` and port 8080.
The Watchtower binary embeds its frontend, so it is downloaded from a reviewed
Watchtower GitHub release rather than built from this repository's Go sources.

The [Makefile](../../Makefile) pins `WATCHTOWER_VERSION` and supports an explicit
architecture. Authenticate GitHub CLI with read access to `wandb/watchtower`, then
prepare a production image build:

```bash
GH_TOKEN="$(gh auth token)" make download-watchtower
make docker-build
```

Keep the Watchtower binary architecture consistent with the image build. The
[Dockerfile](../../Dockerfile) copies the downloaded binary; it does not download
it. Tilt uses its `Watchtower-Download` resource when Console is enabled and
matches the local manager architecture.

The chart supplies `OPERATOR_IMAGE`; `watchtowerImage()` uses it so the Console
image follows the installed Operator image and registry override. An unset value
is an error. The frontend's base path is compiled into its assets and must match
the runtime `/console` path.

## Reconciliation and ownership

[watchtower.go](../../internal/controller/reconciler/watchtower.go) synthesizes an
Application, ServiceAccount, password Secret, and namespace/cluster RBAC. Generated
names derive from the W&B CR; cluster-scoped names include its namespace, and long
names use the shared hashing helper to fit the DNS label limit.

The Application's component label protects it from manifest-driven pruning.
Console has one replica because deployment jobs and event streams are held in
the serving process. Probes use `/console/healthz` and `/console/ready`.

Networking and Console reconcile before the infrastructure readiness gate.
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
The generated ClusterRole omits CRD discovery and pod port-forwarding. Expanding
Console grants also requires Operator to possess those permissions.

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
