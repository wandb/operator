# Migrating from Operator v1 to v2

Use this runbook for an existing W&B deployment managed by Operator v1. Treat
the migration as a controlled cutover: preserve stateful dependencies, run one
operator controller at a time, and keep a tested rollback path until v2 is
healthy.

Operator v2 does not uninstall the v1 Helm releases or modify migration
metadata inside MySQL or ClickHouse. Those actions can destroy data or make
rollback impossible and must remain explicit operator decisions.

## Before the change window

1. Record the v1 operator and W&B Helm release names, namespaces, chart
   versions, values, and current replica counts.
2. Back up MySQL, object storage, ClickHouse, and Redis according to the
   provider's restore procedure, and verify that the backups can be read.
3. Use `spec.retentionPolicy.onDelete: detach` in the v2 resource while
   validating the migration. Do not delete the v1 resource or uninstall a
   state-owning Helm release as a migration step.
4. Inspect the converted `apps.wandb.com/v2` `WeightsAndBiases` resource before
   applying it:

   - Preserve the public `spec.wandb.hostname`.
   - Verify every external infrastructure selector names an existing Secret
     and key.
   - Verify Redis has a non-empty host and port (or a complete URL), not only a
     password.
   - Verify object-store endpoint, bucket, region, TLS, and path-style settings.
   - Remove `status` and other server-populated metadata from a live-resource
     export before applying it as desired state.

5. Determine whether the v1 W&B Helm release owns Redis or another dependency
   that v2 will continue using:

   ```bash
   helm get manifest <v1-wandb-release> -n <wandb-namespace>
   kubectl get statefulset,service,pvc -n <wandb-namespace>
   ```

   If the release owns Redis, keep that release installed until Redis has been
   migrated to an external service or rehomed outside the release's ownership.
   Pointing v2 at the v1 Redis Service does not make it safe to uninstall the
   v1 release.

## Enter single-controller mode

Do not let Operator v1 and v2 reconcile the same W&B deployment concurrently.
The v1 controller can restore legacy values or Secrets while v2 is trying to
apply the converted configuration.

1. Identify the v1 controller Deployment from its Helm release and save its
   replica count.
2. Scale that Deployment to zero.
3. Confirm no v1 controller pods remain before applying the v2 resource.

For example:

```bash
kubectl -n <v1-operator-namespace> scale \
  deployment/<v1-controller-deployment> --replicas=0
kubectl -n <v1-operator-namespace> get pods
```

Do not uninstall the v1 operator yet. Leaving the release installed makes the
controller rollback reversible.

## Install Operator v2 and apply the resource

Pin a reviewed version from the Operator v2 OCI repository:

```bash
helm upgrade --install wandb-operator \
  oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator \
  --version <operator-version> \
  --namespace <v2-operator-namespace> \
  --create-namespace

kubectl apply -f <weightsandbiases-v2.yaml>
```

The `wandb/operator` chart in `charts.wandb.ai` is Operator v1 and is not an
Operator v2 upgrade source.

The v2 chart installs and upgrades its CRDs with server-side apply. If the CRD
installer Job fails, inspect that Job and resolve the ownership or RBAC error;
do not delete existing CRDs as a recovery shortcut.

## Wait for the cutover gates

Applications are gated on infrastructure readiness, MySQL initialization, and
the W&B migration Jobs. Do not switch traffic or remove v1 workloads until all
of these checks pass:

```bash
kubectl -n <wandb-namespace> get wandb <wandb-name> -o yaml
kubectl -n <wandb-namespace> get jobs \
  -l app.kubernetes.io/instance=<wandb-name>,app.kubernetes.io/component=migration
kubectl -n <wandb-namespace> get applications
kubectl -n <wandb-namespace> get deployments
```

Confirm:

- Every configured infrastructure status is ready.
- `status.wandb.migration.ready` is `true`, its version matches
  `spec.wandb.version`, and every migration Job succeeded.
- Every v2 application Deployment is fully rolled out with available replicas.
- The public route targets the intended v2 Services and no longer returns
  transient 5xx responses.

If a migration Job fails, collect its status and logs:

```bash
kubectl -n <wandb-namespace> describe job/<migration-job>
kubectl -n <wandb-namespace> logs job/<migration-job> -c migrate
```

Do not infer migration success from a similarly named Deployment. Do not
automatically clear `partially_applied_version` or edit migration tables: that
requires a migration-specific recovery decision and a verified backup.

## Smoke test before cleanup

Run the tests through the customer-facing hostname:

1. Sign in and load the main UI.
2. Create an SDK run using an existing, valid entity.
3. Upload and download an artifact through the SDK.
4. Download the artifact in a browser using the presigned URL.
5. Check the browser request for the expected external object-store endpoint,
   trusted TLS, and successful CORS headers.
6. Exercise Runs and Weave views used by the deployment.

An SDK upload alone is not sufficient for S3-compatible storage. Browser
downloads can still fail when the external endpoint or CORS policy is wrong.

## Retire v1 in the safe order

1. Keep the v1 controller at zero replicas.
2. Verify the v2 route and workloads remain healthy for the agreed observation
   period.
3. Inventory remaining v1 Deployments and scale any that are still serving
   traffic to zero. Operator v2 removes the known legacy `*-bc` Deployments
   only after all desired v2 Deployments are ready; verify rather than assume
   that all v1 workloads are covered.
4. Uninstall the v1 operator release only after the rollback window no longer
   requires it.
5. Uninstall the v1 W&B release only after confirming it owns no Redis,
   database, object-store, ClickHouse, PVC, Secret, or other resource still
   used by v2.

Never uninstall the v1 W&B release while v2 still references its Helm-owned
Redis. The uninstall deletes the Redis workload even if the v2 resource calls
it "external."

## Rollback

Rollback must also use one controller at a time:

1. Stop traffic to the v2 workloads or restore the previous route.
2. Scale the v2 operator controller to zero.
3. Restore the saved v1 configuration and workload replica counts.
4. Scale the v1 operator controller back to its saved replica count.
5. Verify database compatibility before rolling the W&B application version
   back; a completed forward migration may not be reversible by changing only
   the image tag.
6. Re-run the UI, SDK, artifact, and browser-download smoke tests.

Do not delete the v2 resource or its dependencies during rollback. Keep them
detached until the incident is understood and the retained data is no longer
needed.
