# Resource retention when deleting a WeightsAndBiases CR

Status: implementation and test contract on the retention-lifecycle branch.
Inventory, ownership migration, durable finalization, Argo metadata, and DELETE
admission are implemented. Real-cluster acceptance remains a release gate;
passing unit/envtest checks alone does not establish garbage-collector behavior.

Inventory audited against operator revision `892b4bff`.

## Contract

Retention applies to an infrastructure **instance and its required resources**,
not just its primary custom resource. An instance is identified by the WandB
installation UID, component, and instance key. Kafka currently has one instance;
MySQL, Redis, object storage, and ClickHouse have named instances.

Keep the existing configuration surface:

- `spec.retentionPolicy.onDelete` defaults to `detach`.
- A managed instance's `retentionPolicy` overrides the global policy. An absent
  override inherits the global policy. An explicitly present empty policy
  defaults its `onDelete` to `detach`, consistent with the schema default.
- Generated connection Secrets for external instances use the global policy;
  external services and customer-supplied resources are never purge targets.
- Only `detach` and `purge` are valid. Reject unknown values at admission and
  stop destructive reconciliation if an invalid value is encountered in stored
  state. Never interpret an unrecognized value as permission to delete.

| Effective policy | Required result after WandB deletion |
| --- | --- |
| `detach` | Retain the instance's roots, credentials, configuration, identities, storage, and required dependencies. The service remains usable and can restart using its retained configuration and data. |
| `purge` | Explicitly delete the instance's verified operator-managed inventory, including persistent claims and generated credentials. Wait for deletion to finish before completing finalization. |

These results must hold for background, foreground, and orphan propagation.
Orphan propagation does not override an explicit `purge` policy. Foreground
deletion must finish without waiting for retained resources to disappear.

No retention-controlled resource, including one currently configured for purge,
may have an ownership path to the WandB CR. A retained resource must also be
independent of any owner scheduled for explicit purge. Establish this invariant
**before accepting deletion**, rather than removing owner references during
finalization. Preserve ownership within each infrastructure instance, such as
Application -> StatefulSet -> Pod. Policy changes must not reintroduce ownership
of infrastructure by WandB.

This contract covers deletion of the WandB CR and supported Argo CD application
deletion. It cannot preserve a resource whose namespace or CRD is deleted, whose
own DELETE has already been accepted, or which another actor explicitly deletes.
Removing finalizers manually bypasses lifecycle completion. Vendor controllers
and the required cluster services must remain available for retained services
to reconcile and restart.

W&B application workloads, initialization/migration jobs, application ingress/routes,
support bundles, and installation telemetry are outside infrastructure
retention. Their cleanup must not delete a retained instance's dependencies.
Retention does not promise continued access through the deleted installation's
ingress or continued operation of the W&B application itself.

Infrastructure HTTPRoutes and their GKE HealthCheckPolicies are inventoried
separately from the backend data. With an external Gateway, they follow each
backend instance's retention policy: preserve routes for detached services and
remove routes for purged services. When the parent Gateway belongs to this
installation, delete these routes before completing Gateway teardown even if the
backend is detached. The external Gateway itself is never a purge target.

Routes have no WandB owner reference and use the same UID verification and Argo
protection as other retained resources. Migration of previously unowned routes
requires operator metadata plus a live, verified backend Service. Names alone
do not authorize adoption or deletion. A managed Gateway carries installation
UID metadata so orphan propagation cannot prevent its explicit cleanup.

## Inventory and provenance

Persist inventory while resources are provisioned. Each record must contain:

- Installation UID, component, original instance key, and the effective policy
  with its configuration source.
- Resource group/kind, namespace, name, and observed UID; enough information to
  distinguish a replacement at the same name from the recorded resource.
- Whether the resource was created by this installation, created by a verified
  vendor-owned descendant, or supplied by the customer.
- Dependencies, including references to other instances and shared resources.
- Lifecycle progress that survives restarts and partial failures. Do not store
  credential values in inventory/status; record Secret identities instead.

Use installation UID + component + instance association metadata for discovery
and watches. Keep the original instance key losslessly; a label-safe encoding
must not conflate different keys. Existing name/namespace/component labels are
insufficient to authorize deletion: they omit the installation UID and instance.
Names and prefix matches alone are insufficient too.

The identities below describe the resources created by the current builders.
`N` is the resolved managed spec name and `NS` its resolved namespace, including
custom values and default-name shortening. Consult each naming helper rather
than independently reconstructing names from the WandB name.

### MySQL: MOCO

| Resource | Identity / relationship |
| --- | --- |
| `MySQLCluster` | `NS/N` |
| MySQL configuration ConfigMap | `NS/N-mycnf`, referenced by the cluster |
| Connection Secret | `NS/N-connection` |
| Initialization credential Secret | `NS/N-db-password`; created if absent, reused if present |
| Vendor resources | MOCO-created workloads, Services, configuration and credential Secrets, and persistent claims associated with this exact cluster |

Both generated Secrets belong in the retained inventory. The database-password
Secret currently has no WandB owner reference, so owner-reference traversal alone
will miss it. Its name and existing labels also cannot prove who created a
pre-existing Secret. Record provenance at creation; migration must protect
ambiguous resources instead of treating them as purgeable.

The `N-moco-init` Job in the WandB namespace is transient installation work,
not a retained database service. Stop it before deleting credentials on purge.

Sources: [MOCO builders](../../../internal/controller/infra/managed/mysql/moco/spec.go),
[connection Secret](../../../internal/controller/infra/managed/mysql/moco/conn.go),
[credential creation and initialization](../../../internal/controller/reconciler/mysql.go).

### Redis: Opstree

| Resource | Identity / relationship |
| --- | --- |
| Standalone `Redis` | `NS/N`, when sentinel is disabled |
| `RedisSentinel` | `NS/N`, when sentinel is enabled |
| `RedisReplication` | `NS/N-replica`, when sentinel is enabled |
| Connection Secret | `NS/N-connection` |
| Vendor resources | Workloads, Services, configuration, and persistent claims for the recorded standalone or replication/sentinel roots |

Inventory includes previously created roots if an instance changes mode.
Discover claims from the actual StatefulSet claim templates and recorded
identities, including scaled-down claims. Do not assume one fixed claim-template
name or select all Redis claims in the namespace.

Sources: [Redis builders](../../../internal/controller/infra/managed/redis/opstree/spec.go),
[names](../../../internal/controller/infra/managed/redis/opstree/naming.go),
[claim discovery](../../../internal/controller/infra/managed/redis/opstree/write.go).

### Object storage: SeaweedFS

| Resource | Identity / relationship |
| --- | --- |
| `Seaweed` | `NS/N` |
| S3 identity/configuration Secret | `NS/N-s3-config`, owned by the Seaweed CR |
| Connection Secret | `NS/<N with trailing -seaweedfs removed>-objectstore-connection` |
| Vendor resources | Master, volume, filer, and S3 gateway workloads; Services; configuration; TLS resources when enabled; and all persistent claims |

Retain the filer metadata and volume data together. Discover any master storage
created by the deployed vendor version as well. Preserving bucket data without
the filer metadata or S3 identity is not a successful detach. The internal
Seaweed -> S3 configuration Secret ownership is valid and should remain.

Legacy default-instance inventory also includes a verified WandB-owned MinIO
`Tenant` named `<wandb-name>-minio` in the WandB namespace, its generated
configuration/credentials and persistent claims, and the legacy connection
Secret `<wandb-name>-objectstore-connection`. Deduplicate shared identities with
the current inventory. A missing MinIO CRD may be treated as unavailable legacy
inventory; permission or API errors must not be silently treated as absence.

Sources: [SeaweedFS builders](../../../internal/controller/infra/managed/objectstore/seaweedfs/spec.go),
[names](../../../internal/controller/infra/managed/objectstore/seaweedfs/naming.go),
[S3 configuration](../../../internal/controller/infra/managed/objectstore/seaweedfs/write.go),
[legacy MinIO handling](../../../internal/controller/infra/managed/objectstore/seaweedfs/legacy_minio.go).

### ClickHouse: Altinity and Keeper

| Resource | Identity / relationship |
| --- | --- |
| `ClickHouseInstallation` | `NS/N` |
| `ClickHouseKeeperInstallation` | `NS/<N with trailing -chi removed>-chk` |
| ServiceAccount | Configured name, otherwise `N`; managed only when creation is enabled and provenance is established |
| Connection Secret | `NS/N-connection` |
| Vendor resources | ClickHouse and Keeper workloads, Services, generated configuration, and persistent claims |
| Object-store dependency | `objectStore[clickhouse]`, falling back to `objectStore[default]` |

Keeper is part of the ClickHouse instance even though it has a separate CR and
component label. Retain Keeper data, ClickHouse local metadata/system data, and
the backing object store together. An unspecified vendor PVC reclaim policy
does not establish retention; configure and verify the required vendor behavior.

Every managed ClickHouse instance currently uses the same `clickhouse` object
store selection, regardless of its own instance key. With static credentials,
the generated ClickHouse configuration references the resolved object-store
credential Secret. That Secret is a dependency even when the bucket is external.

Sources: [ClickHouse reconciliation and store selection](../../../internal/controller/reconciler/clickhouse.go),
[builders and ServiceAccount](../../../internal/controller/infra/managed/clickhouse/altinity/spec.go),
[Keeper naming](../../../internal/controller/infra/managed/clickhouse/altinity/naming.go),
[storage credential references](../../../internal/controller/infra/managed/clickhouse/altinity/objectstorage.go).

### Kafka: Bufstream and etcd

| Resource | Identity / relationship |
| --- | --- |
| Bufstream `Application` | `NS/N` |
| etcd `Application` | `NS/N-etcd` |
| ConfigMap | `NS/N-config` |
| Storage credential Secret | `NS/N-storage-credentials` |
| Connection Secret | `NS/N-connection` |
| ServiceAccount | Configured name, otherwise `N`; managed only when creation is enabled and provenance is established |
| OpenShift SCC RoleBinding | `NS/N-scc-nonroot-v2`, when created; its shared ClusterRole is not an inventory member |
| Application children | Bufstream Deployment and Service, etcd StatefulSet and headless Service, ReplicaSets/Pods, and etcd persistent claims |
| Object-store dependency | `objectStore[bufstream]`, falling back to `objectStore[default]` |

Retain etcd metadata, broker configuration, the copied storage credentials,
identity/permission resources, and object-store data together. Bufstream copies
resolved storage credentials into its own Secret; the source connection Secret
and the copied Secret must remain distinct inventory entries with their actual
consumers recorded.

Sources: [Bufstream builders](../../../internal/controller/infra/managed/kafka/bufstream/spec.go),
[names](../../../internal/controller/infra/managed/kafka/bufstream/naming.go),
[storage selection and writes](../../../internal/controller/infra/managed/kafka/bufstream/write.go).

### External services and supplied resources

External MySQL, Redis, object-store, and ClickHouse instances produce connection
Secrets in the WandB namespace. Their current default names are respectively
`wandb-mysql-connection`, `wandb-redis-connection`,
`wandb-objectstore-connection`, and `wandb-clickhouse-connection`. Non-default
instances append `-<instance-key>`. These names are not installation-unique;
detect collisions instead of adopting or deleting another installation's Secret.

Only the generated output Secrets are eligible for retention/purge. Supplied
`ValueOrSecret` input Secrets, `create: false` ServiceAccounts, image-pull
Secrets, external databases/buckets, cloud identities, StorageClasses,
namespaces, CRDs, and shared controllers/RBAC are not purge targets. Merely
referencing a resource does not authorize ownership, annotation changes, or
deletion. Purge must not recursively delete data from an external bucket.

PersistentVolumes follow the cluster's reclaim policy when an inventoried PVC
is deleted. Purge is not a promise to erase external disks or backups; do not
delete arbitrary PVs to force that outcome.

Sources: [external connection writers](../../../internal/controller/infra/external),
[shared connection Secret handling](../../../internal/controller/infra/external/common.go).

## Dependencies and policy validation

Use the same instance-resolution rules as provisioning: a requested key wins if
present; otherwise select `default`. An empty or invalid named entry must not be
silently treated as absent. Record the resolved instance key as well as resource
identities; a later fallback change must not lose the previous dependency.

Reject a configuration if a retained instance needs an operator-managed
dependency that is scheduled for purge. Report both the consumer and dependency
configuration paths. Do not silently upgrade a purge policy to detach or delete
the dependency and call the retained instance healthy.

| Combination | Required decision |
| --- | --- |
| ClickHouse detach, selected managed object store purge | Reject, including when selection falls back to `default` |
| Kafka detach, selected managed object store purge | Reject, including when selection falls back to `default` |
| Retained ClickHouse using static credentials from an external connection Secret scheduled for purge | Reject the generated-Secret conflict; the external bucket itself is never a purge target |
| Retained Bufstream using its own retained credential copy and an external bucket | No managed-service purge conflict; validate the actual credential/identity references |
| Consumer purge, dependency detach | Allow; delete the consumer without deleting its retained dependency |
| Two consumers share a generated identity/configuration resource, one detach and one purge | Reject deleting that shared dependency; component labels alone do not establish exclusive ownership |
| Customer-supplied dependency | Never purge it; check availability without claiming ownership |

Validate at admission and again before executing a deletion plan. Existing
incompatible configurations need a migration path: allow metadata/status and
finalizer progress, surface the conflict, and withhold deletion readiness until
the conflict is resolved. Admission must not strand a legacy CR by rejecting
every unrelated update. An already-deleting legacy installation needs an explicit
recovery procedure; do not purge conflicting dependencies automatically. Both
served API versions must enforce the same contract.

## Deletion and migration acceptance criteria

1. Complete inventory, protect Argo CD association, and remove all WandB
   ownership paths before declaring retention readiness. An admission DELETE
   guard must be in place before migrating legacy installations. An already
   terminating resource is a reported retention failure, not a repairable detach.
2. Persist a deletion plan from the latest accepted policies and inventory;
   freeze retention-relevant changes once deletion begins. Account for removed
   or renamed instances from recorded history. Removing an instance from spec
   is not, by itself, authorization to purge it.
3. Process every inventoried object independently. A missing or already detached
   primary CR must not skip its remaining configuration, credentials, or claims.
   Preserve unrelated owner references. The shared update helper must explicitly
   remove legacy references; omitting them from desired state is insufficient.
4. Stop workloads before deleting their required configuration/storage. Purge
   consumers before their dependencies, using exact resource identities and UID
   preconditions. Retry conflicts and API errors; do not delete a replacement
   resource at a reused name. Finish only when required cleanup is observed.
   A late vendor-created PVC is eligible for a new inventory record only after
   a persisted purge intent for the old UID, with the complete installation
   association and independently matching owner or StatefulSet-template
   provenance. Name reuse alone never authorizes deleting a replacement.
5. Retain provenance after detachment. A new WandB CR with the same name has a
   different UID and must not automatically acquire the old inventory. Define
   an explicit, verified readoption workflow. Older operators that restore
   owner references require a documented rollback restriction.

The [v2 reconcile path](../../../internal/controller/reconciler/reconcile_v2.go)
now delegates deletion to the [durable lifecycle](../../../internal/controller/retention/finalize.go).
The scoped infrastructure client blocks legacy cleanup outside the verified
inventory and explicitly removes ownership during resource update merging.
The inventory includes the MySQL ConfigMap, ClickHouse Keeper/ServiceAccount,
and Kafka ServiceAccount/SCC binding as well as discovered vendor descendants.

## Argo CD integration

For annotation tracking, copy the parent CR's exact
`argocd.argoproj.io/tracking-id` and, when present,
`argocd.argoproj.io/installation-id` onto generated infrastructure roots. The
tracking ID intentionally still identifies the WandB CR. Argo CD supports this
non-self-referencing association without a Kubernetes owner reference; roots
appear at the top level of the Application tree. Internal owner references
provide the hierarchy beneath those roots.

Merge `Prune=false,Delete=false` into the generated roots' Argo CD sync options
for **both** retention policies. Non-self-referencing tracking prevents sync
pruning, but it must not be relied on to prevent Application cascade deletion.
The operator remains responsible for explicit purge. Preserve unrelated options;
copy only selected association metadata, never arbitrary hooks or sync settings.

Use `annotation` or `annotation+label` tracking. Label-only association is not
enabled: vendor controllers can propagate an Application's instance label to
new children before the operator can add deletion protection to them. This
creates another prune race. Neither the default instance label nor a custom
tracking label is copied from the WandB CR. `IgnoreExtraneous` only changes sync
status and does not close that deletion race. Customers using label-only
tracking must configure Argo CD's `application.resourceTrackingMethod` as
`annotation` or `annotation+label` for this association. Preserving a namespace
and required CRDs is a separate Argo CD application-layout requirement. Keep
the vendor-controller releases available as well: deleting the Application that
installs those controllers is outside the tested WandB-CR deletion flow and
prevents retained services from being reconciled or restarted by their vendors.

References: [non-self-referencing tracking](https://argo-cd.readthedocs.io/en/stable/user-guide/resource_tracking/#non-self-referencing-annotations),
[sync deletion options](https://argo-cd.readthedocs.io/en/stable/user-guide/sync-options/#no-resource-deletion).

## Test coverage and completion criteria

Unit and envtest coverage belongs in this repository. All real-cluster coverage
belongs in **WESTest scenarios**, reusing its Kind provisioning and W&B deployment
logic. Do not add another standalone Kind retention harness. Envtest cannot
prove garbage-collector behavior because it does not run the controller manager.

| Layer | Required coverage |
| --- | --- |
| Unit: policy/dependency resolution | Defaults, explicit overrides including an empty policy object, invalid values, every named instance, fallback selection, external inputs versus generated outputs, shared dependencies, and conflicting policies |
| Unit: inventory/builders | Every direct resource listed above; mode changes; custom names/namespaces; shortened names; customer-supplied resources; UID/instance association; no ownership path to WandB; preserve valid internal references |
| Unit: lifecycle | Missing/already-detached roots, retries after each mutation, conflicts/forbidden errors, changed UIDs, removed/renamed instances, mixed policies in one namespace, and dependency ordering |
| Unit: Argo metadata | Exact tracking ID/installation ID, sync-option merging, no hooks, both policies, supported tracking modes and custom keys |
| Envtest | Persisted metadata migration, finalizer and deletion-plan progress, status conditions, DELETE admission, both API versions, concurrent updates, and rejection of changes after deletion begins |
| WESTest: detach | Each supported component/mode under background, foreground, and orphan deletion; retained roots/configuration/credentials/storage survive and the CR finishes deleting |
| WESTest: purge/mixed | Each propagation mode; full purge inventory disappears; retained sibling instances and customer-supplied resources remain; retained backing dependencies stay usable |
| WESTest: recovery | Operator pause/restart, partial progress, baseline-to-candidate upgrade, legacy inventory, and explicit readoption |
| WESTest: Argo CD | CR removed from Git with automated prune, and Application cascade deletion as separate cases; tree association, sync status, and retention/purge results for supported tracking modes |

WESTest validators must capture resource UIDs, required Secret/ConfigMap content
digests, claim identities, and ownership before deletion. After deletion, assert
that retained roots, configuration, credentials, and PVCs have the same UIDs and
no deletion timestamp. These deletion assertions run before deliberate restart.
During explicit readoption, vendor workloads may reconcile or be recreated
(Altinity can recreate Keeper StatefulSets when adoption metadata changes), and
vendor runtime ConfigMaps may update Pod/IP/topology state. Recovery must still
preserve root, configuration, credential and claim identities, credential and
operator-created configuration contents, workload readiness, and stored data.
Verify real data reads/writes and restartability, not only resource counts or
the existence of the primary CR. Avoid logging credential contents.

At least one detach scenario must pause WandB reconciliation while leaving
admission available, delete the CR, and prove the cluster's garbage collector
is active by deleting a disposable owner/dependent pair. Retained resources
must survive that interval; resume reconciliation and assert final completion.
This distinguishes structural retention from winning a finalizer race.

Run all assertions before WESTest teardown removes namespaces or the cluster.
Add reusable scenario validators, strict parameter decoding, scenario-loader
golden snapshots, and authoring documentation. Run a small representative set
on PRs and the full component/propagation/Argo matrix nightly and before release.
OpenShift-specific SCC behavior needs its own platform coverage; Kind validates
only the generated binding contract, not SCC admission.

References: [Kubernetes foreground deletion](https://kubernetes.io/docs/concepts/architecture/garbage-collection/#foreground-cascading-deletion),
[finalizer limitations](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/#how-finalizers-work),
[envtest limitations](https://book.kubebuilder.io/reference/envtest).

## Implementation sequence

The DELETE guard matches both served versions exactly and reads canonical v2
state without converting the incoming spec. Its new endpoint fails closed on
older replicas during rolling upgrades. Conversion preserves the canonical spec
when a native v2 installation is deleted through v1. Install the new webhook
configuration before migration; do not delete installations during an upgrade
until the guard is deployed and `RetentionReady=True` for their generation.
Do not roll back to an ownership-adding operator after migration without a
separate rollback procedure and blocked deletion window.

1. **This document:** establish the contract, direct-resource inventory,
   dependency rules, and test acceptance criteria. Vendor-created inventory
   discovery must be verified against the deployed versions in WESTest.
2. Separate management association from ownership, update watches, and remove
   legacy references explicitly. Build per-component inventory adapters from
   the source inventory above.
3. Implement the durable, retryable lifecycle and dependency validation, including
   exact instance scoping, safe purge ordering, and readoption.
4. Implement Argo CD association and deletion protection, including tracking-mode
   compatibility documentation and metadata tests.
5. Implement migration readiness and DELETE admission for both served versions;
   add policy enum validation and generated deployment/CRD changes. Deploy the
   guard before removing legacy ownership and document rollback constraints.
6. Implement operator unit/envtest coverage alongside each change, and add the
   real-cluster scenario families in WESTest.
7. Wire PR/nightly/release coverage and upgrade fixtures into CI. Release the
   ownership change, explicit purge, migration guard, and Argo protection
   together; ownership removal alone would break purge.
