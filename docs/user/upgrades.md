# Upgrade Operator and W&B

Operator and W&B are upgraded separately. The Helm release selects Operator;
`spec.wandb.version` selects the W&B server manifest and its migrations. For a v1
deployment, use [migration preparation and validation](migrating-v1-to-v2.md).

## Prepare

Review the target release notes and test the selected version combination in a
representative environment. Record the existing Operator chart version, complete
Helm values, W&B spec, infrastructure connections, and data-service versions.
Keep exports containing licenses or credentials in protected storage.

Back up the backing services and establish the restore procedure before a change
that runs data migrations. A Helm rollback does not reverse a W&B database
migration. See [retention and recovery](retention.md).

## Upgrade Operator

Set `OPERATOR_VERSION` to the reviewed target version. Update your complete saved
values file for any changed chart settings, then run:

```bash
helm upgrade wandb-operator \
  oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator \
  --version "$OPERATOR_VERSION" --namespace wandb-operators \
  -f operator-values.yaml --wait --timeout 10m
kubectl rollout status deployment/wandb-operator -n wandb-operators --timeout=300s
```

The chart's CRD installer updates CRDs. Inspect its Job if the upgrade fails;
resolve the reported ownership or RBAC problem rather than deleting existing
CRDs. Verify every W&B resource managed by the upgraded controller, not just the
Operator Deployment.

## Upgrade W&B

Update `spec.wandb.version` in the saved W&B resource to a specific published
version and confirm that its manifest and images are reachable. Then:

```bash
kubectl apply --dry-run=server -f wandb.yaml
kubectl diff -f wandb.yaml
kubectl apply -f wandb.yaml
kubectl get weightsandbiases wandb -n wandb -w
```

Confirm the `Ready` condition is for the new generation, the migration status
reports the requested version and success, and every application has rolled out.
Test sign-in, an SDK run, an artifact upload/download, and the W&B views used by
the deployment. Inspect migration Job logs on failure; do not clear migration
metadata as a generic retry mechanism.

## Recovery limits

The repository does not define a universal downgrade or data-restore procedure
for every W&B version and backing service. Establish that procedure for the
selected releases before upgrading. Retain the old configuration and backups,
but do not assume reapplying an old version will make migrated data compatible.
