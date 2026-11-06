# Collect a support bundle

Operator enables support-bundle collection templates by default. It installs a
suspended CronJob, collection configuration, and permissions for the Lumen
collector. The CronJob does not run on a schedule; a collection must be launched
explicitly. Collection readiness is separate from W&B readiness.

## Configuration and prerequisites

The default object-store connection must be available before the template can
become ready. The collector uses that connection for bundle storage. The cluster
must also support `admissionregistration.k8s.io/v1` ValidatingAdmissionPolicy and
ValidatingAdmissionPolicyBinding resources used to constrain collection helpers.

To opt out, merge this fragment into the W&B resource:

```yaml
spec:
  supportBundle:
    enabled: false
```

Set `enabled: true` to enable it again. Omit `spec.supportBundle.version` to use
Operator's pinned Lumen version; an explicit version overrides the collector image
tag. For a private registry, mirror the selected `wandb/lumen` image and configure
`spec.global.imageRegistry` and `imagePullSecrets` as described in
[restricted environments](restricted-environments.md).

## Launch and inspect a collection

For a W&B resource named `wandb` in namespace `wandb`, first inspect its status:

```bash
kubectl get weightsandbiases wandb -n wandb -o jsonpath='{.status.supportBundle}'
```

Wait for `ready: true`, and use the `cronJob` name from that status. The status also
reports the selected image/version, whether a collection is running, and the last
successful time. With permission to create Jobs, launch a one-off collection:

```bash
SUPPORT_CRONJOB=$(kubectl get weightsandbiases wandb -n wandb \
  -o jsonpath='{.status.supportBundle.cronJob}')
SUPPORT_JOB="wandb-support-$(date +%s)"
kubectl create job "$SUPPORT_JOB" -n wandb --from="cronjob/$SUPPORT_CRONJOB"
kubectl logs -n wandb -f "job/$SUPPORT_JOB" -c collector
kubectl get pods -n wandb -l "batch.kubernetes.io/job-name=$SUPPORT_JOB" -o yaml
```

The collector writes its result to the Pod's termination message. Inspect that
result and the logs to locate the output and diagnose failed collection or storage
access. The Job has a 30-minute active deadline and a one-day completion TTL;
save the relevant result before the Job and Pod are removed.

## Collection scope and troubleshooting

The default collection includes cluster information, Kubernetes resources, W&B
namespace Pod logs, and DNS checks. When telemetry is enabled and ready, it also
includes data from the Victoria stack. The collector's Kubernetes read rules omit
Secrets, but logs and resource configuration can still contain sensitive material;
review a bundle before sharing it.

If the template is not ready, inspect Operator logs, the default object-store
status, admission-policy availability, and RBAC. Collection helpers need access
to the W&B namespace, `default` and `kube-system` for DNS diagnostics, and the
telemetry namespace when applicable. Support-bundle failures are retried without
blocking W&B readiness.

The [developer guide](../developer/support-bundles.md) describes ownership,
permissions, and the Console integration.
