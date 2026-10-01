# Server manifests and integration development

Use the [architecture overview](architecture.md) to locate the relevant layer,
then trace an existing implementation from API fields through desired resources,
connection information, status, and cleanup.

## Manifest resolution and local fixtures

[GetServerManifest](../../pkg/wandb/manifest/manifest.go) resolves OCI artifacts or
`file://` manifests. Registry authentication is supplied by callers with access
to the W&B namespace and pull-secret configuration. The disk cache is keyed by
repository as well as version so equal tags in different registries do not share
content accidentally.

For local fixture work, set `manifestSource="local"` and choose a `wandbVersion`
directory under `localManifestPath` in `tilt-settings.star`. Tilt mounts that
directory tree and sets the CR's file repository. In published mode it leaves
the repository to admission defaults. See [setup](setup.md).

When adding a manifest field, update the loader/merge behavior and consumers,
and test both local and OCI loading where relevant. Coordinate publication with
the upstream manifest generator; checked-in testing manifests are fixtures.

## Infrastructure adapters

Managed implementations under [infra/managed](../../internal/controller/infra/managed)
write service resources and infer readiness. External implementations under
[infra/external](../../internal/controller/infra/external) resolve connection
fields and publish the connection material consumed by applications.

For a change, verify per-instance defaults, stable names, Secret references,
owner references, status conditions, and detach/purge behavior. Infrastructure
readiness gates application initialization and migrations. Merely creating a
Kubernetes object is not sufficient to declare the service ready.

## Application rendering and networking

W&B reconciliation produces `Application` resources from the selected manifest.
The Application controller renders the workload and supporting Services, routes,
jobs, and autoscaling resources. Keep resources for operator-owned integrations
such as Console out of manifest-driven pruning.

Ingress mode consolidates application paths into one Ingress resource; Gateway mode uses
HTTPRoutes attached to the selected Gateway. Networking and Console run before
the infrastructure readiness gate so administration remains reachable during an
infrastructure outage. Preserve that ordering when moving reconcile steps.

Application Services use server-side apply with field manager
`application-controller`. The partial desired object should own only fields
Operator manages; preserve API-assigned values and fields owned by other
controllers. AWS ALB health-check annotations are derived per Service from HTTP
readiness probes. See [user networking](../user/networking.md) for the public
configuration contract and [networking design history](../design/networking-plan.md)
for the original rationale.

## Additional integrations

- [Telemetry internals](telemetry.md): runtime configuration and manifest env sources.
- [Console integration](console.md): packaging, ordering, authentication, and RBAC.
- [Connection internals](connection-internals.md): upstream application URL parameters.
- [Legacy Config API note](../design/legacy-config-api.md): historical console/configmap
  behavior whose current applicability requires review.

Update the relevant user guide when behavior or supported configuration changes;
keep implementation history in design records.
