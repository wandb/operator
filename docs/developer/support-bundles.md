# Support-bundle integration

Use the [user guide](../user/support-bundles.md) for configuration and manual
collection. Operator reconciles a suspended CronJob through an Application,
a ConfigMap containing `full.yaml`, a ServiceAccount, and collection permissions.
The implementation is split between
[the reconciler](../../internal/controller/reconciler/support_bundle.go) and
[the collection package](../../internal/observability/supportbundle).

Reconciliation runs before the main infrastructure readiness gate and retries
independently of W&B readiness. It applies its ValidatingAdmissionPolicies first;
without that API it requeues and installs nothing. It then waits for a resolved
default object-store URL Secret before installing the collection template.
`status.supportBundle` reports template readiness, image/version, CronJob name,
whether a collection is `running`, and the last successful time. Disabling the feature removes its managed installation resources.

The collector image is separate from the Operator image. Its pinned version and
registry rewrite live in [image.go](../../pkg/observability/supportbundle/image.go).
The template runs `/app/bin/collector`, obtains its bundle ID from the Job name,
and publishes its result through the Pod termination message. Temporary staging
uses an emptyDir; the default object-store connection supplies persistent output
storage. The ServiceAccount copies W&B application-account annotations for
workload identity configuration.

## Permissions and Console

Review [rbac.go](../../internal/observability/supportbundle/rbac.go) with any
collector change. Collection reads span the W&B namespace, optional telemetry
namespace, and cluster metadata. DNS checks need grants in `default` and
`kube-system`; Victoria export needs temporary Pod creation and exec. Secret reads
are absent from the collector's RBAC.

ValidatingAdmissionPolicies constrain the helper Pods and the Jobs that Console
may create. Console receives Job create/delete, Pod delete, and Pod exec grants
only after its expected Job policy and binding exist and match. Keep that ordering
when changing the Console integration. Missing policy APIs or failed grants must
not silently broaden permissions.

Verify rendered collection configuration, namespace selection, ownership,
enable/disable cleanup, permission-policy ordering, and status in unit/envtest
checks. A live collection must additionally verify helper admission, object-store
authentication, bundle contents, redaction, and retrieval. See the
[coverage tracker](documentation.md#remaining-coverage) for runtime gaps.
