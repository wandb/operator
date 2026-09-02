# Networking

`spec.networking.mode` selects how the W&B application is exposed. Leaving it
empty preserves the existing behavior and does not default or create an
Ingress.

## Ingress

Ingress mode always requires `spec.networking.ingress.ingressClassName`. The
operator reads that cluster-scoped `IngressClass` to determine which controller
implements the Ingress, so its ClusterRole requires `get`, `list`, and `watch`
on `networking.k8s.io/ingressclasses`.

By default, the operator creates and owns a consolidated Ingress:

```yaml
spec:
  networking:
    mode: ingress
    ingress:
      ingressClassName: nginx
      managed: true
```

The defaulting webhook sets `managed: true` only when mode is explicitly
`ingress`, the ingress block exists, and `managed` was omitted.

Set `managed: false` when another system or the user creates the Ingress:

```yaml
spec:
  networking:
    mode: ingress
    ingress:
      ingressClassName: alb
      managed: false
```

In external mode the operator does not need the external Ingress name and never
creates, updates, or deletes it. It still configures the W&B backend Services
for controller-specific health checks. Switching an operator-managed Ingress to
`managed: false` deletes only the consolidated Ingress owned by that
`WeightsAndBiases` resource.

### AWS Load Balancer Controller

When the selected `IngressClass.spec.controller` is `ingress.k8s.aws/alb`, the
operator translates each application's first HTTP readiness probe into the
corresponding `alb.ingress.kubernetes.io/healthcheck-*` annotations on its
Service. Service annotations take precedence over Ingress annotations, which
allows each backend in the consolidated Ingress to use its own path and port.

The operator propagates the HTTP/HTTPS protocol, path, port, interval, timeout,
healthy threshold, and unhealthy threshold when the value is supported by an
ALB target group. Kubernetes defaults below AWS minimums are omitted so AWS can
use a valid default. Exec, TCP socket, and gRPC readiness probes are not
translated in this implementation. HTTP headers, host overrides, and initial
delay have no equivalent AWS health-check annotation.

Application Services are reconciled using server-side apply with the
Application reconciler's field manager, `application-controller`. The operator
owns only the fields present in its partial Service configuration. API-assigned
values such as ClusterIPs and allocated NodePorts, and labels or annotations
owned by other controllers, remain untouched. Removing an operator-generated
annotation from the desired configuration removes that annotation from the
Service.
