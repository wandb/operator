# Debug local development

First identify whether the failure is in the local toolchain, the installed
schema, reconciliation, or the W&B application. Use [user troubleshooting](../user/troubleshooting.md)
for deployment failures that also occur outside Tilt.

## Check the local rebuild and schema

1. Confirm the intended context with `kubectl config current-context` and compare
   it with the allowed contexts in `tilt-settings.star`.
2. Inspect `Operator-Codegen`, `Operator-Build`, and the `wandb-operator` Helm
   resource in Tilt. An API edit can rebuild the binary without updating CRDs.
3. Follow the [generation workflow](workflow.md), then inspect the installed
   schema with `kubectl explain weightsandbiases.spec --api-version=apps.wandb.com/v2`.
4. Inspect the CRD installer Job and webhook certificate if chart updates or
   admission fail.

```bash
kubectl get jobs,pods,certificate -n wandb-operators
kubectl logs deployment/wandb-operator -n wandb-operators --tail=200
kubectl get weightsandbiases wandb -n wandb -o yaml
```

## Trace reconciliation

Read infrastructure status by instance, then MySQL initialization and migration
status, then the generated Applications and their Deployments. Compare desired
and observed generation when a resource appears ready but a new change has not
landed. Look for a requeue or failing condition at the layer that owns the resource.

Use `kubectl describe` for scheduling, PVC, image-pull, and probe events. Check
application container logs when the controller has successfully created the
workload but the application is failing.

For host debugging, inspect `make run` and `make run-local-webhook` in the
[Makefile](../../Makefile). The optional [colocated local webhook guide](../../config/local-dev/README.md)
contains older Tilt settings; the current manager entrypoint is `cmd/manager`.
Do not run two controllers against the same test resource while debugging.

## Reset a development environment

Use the [Tilt cleanup procedure](setup.md#cleaning-up-tilt), which leaves the
operators running long enough to finalize the W&B resource. Inspect finalizer
errors if cleanup stalls. A development reset is destructive to local fixtures
and is not a production recovery procedure.
