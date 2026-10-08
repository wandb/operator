# Networking and TLS

Set `spec.wandb.hostname` to the public URL, including `http://` or `https://`.
Choose `spec.networking.mode: ingress` or `gateway`. An empty mode creates no
Operator-managed public route. Configure only the block for the selected mode.

The following examples are fragments to merge into your complete W&B resource.
Install the corresponding routing controller and class before applying them.

## Managed Ingress

```yaml
spec:
  wandb:
    hostname: https://wandb.example.com
  networking:
    mode: ingress
    ingress:
      ingressClassName: nginx
      managed: true
    tls:
      secretName: wandb-tls
```

`ingressClassName` is required. `managed` defaults to `true` when the Ingress
block exists and mode is `ingress`. Operator creates one consolidated Ingress
containing the routes declared by the server manifest. Create the TLS Secret in
the W&B namespace. To use cert-manager's ingress integration, configure the
appropriate issuer as well:

```yaml
spec:
  networking:
    tls:
      secretName: wandb-tls
      certManager:
        clusterIssuer: your-cluster-issuer
```

Use `issuer` instead for a namespaced Issuer. The selected issuer must already
exist and be able to issue a certificate for your hostname.

## External Ingress

Set `spec.networking.ingress.managed: false` when you own the Ingress. Operator
does not need its name and does not create, update, or delete it. You must route
the application's paths to the generated Services yourself, including `/console`
when Console is enabled. `ingressClassName` is still required so Operator can
configure controller-specific backend Service settings.

Changing an Operator-managed Ingress to `managed: false` removes the consolidated
Ingress owned by that W&B resource. Arrange the replacement route before changing
ownership.

### AWS Load Balancer Controller

For an IngressClass whose controller is `ingress.k8s.aws/alb`, Operator derives
per-Service health-check annotations from the first HTTP readiness probe of each
application. Per-Service settings allow different backends to use different
health paths and ports. Exec, TCP, and gRPC probes are not translated. The
IngressClass must exist for controller-specific configuration to be detected.

## Managed Gateway

Install Gateway API CRDs and a controller providing the GatewayClass used below.
Operator creates the Gateway resource and application HTTPRoutes; it does not
install that controller through this CR.

```yaml
spec:
  wandb:
    hostname: https://wandb.example.com
  networking:
    mode: gateway
    tls:
      secretName: wandb-tls
    gatewayAPI:
      listenerName: https
      gateway:
        managed: true
        gatewayClassName: nginx
        listeners:
          - name: https
            hostname: wandb.example.com
            port: 443
            protocol: HTTPS
            tls:
              mode: Terminate
              certificateRef:
                name: wandb-tls
```

Use your actual GatewayClass and provide the certificate Secret in the W&B
namespace. Explicit listeners make the port, protocol, and attachment name clear.
When listeners are omitted, Operator derives a listener named `http` from the
hostname's scheme and port, including when the scheme is HTTPS.

## Existing Gateway

```yaml
spec:
  networking:
    mode: gateway
    gatewayAPI:
      listenerName: https
      gateway:
        managed: false
        gatewayRef:
          name: shared-gateway
          namespace: gateway-system
```

The Gateway and listener must already exist and accept HTTPRoutes from the W&B
namespace. Configure certificates and listener policy on that external Gateway.
Operator attaches routes without managing the Gateway. Managed Gateways require
`gatewayClassName`; external Gateways require `gatewayRef`.

## Implementation-specific TLS options

An explicit managed listener accepts `tls.options`. For example, the existing
GKE configuration recipe attaches a Compute SSL certificate using:

```yaml
tls:
  mode: Terminate
  options:
    networking.gke.io/pre-shared-certs: wandb-cert
```

Choose options supported by your Gateway controller and class. Regional GKE
Certificate Manager certificates use `networking.gke.io/cert-manager-certs`.
Certificate maps use the `networking.gke.io/certmap` Gateway annotation under
`spec.networking.annotations`, rather than a listener option.

## Verify and change routing

```bash
kubectl get ingress -n wandb
kubectl get gateway,httproute -n wandb
kubectl get weightsandbiases wandb -n wandb -o yaml
```

Run the commands appropriate to your installed APIs. Check the controller-assigned
address, route acceptance and backend references, DNS, and certificate trust.
Switching modes cleans up resources owned by Operator for the inactive mode;
plan for route replacement and test application access after the change.
