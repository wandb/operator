# Troubleshoot a deployment

Start with the W&B resource and work through its dependencies. Examples use
`wandb` in namespace `wandb`, with Operator installed in `wandb-operators`.

```bash
kubectl get weightsandbiases wandb -n wandb -o yaml
kubectl get pods,deployments,statefulsets,jobs,pvc -n wandb
kubectl get events -n wandb --sort-by=.lastTimestamp
kubectl logs deployment/wandb-operator -n wandb-operators --tail=200
```

Compare the `Ready` condition's `observedGeneration` with `metadata.generation`.
An old successful status is not evidence that the newest change has completed.

| Failure stage | What to inspect |
| --- | --- |
| Helm cannot find Certificate or Issuer kinds | cert-manager and its CRDs must be installed first |
| CRD installer Job fails | Job logs, RBAC, and conflicting field ownership; avoid deleting CRDs |
| Admission webhook unavailable | Operator readiness, serving certificate, webhook CA bundle, and Service endpoints |
| CR rejected | Required hostname/version, named infrastructure `default` entries, networking blocks, and value-or-secret shapes |
| Server manifest cannot be fetched | Pinned W&B version, repository, registry credentials, and Operator CA trust |
| Pods report image pull failures | Mirrored image layout, workload pull credentials, and node registry trust |
| Infrastructure is not ready | Per-instance status, backing controller logs, PVC binding, resource capacity, and connection Secrets |
| Migration blocks rollout | The migration Job's status, container logs, and requested W&B version |
| Application pods are not ready | Deployment rollout, pod events, probes, and application logs |
| Public URL fails | Ingress/Gateway status, HTTPRoute acceptance, DNS, certificates, and backend Services |
| Artifact browser download fails | Presigned URL host, external reachability, TLS trust, and CORS |

## Inspect a migration failure

```bash
kubectl get jobs -n wandb \
  -l app.kubernetes.io/instance=wandb,app.kubernetes.io/component=migration
```

Set `MIGRATION_JOB` to the failed Job name from that list, then inspect it:

```bash
kubectl describe job/"$MIGRATION_JOB" -n wandb
kubectl logs job/"$MIGRATION_JOB" -n wandb -c migrate
```

A similarly named Deployment is not evidence of migration success. Do not edit
migration tables or clear a partially applied
version without a migration-specific recovery procedure and verified backup.

## Collect evidence

Record the Operator chart/image version, W&B version, affected resource names,
current generation and status, relevant events, and failing Job or pod logs.
Inspect exports before sharing: CRs can contain licenses or literal credentials,
and logs may include application data. Include the failure stage and last change.

Use the specialized [Console](console.md), [monitoring](monitoring.md),
[networking](networking.md), and [private registry](restricted-environments.md)
guides for their configuration and verification steps.
