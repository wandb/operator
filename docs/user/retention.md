# Retention and recovery

`spec.retentionPolicy.onDelete` controls how Operator handles managed backing
infrastructure when the W&B resource is deleted. Its default is `detach`.

| Policy | Meaning |
| --- | --- |
| `detach` | Remove W&B ownership from managed infrastructure so it can survive CR deletion |
| `purge` | Run the managed infrastructure deletion path; dependent data may be deleted |

A managed instance can override the top-level policy:

```yaml
spec:
  retentionPolicy:
    onDelete: detach
  mysql:
    default:
      managedMysql:
        retentionPolicy:
          onDelete: detach
```

Retention does not preserve a complete running W&B application. Application
resources, Operator-owned routes, and Console are removed during deletion or
owner garbage collection. External services remain under their provider's
ownership. Preserving infrastructure is also not a backup or a tested restore.

## Before removing a deployment

1. Record the W&B resource, release values, infrastructure resource names,
   connection Secrets, and PVCs needed for recovery.
2. Back up the data services and verify that the backups can be restored.
   Include object storage used by managed ClickHouse, as well as application
   objects and database state.
3. Inspect top-level and per-instance retention settings and the backing
   controllers' own PVC retention behavior.
4. Keep Operator and the backing controllers running while deleting the W&B CR
   so finalizers can finish their work.
5. Verify the expected resources and data remain before uninstalling controllers.

If the Operator manages multiple W&B resources, removing its Helm release affects
all of them. Uninstalling a release does not replace the CR finalization process.
Removing a namespace or CRD can delete resources beyond a single deployment.

## Investigate a stuck deletion

```bash
kubectl get weightsandbiases wandb -n wandb -o yaml
kubectl logs deployment/wandb-operator -n wandb-operators --tail=200
kubectl get events -n wandb --sort-by=.lastTimestamp
```

Check the deletion timestamp, finalizers, backing controller availability, and
reported errors. Fix the failing cleanup step before considering manual finalizer
removal; bypassing a finalizer skips the code responsible for retention.

## Restore and reconnect

Restore procedures depend on the database, object-store provider, and W&B version.
This repository does not yet provide a fully tested recovery runbook covering
all of those combinations. Restore service data and credentials consistently,
then use verified connection settings when reconnecting W&B. Do not assume that
creating a new CR with the same name automatically recovers a detached deployment.
