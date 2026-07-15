# Plan: Ingress and Gateway API Support for WeightsAndBiases Operator

## Context

The W&B operator currently exposes services via NodePort (with an nginx-proxy feature flag) but has no support for Kubernetes Ingress or Gateway API resources. Users deploying W&B on-prem need standard ingress/routing to expose the application externally. The `ApplicationSpec` already defines an `IngressTemplate` field and `IngressStatus` in the status, but neither is wired up — no reconciliation logic exists.

This plan adds a top-level networking configuration to the `WeightsAndBiasesSpec` CR, manifest-level route declarations per application, and reconciliation logic in the V2 reconciler (for Ingress and Gateway) and the Application controller (for HTTPRoute). Ingress and Gateway API are supported as a mutually exclusive choice.

### Key architectural decisions

**1. Ingress is a single consolidated resource, not per-application.**

A Kubernetes Ingress naturally aggregates multiple path-based routing rules into a single resource. We create one Ingress for the entire W&B deployment with path rules routing to each application's Service backend. This is managed in `reconcileApplications()` inside `reconcile_v2.go` — the same place that already has visibility into all applications and their services.

**2. HTTPRoutes are per-application.**

Gateway API is designed for distributed route ownership — each team/service manages its own HTTPRoute. Each application gets its own HTTPRoute in the Application controller, attaching to the shared Gateway via `parentRef`.

**3. Managed vs. External Gateway.**

When the user selects Gateway API mode, the operator supports two sub-modes:

- **Managed Gateway** (`gateway.managed: true`): The operator creates and reconciles a `Gateway` resource in `ReconcileWandbManifest()` alongside other shared resources (service accounts, roles, etc.). The operator owns the full lifecycle — create, update, and delete on CR teardown.
- **External Gateway** (`gateway.managed: false` or omitted): The operator only creates `HTTPRoute` resources that attach to a user-provisioned `Gateway` via `gatewayRef`. The operator never touches the `Gateway` itself.

**4. Resource ownership summary.**

| Resource | Where reconciled | Cardinality |
|----------|-----------------|-------------|
| Ingress | `reconcile_v2.go` (`reconcileApplications()`) | One per W&B deployment |
| Gateway | `reconcile_v2.go` (`ReconcileWandbManifest()`) | One per W&B deployment (managed only) |
| HTTPRoute | `application_controller.go` | One per application |

---

## Step 1: Add Gateway API dependency

**File:** `go.mod`

Run `go get sigs.k8s.io/gateway-api` to add the Gateway API types. The project already uses `k8s.io/api v0.35.0` and `sigs.k8s.io/controller-runtime v0.22.4`, so Gateway API v1.2+ should be compatible.

---

## Step 2: Add Networking types to the WeightsAndBiases CR

**File:** `api/v2/weightsandbiases_types.go`

Add a `Networking` field to `WeightsAndBiasesSpec` and define the supporting types:

```go
type NetworkingMode string

const (
    NetworkingModeNone       NetworkingMode = ""
    NetworkingModeIngress    NetworkingMode = "Ingress"
    NetworkingModeGatewayAPI NetworkingMode = "GatewayAPI"
)

type NetworkingSpec struct {
    // Mode selects the networking strategy: "Ingress" or "GatewayAPI".
    // Empty/unset means no operator-managed ingress (preserves current NodePort behavior).
    // +kubebuilder:validation:Enum="";Ingress;GatewayAPI
    Mode NetworkingMode `json:"mode,omitempty"`

    // Ingress configures Kubernetes Ingress resources. Only used when Mode=Ingress.
    // +optional
    Ingress *IngressConfig `json:"ingress,omitempty"`

    // GatewayAPI configures Gateway API resources. Only used when Mode=GatewayAPI.
    // +optional
    GatewayAPI *GatewayAPIConfig `json:"gatewayAPI,omitempty"`

    // TLS configures TLS termination for both Ingress and Gateway API modes.
    // +optional
    TLS *TLSConfig `json:"tls,omitempty"`

    // Annotations applied to all generated Ingress or HTTPRoute resources.
    // +optional
    Annotations map[string]string `json:"annotations,omitempty"`
}

type IngressConfig struct {
    // IngressClassName sets the spec.ingressClassName field on Ingress resources.
    // +optional
    IngressClassName *string `json:"ingressClassName,omitempty"`
}

type GatewayAPIConfig struct {
    // Gateway configures the Gateway resource. Specifies either a managed
    // Gateway (created by the operator) or a reference to an external one.
    Gateway GatewayConfig `json:"gateway"`

    // ListenerName is the specific listener on the Gateway to attach HTTPRoutes to.
    // When the Gateway is managed, this should match one of the listeners defined
    // in the managed Gateway spec. When using an external Gateway, this selects
    // which listener on that Gateway to target.
    // +optional
    ListenerName *string `json:"listenerName,omitempty"`
}

type GatewayConfig struct {
    // Managed controls whether the operator creates and manages the Gateway resource.
    // When true, the operator creates a Gateway using the configuration below.
    // When false (default), gatewayRef must be set to reference an existing Gateway.
    // +kubebuilder:default=false
    Managed bool `json:"managed,omitempty"`

    // GatewayRef references an existing external Gateway. Required when managed=false.
    // +optional
    GatewayRef *GatewayReference `json:"gatewayRef,omitempty"`

    // GatewayClassName is the name of the GatewayClass to use.
    // Required when managed=true.
    // +optional
    GatewayClassName *string `json:"gatewayClassName,omitempty"`

    // Listeners defines the listeners on the managed Gateway.
    // If empty and managed=true, a default HTTPS listener is created using
    // the hostname from spec.wandb.hostname and the TLS secret from spec.networking.tls.
    // +optional
    Listeners []GatewayListener `json:"listeners,omitempty"`

    // Annotations applied to the managed Gateway resource.
    // +optional
    Annotations map[string]string `json:"annotations,omitempty"`
}

type GatewayReference struct {
    // Name of the Gateway resource.
    Name string `json:"name"`
    // Namespace of the Gateway resource. Defaults to the CR namespace if empty.
    // +optional
    Namespace string `json:"namespace,omitempty"`
}

// GatewayListener defines a listener on a managed Gateway.
// This is a simplified view of gatewayv1.Listener — the operator
// constructs the full Listener from these fields plus CR-level TLS config.
type GatewayListener struct {
    // Name is the listener name (e.g., "https", "http").
    Name string `json:"name"`
    // Port is the network port (e.g., 443, 80).
    Port int32 `json:"port"`
    // Protocol is the listener protocol (e.g., "HTTPS", "HTTP").
    Protocol string `json:"protocol"`
    // Hostname restricts this listener to a specific hostname. Optional.
    // +optional
    Hostname *string `json:"hostname,omitempty"`
    // TLS configures TLS for this listener. If nil and protocol is HTTPS,
    // the operator uses the top-level spec.networking.tls configuration.
    // +optional
    TLS *ListenerTLSConfig `json:"tls,omitempty"`
}

type ListenerTLSConfig struct {
    // Mode is the TLS mode (Terminate, Passthrough). Defaults to Terminate.
    // +optional
    Mode *string `json:"mode,omitempty"`
    // CertificateRef references the Secret containing the TLS certificate.
    // If nil, falls back to spec.networking.tls.secretName.
    // +optional
    CertificateRef *SecretRef `json:"certificateRef,omitempty"`
}

type SecretRef struct {
    Name      string `json:"name"`
    Namespace string `json:"namespace,omitempty"`
}

type TLSConfig struct {
    // SecretName is the name of a kubernetes.io/tls Secret for TLS termination.
    // +optional
    SecretName string `json:"secretName,omitempty"`

    // CertManager enables cert-manager annotations on Ingress resources.
    // +optional
    CertManager *CertManagerConfig `json:"certManager,omitempty"`
}

type CertManagerConfig struct {
    // ClusterIssuer is the name of the cert-manager ClusterIssuer to use.
    // +optional
    ClusterIssuer string `json:"clusterIssuer,omitempty"`
    // Issuer is the name of the cert-manager Issuer (namespace-scoped) to use.
    // +optional
    Issuer string `json:"issuer,omitempty"`
}
```

Add field to `WeightsAndBiasesSpec`:
```go
type WeightsAndBiasesSpec struct {
    // ... existing fields ...

    // Networking configures how the W&B application is exposed externally.
    // +optional
    Networking NetworkingSpec `json:"networking,omitempty"`
}
```

---

## Step 3: Add HTTPRouteTemplate to Application types

**File:** `api/v2/application_types.go`

Add `HTTPRouteTemplate` for Gateway API. This is per-application since Gateway API supports distributed route ownership. The existing `IngressTemplate` field is left in place but remains unused — Ingress is managed as a consolidated resource at the V2 reconciler level (see Step 5).

```go
import (
    // add: gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type ApplicationSpec struct {
    // ... existing fields ...

    // HTTPRouteTemplate is the desired HTTPRoute spec. Nil means no HTTPRoute.
    // +optional
    HTTPRouteTemplate *HTTPRouteTemplateSpec `json:"httpRouteTemplate,omitempty"`
}

// HTTPRouteTemplateSpec wraps the key fields needed to build an HTTPRoute
// rather than embedding the full upstream spec (which has server-managed fields).
type HTTPRouteTemplateSpec struct {
    ParentRefs []gatewayv1.ParentReference `json:"parentRefs"`
    Hostnames  []gatewayv1.Hostname        `json:"hostnames,omitempty"`
    Rules      []gatewayv1.HTTPRouteRule   `json:"rules,omitempty"`
}

type ApplicationStatus struct {
    // ... existing fields ...

    // HTTPRouteStatus summarizes the observed state of the managed HTTPRoute.
    // +optional
    HTTPRouteStatus *HTTPRouteStatusSummary `json:"httpRouteStatus,omitempty"`
}

type HTTPRouteStatusSummary struct {
    // Accepted indicates if the HTTPRoute was accepted by the Gateway.
    Accepted bool `json:"accepted,omitempty"`
}
```

---

## Step 4: Add route declarations to the Manifest

**File:** `pkg/wandb/manifest/manifest.go`

Add an optional `Ingress` field to the manifest `Application` struct so server manifests can declare per-application routing needs. This is used by both the consolidated Ingress builder (Step 5c) and the per-app HTTPRoute builder (Step 5d).

```go
type Application struct {
    // ... existing fields ...

    // Ingress declares per-application routing configuration.
    // Used by the operator to build Ingress path rules or HTTPRoute resources
    // when networking is enabled in the CR spec.
    Ingress *AppIngressSpec `yaml:"ingress,omitempty"`
}

type AppIngressSpec struct {
    // Paths defines the URL path prefixes this application should serve.
    // Defaults to ["/"] if empty.
    Paths []string `yaml:"paths,omitempty"`

    // ServicePort is the name or number of the backend service port.
    // If empty, the first port from the Service spec is used.
    ServicePort string `yaml:"servicePort,omitempty"`

    // PathType controls path matching semantics ("Prefix", "Exact", "ImplementationSpecific").
    // Defaults to "Prefix".
    PathType string `yaml:"pathType,omitempty"`
}
```

---

## Step 5: Networking reconciliation in reconcile_v2.go

**File:** `internal/controller/v2/reconcile_v2.go` and new file `internal/controller/v2/gateway.go`

### 5a: Reconcile managed Gateway in ReconcileWandbManifest

The Gateway is a shared namespace-level resource. It is reconciled in `ReconcileWandbManifest()` after service account setup but before `reconcileApplications()`:

```go
// In ReconcileWandbManifest(), after createOrUpdateRoleBinding() and before runMigrations():
if wandb.Spec.Networking.Mode == apiv2.NetworkingModeGatewayAPI {
    if err := reconcileGateway(ctx, client, wandb); err != nil {
        logger.Error(err, "Failed to reconcile Gateway")
        return ctrl.Result{}, err
    }
}
```

New file `internal/controller/v2/gateway.go`:

```go
func reconcileGateway(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases) error {
    gwConfig := wandb.Spec.Networking.GatewayAPI.Gateway

    if !gwConfig.Managed {
        if gwConfig.GatewayRef == nil {
            return fmt.Errorf("gatewayAPI.gateway.gatewayRef is required when managed=false")
        }
        return validateExternalGatewayExists(ctx, c, wandb, gwConfig.GatewayRef)
    }

    // Managed gateway: build and reconcile
    gatewayName := fmt.Sprintf("%s-gateway", wandb.Name)
    hostname := parseHostname(wandb.Spec.Wandb.Hostname)

    desired := &gatewayv1.Gateway{}
    desired.Name = gatewayName
    desired.Namespace = wandb.Namespace
    desired.Spec.GatewayClassName = gatewayv1.ObjectName(*gwConfig.GatewayClassName)

    if len(gwConfig.Listeners) > 0 {
        desired.Spec.Listeners = buildListenersFromConfig(gwConfig.Listeners, wandb)
    } else {
        desired.Spec.Listeners = buildDefaultListeners(hostname, wandb)
    }

    desired.Labels = map[string]string{
        "app.kubernetes.io/managed-by": "wandb-operator",
        "app.kubernetes.io/instance":   wandb.Name,
    }
    desired.Annotations = gwConfig.Annotations

    if err := controllerutil.SetOwnerReference(wandb, desired, c.Scheme()); err != nil {
        return err
    }

    current := &gatewayv1.Gateway{}
    err := c.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: wandb.Namespace}, current)
    if err != nil {
        if apiErrors.IsNotFound(err) {
            return c.Create(ctx, desired)
        }
        return err
    }

    desired.ResourceVersion = current.ResourceVersion
    return c.Update(ctx, desired)
}

func deleteGateway(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases) error {
    gatewayName := fmt.Sprintf("%s-gateway", wandb.Name)
    gw := &gatewayv1.Gateway{}
    if err := c.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: wandb.Namespace}, gw); err != nil {
        if apiErrors.IsNotFound(err) {
            return nil
        }
        return err
    }
    return c.Delete(ctx, gw)
}

func validateExternalGatewayExists(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases, ref *apiv2.GatewayReference) error {
    ns := ref.Namespace
    if ns == "" {
        ns = wandb.Namespace
    }
    gw := &gatewayv1.Gateway{}
    return c.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ns}, gw)
}
```

Helper functions `buildListenersFromConfig()` and `buildDefaultListeners()` translate CR config into `gatewayv1.Listener` structs:
- Map `GatewayListener.Protocol` → `gatewayv1.ProtocolType`
- Map `GatewayListener.TLS` → `gatewayv1.GatewayTLSConfig` with certificate refs
- For the default listener: use hostname from `spec.wandb.hostname`, port 443, protocol HTTPS, TLS terminate mode, and secret from `spec.networking.tls.secretName`

### 5b: Add Gateway cleanup to finalization

In the `Reconcile()` function's deletion/finalization block (lines 85-116), add Gateway cleanup before the finalizer is removed:

```go
if wandb.Spec.Networking.Mode == apiv2.NetworkingModeGatewayAPI &&
    wandb.Spec.Networking.GatewayAPI != nil &&
    wandb.Spec.Networking.GatewayAPI.Gateway.Managed {
    if err = deleteGateway(ctx, client, wandb); err != nil {
        return ctrl.Result{}, err
    }
}
```

### 5c: Build consolidated Ingress in reconcileApplications

The Ingress is built **after** the application loop in `reconcileApplications()`, since it needs to aggregate path rules from all applications. This follows the same pattern as the existing hostname resolution block (lines 408-429) which also runs after all apps are processed.

```go
func reconcileApplications(...) (ctrl.Result, error) {
    // ... existing application loop (lines 315-406) ...

    // After the application loop, build consolidated networking resources
    if wandb.Spec.Networking.Mode == apiv2.NetworkingModeIngress {
        if err := reconcileConsolidatedIngress(ctx, client, wandb, manifest); err != nil {
            logger.Error(err, "Failed to reconcile Ingress")
            return ctrl.Result{}, err
        }
    }

    // ... existing hostname resolution block (lines 408-429) ...
}
```

New file `internal/controller/v2/ingress.go`:

```go
func reconcileConsolidatedIngress(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) error {
    ingressName := fmt.Sprintf("%s-ingress", wandb.Name)
    hostname := parseHostname(wandb.Spec.Wandb.Hostname)
    ingressConfig := wandb.Spec.Networking.Ingress

    // Build path rules from all applications that declare routing
    var rules []networkingv1.IngressRule
    var paths []networkingv1.HTTPIngressPath

    for _, app := range manifest.Applications {
        if len(app.Features) > 0 && !manifestFeaturesEnabled(app.Features, manifest.Features) {
            continue
        }
        if app.Ingress == nil && app.Service == nil {
            continue
        }

        appPaths := []string{"/"}
        pathType := networkingv1.PathTypePrefix
        if app.Ingress != nil {
            if len(app.Ingress.Paths) > 0 {
                appPaths = app.Ingress.Paths
            }
            if app.Ingress.PathType == "Exact" {
                pathType = networkingv1.PathTypeExact
            } else if app.Ingress.PathType == "ImplementationSpecific" {
                pathType = networkingv1.PathTypeImplementationSpecific
            }
        }

        serviceName := fmt.Sprintf("%s-%s", wandb.Name, app.Name)
        servicePort := resolveServicePort(app)

        for _, p := range appPaths {
            paths = append(paths, networkingv1.HTTPIngressPath{
                Path:     p,
                PathType: &pathType,
                Backend: networkingv1.IngressBackend{
                    Service: &networkingv1.IngressServiceBackend{
                        Name: serviceName,
                        Port: servicePort,
                    },
                },
            })
        }
    }

    // Build rules for primary hostname
    rules = append(rules, networkingv1.IngressRule{
        Host: hostname,
        IngressRuleValue: networkingv1.IngressRuleValue{
            HTTP: &networkingv1.HTTPIngressRuleValue{Paths: paths},
        },
    })

    // Add rules for additional hostnames (same paths, different host)
    for _, additionalHost := range wandb.Spec.Wandb.AdditionalHostnames {
        rules = append(rules, networkingv1.IngressRule{
            Host: additionalHost,
            IngressRuleValue: networkingv1.IngressRuleValue{
                HTTP: &networkingv1.HTTPIngressRuleValue{Paths: paths},
            },
        })
    }

    // Build the Ingress
    desired := &networkingv1.Ingress{}
    desired.Name = ingressName
    desired.Namespace = wandb.Namespace
    desired.Spec.IngressClassName = ingressConfig.IngressClassName
    desired.Spec.Rules = rules

    // TLS configuration
    if wandb.Spec.Networking.TLS != nil && wandb.Spec.Networking.TLS.SecretName != "" {
        allHosts := []string{hostname}
        allHosts = append(allHosts, wandb.Spec.Wandb.AdditionalHostnames...)
        desired.Spec.TLS = []networkingv1.IngressTLS{{
            Hosts:      allHosts,
            SecretName: wandb.Spec.Networking.TLS.SecretName,
        }}
    }

    // Labels
    desired.Labels = map[string]string{
        "app.kubernetes.io/managed-by": "wandb-operator",
        "app.kubernetes.io/instance":   wandb.Name,
    }

    // Annotations: merge networking-level + cert-manager annotations
    desired.Annotations = map[string]string{}
    for k, v := range wandb.Spec.Networking.Annotations {
        desired.Annotations[k] = v
    }
    if wandb.Spec.Networking.TLS != nil && wandb.Spec.Networking.TLS.CertManager != nil {
        cm := wandb.Spec.Networking.TLS.CertManager
        if cm.ClusterIssuer != "" {
            desired.Annotations["cert-manager.io/cluster-issuer"] = cm.ClusterIssuer
        }
        if cm.Issuer != "" {
            desired.Annotations["cert-manager.io/issuer"] = cm.Issuer
        }
    }

    if err := controllerutil.SetOwnerReference(wandb, desired, c.Scheme()); err != nil {
        return err
    }

    // Create or Update
    current := &networkingv1.Ingress{}
    err := c.Get(ctx, types.NamespacedName{Name: ingressName, Namespace: wandb.Namespace}, current)
    if err != nil {
        if apiErrors.IsNotFound(err) {
            return c.Create(ctx, desired)
        }
        return err
    }

    desired.ResourceVersion = current.ResourceVersion
    return c.Update(ctx, desired)
}

func deleteConsolidatedIngress(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases) error {
    ingressName := fmt.Sprintf("%s-ingress", wandb.Name)
    ingress := &networkingv1.Ingress{}
    if err := c.Get(ctx, types.NamespacedName{Name: ingressName, Namespace: wandb.Namespace}, ingress); err != nil {
        if apiErrors.IsNotFound(err) {
            return nil
        }
        return err
    }
    return c.Delete(ctx, ingress)
}
```

Add Ingress cleanup to the finalization block alongside Gateway:

```go
if wandb.Spec.Networking.Mode == apiv2.NetworkingModeIngress {
    if err = deleteConsolidatedIngress(ctx, client, wandb); err != nil {
        return ctrl.Result{}, err
    }
}
```

### 5d: Populate HTTPRouteTemplate per application

Inside the existing application loop in `reconcileApplications()`, after the `ServiceTemplate` block (~line 380-388), populate the HTTPRoute template for Gateway API mode:

```go
// After ServiceTemplate population:
if wandb.Spec.Networking.Mode == apiv2.NetworkingModeGatewayAPI && app.Ingress != nil {
    application.Spec.HTTPRouteTemplate = buildHTTPRouteTemplate(wandb, app)
} else {
    application.Spec.HTTPRouteTemplate = nil
}
```

```go
func buildHTTPRouteTemplate(wandb *apiv2.WeightsAndBiases, app serverManifest.Application) *apiv2.HTTPRouteTemplateSpec {
    gwConfig := wandb.Spec.Networking.GatewayAPI

    // Determine the parent ref
    var parentRef gatewayv1.ParentReference
    if gwConfig.Gateway.Managed {
        gatewayName := gatewayv1.ObjectName(fmt.Sprintf("%s-gateway", wandb.Name))
        parentRef = gatewayv1.ParentReference{Name: gatewayName}
    } else {
        ns := gatewayv1.Namespace(gwConfig.Gateway.GatewayRef.Namespace)
        parentRef = gatewayv1.ParentReference{
            Name:      gatewayv1.ObjectName(gwConfig.Gateway.GatewayRef.Name),
            Namespace: &ns,
        }
    }
    if gwConfig.ListenerName != nil {
        sectionName := gatewayv1.SectionName(*gwConfig.ListenerName)
        parentRef.SectionName = &sectionName
    }

    // Build hostnames
    hostname := parseHostname(wandb.Spec.Wandb.Hostname)
    hostnames := []gatewayv1.Hostname{gatewayv1.Hostname(hostname)}
    for _, h := range wandb.Spec.Wandb.AdditionalHostnames {
        hostnames = append(hostnames, gatewayv1.Hostname(h))
    }

    // Build rules from manifest paths
    appPaths := []string{"/"}
    if app.Ingress != nil && len(app.Ingress.Paths) > 0 {
        appPaths = app.Ingress.Paths
    }

    serviceName := fmt.Sprintf("%s-%s", wandb.Name, app.Name)
    servicePort := resolveServicePort(app)

    var matches []gatewayv1.HTTPRouteMatch
    for _, p := range appPaths {
        matchType := gatewayv1.PathMatchPathPrefix
        if app.Ingress != nil && app.Ingress.PathType == "Exact" {
            matchType = gatewayv1.PathMatchExact
        }
        matches = append(matches, gatewayv1.HTTPRouteMatch{
            Path: &gatewayv1.HTTPPathMatch{
                Type:  &matchType,
                Value: &p,
            },
        })
    }

    backendRef := gatewayv1.HTTPBackendRef{
        BackendRef: gatewayv1.BackendRef{
            BackendObjectReference: gatewayv1.BackendObjectReference{
                Name: gatewayv1.ObjectName(serviceName),
                Port: &servicePort,
            },
        },
    }

    return &apiv2.HTTPRouteTemplateSpec{
        ParentRefs: []gatewayv1.ParentReference{parentRef},
        Hostnames:  hostnames,
        Rules: []gatewayv1.HTTPRouteRule{{
            Matches:     matches,
            BackendRefs: []gatewayv1.HTTPBackendRef{backendRef},
        }},
    }
}
```

### 5e: Guard NodePort hostname override

Modify the hostname resolution block (lines 408-429): skip the NodePort proxy hostname override when `Networking.Mode != ""`:

```go
if wandb.Spec.Networking.Mode == apiv2.NetworkingModeNone {
    // Existing NodePort proxy hostname logic
    if manifestFeaturesEnabled([]string{"proxy"}, manifest.Features) && hostname.Port() == "" {
        // ... existing NodePort discovery ...
    }
}
```

---

## Step 6: Implement reconcileHTTPRoute() in the Application controller

**File:** `internal/controller/application_controller.go`

Only HTTPRoute is managed per-application. Ingress is not — it's a consolidated resource handled in Step 5c.

Following the `reconcileService()` pattern (lines 714-804):

```go
func (r *ApplicationReconciler) reconcileHTTPRoute(ctx context.Context, app *wandbv2.Application) error {
    if app.Spec.HTTPRouteTemplate == nil {
        return r.deleteHTTPRoute(ctx, app)
    }

    logger := logx.GetSlog(ctx)

    desired := &gatewayv1.HTTPRoute{}
    desired.Name = app.Name
    desired.Namespace = app.Namespace

    desired.Labels = utils.MergeMapsStringString(desired.Labels, app.Spec.MetaTemplate.Labels)
    desired.Annotations = utils.MergeMapsStringString(desired.Annotations, app.Spec.MetaTemplate.Annotations)

    desired.Spec.ParentRefs = app.Spec.HTTPRouteTemplate.ParentRefs
    desired.Spec.Hostnames = app.Spec.HTTPRouteTemplate.Hostnames
    desired.Spec.Rules = app.Spec.HTTPRouteTemplate.Rules

    current := &gatewayv1.HTTPRoute{}
    err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, current)
    if err != nil {
        if !errors.IsNotFound(err) {
            logger.Error("Failed to get HTTPRoute", logx.ErrAttr(err))
            return err
        }
        logger.Info("Creating HTTPRoute", "HTTPRoute", desired.Name)
        return r.Create(ctx, desired)
    }

    desired.ResourceVersion = current.ResourceVersion
    logger.Info("Updating HTTPRoute", "HTTPRoute", desired.Name)
    return r.Update(ctx, desired)
}

func (r *ApplicationReconciler) deleteHTTPRoute(ctx context.Context, app *wandbv2.Application) error {
    logger := logx.GetSlog(ctx)
    route := &gatewayv1.HTTPRoute{}
    if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, route); err != nil {
        if errors.IsNotFound(err) {
            return nil
        }
        return err
    }
    deletePolicy := client.PropagationPolicy(v1.DeletePropagationBackground)
    if err := r.Delete(ctx, route, deletePolicy); err != nil {
        logger.Error("Failed to delete HTTPRoute", logx.ErrAttr(err), "HTTPRoute", app.Name)
        return err
    }
    logger.Info("Successfully deleted HTTPRoute", "HTTPRoute", app.Name)
    return nil
}
```

Wire into `Reconcile()` after `reconcileHPA()` (~line 196):
```go
if err := r.reconcileHTTPRoute(ctx, &app); err != nil {
    logger.Error("Failed to reconcile HTTPRoute", logx.ErrAttr(err))
    return ctrl.Result{}, err
}
```

Wire into finalization block (~line 116-126):
```go
if err := r.deleteHTTPRoute(ctx, &app); err != nil {
    logger.Error("Failed to delete HTTPRoute during finalization", logx.ErrAttr(err))
    return ctrl.Result{}, err
}
```

Update `SetupWithManager()` to add `Owns(&gatewayv1.HTTPRoute{})`.

---

## Step 7: Add RBAC markers

**File:** `internal/controller/application_controller.go` (for HTTPRoute):

```go
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/status,verbs=get
```

**File:** `internal/controller/weightsandbiases_controller.go` (for Ingress and Gateway):

```go
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/status,verbs=get
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get
```

---

## Step 8: Add webhook validation

**File:** `internal/webhook/v2/weightsandbiases_webhook.go`

### Validation (add to `validateSpec`):
- If `Networking.Mode == "Ingress"` and `Networking.GatewayAPI` is set → error (mutual exclusivity)
- If `Networking.Mode == "GatewayAPI"` and `Networking.Ingress` is set → error
- If `Networking.Mode == "GatewayAPI"` and `Networking.GatewayAPI` is nil → error (config required)
- If `Networking.Mode == "GatewayAPI"` and `Gateway.Managed == false` and `Gateway.GatewayRef == nil` → error (must provide a reference)
- If `Networking.Mode == "GatewayAPI"` and `Gateway.Managed == true` and `Gateway.GatewayClassName == nil` → error (class required for managed)
- If `Networking.Mode == "GatewayAPI"` and `Gateway.Managed == true` and `Gateway.GatewayRef != nil` → error (conflicting: both managed and external)
- If `Networking.TLS.CertManager` is set and `Networking.Mode != "Ingress"` → warning (cert-manager annotations only apply to Ingress)

### Defaulting:
- No aggressive defaults — empty `Networking` means NodePort behavior is preserved
- If `Gateway.Managed == true` and no listeners are specified, a default HTTPS listener is generated at reconcile time (not in the webhook)

---

## Step 9: Add Gateway and Ingress status to WeightsAndBiasesStatus

**File:** `api/v2/weightsandbiases_types.go`

Add status tracking for networking resources:

```go
type WeightsAndBiasesStatus struct {
    // ... existing fields ...

    // GatewayStatus reports the state of the managed Gateway (if any).
    // +optional
    GatewayStatus *GatewayStatusSummary `json:"gatewayStatus,omitempty"`

    // IngressStatus reports the state of the consolidated Ingress (if any).
    // +optional
    IngressStatus *IngressStatusSummary `json:"ingressStatus,omitempty"`
}

type GatewayStatusSummary struct {
    // Name of the managed Gateway resource.
    Name string `json:"name,omitempty"`
    // Ready indicates whether the Gateway has been accepted and programmed.
    Ready bool `json:"ready,omitempty"`
    // Addresses lists the addresses assigned to the Gateway by the infrastructure.
    Addresses []string `json:"addresses,omitempty"`
}

type IngressStatusSummary struct {
    // Name of the consolidated Ingress resource.
    Name string `json:"name,omitempty"`
    // LoadBalancer contains the current status of the load-balancer.
    LoadBalancerIngress []corev1.LoadBalancerIngress `json:"loadBalancerIngress,omitempty"`
}
```

`reconcileConsolidatedIngress()` and `reconcileGateway()` update these status fields after each reconcile.

---

## Step 10: Run code generation and update CRDs

After all type changes:
```bash
make generate    # deepcopy, etc.
make manifests   # CRD YAMLs, RBAC
```

---

## Step 11: Tests

- **Unit tests for `reconcileGateway()`** — test managed create/update, external validation, cleanup on delete
- **Unit tests for `reconcileConsolidatedIngress()`** — test single-app, multi-app path aggregation, TLS config, cert-manager annotations, additional hostnames, cleanup on delete
- **Unit tests for `buildHTTPRouteTemplate()`** — test managed gateway ref, external gateway ref, path mapping, listener name selection
- **Unit tests for `reconcileHTTPRoute()`** — create, update, delete (follow existing reconcileService test patterns)
- **Webhook validation tests** — mutual exclusivity, managed vs external validation, required fields
- Run `make lint` and `make test`

---

## Files to modify (summary)

| File | Change |
|------|--------|
| `go.mod` | Add `sigs.k8s.io/gateway-api` dependency |
| `api/v2/weightsandbiases_types.go` | Add `NetworkingSpec`, `GatewayAPIConfig`, `GatewayConfig`, `GatewayListener`, `GatewayStatusSummary`, `IngressStatusSummary` and related types; add `Networking` field to `WeightsAndBiasesSpec`; add `GatewayStatus`/`IngressStatus` to `WeightsAndBiasesStatus` |
| `api/v2/application_types.go` | Add `HTTPRouteTemplateSpec`, `HTTPRouteStatusSummary`, add `HTTPRouteTemplate` to `ApplicationSpec` |
| `pkg/wandb/manifest/manifest.go` | Add `AppIngressSpec` type, add `Ingress` field to `Application` |
| `internal/controller/v2/gateway.go` | **New file:** `reconcileGateway()`, `deleteGateway()`, `validateExternalGatewayExists()`, listener builder helpers |
| `internal/controller/v2/ingress.go` | **New file:** `reconcileConsolidatedIngress()`, `deleteConsolidatedIngress()`, path aggregation logic |
| `internal/controller/v2/reconcile_v2.go` | Call `reconcileGateway()` from `ReconcileWandbManifest()`, call `reconcileConsolidatedIngress()` from `reconcileApplications()`, add `buildHTTPRouteTemplate()` + populate per-app HTTPRouteTemplate, guard NodePort hostname logic, add cleanup to finalization |
| `internal/controller/application_controller.go` | Add `reconcileHTTPRoute()`, `deleteHTTPRoute()`, RBAC markers for HTTPRoute, wire into `Reconcile()` and finalization, update `SetupWithManager()` |
| `internal/controller/weightsandbiases_controller.go` | Add RBAC markers for Ingress and Gateway resources |
| `internal/webhook/v2/weightsandbiases_webhook.go` | Add `validateNetworkingSpec()`, wire into `validateSpec()` |
| `api/v2/zz_generated.deepcopy.go` | Auto-generated by `make generate` |
| `config/crd/bases/*.yaml` | Auto-generated by `make manifests` |
| `deploy/operator/crds/*.yaml` | Sync updated CRDs |

---

## Example CR usage

### Ingress mode
```yaml
apiVersion: apps.wandb.com/v2
kind: WeightsAndBiases
metadata:
  name: wandb
spec:
  size: small
  wandb:
    hostname: https://wandb.example.com
    version: "0.76.1"
  networking:
    mode: Ingress
    ingress:
      ingressClassName: nginx
    tls:
      secretName: wandb-tls
      certManager:
        clusterIssuer: letsencrypt-prod
    annotations:
      nginx.ingress.kubernetes.io/proxy-body-size: "0"
```

This creates a single Ingress `wandb-ingress` with path rules for each application:
```
wandb.example.com/         → wandb-app:8080
wandb.example.com/api/     → wandb-gorilla:8080
wandb.example.com/console  → wandb-console:8082
...
```

### Gateway API — managed Gateway
```yaml
apiVersion: apps.wandb.com/v2
kind: WeightsAndBiases
metadata:
  name: wandb
spec:
  size: small
  wandb:
    hostname: https://wandb.example.com
    version: "0.76.1"
  networking:
    mode: GatewayAPI
    gatewayAPI:
      gateway:
        managed: true
        gatewayClassName: istio
        listeners:
          - name: https
            port: 443
            protocol: HTTPS
        annotations:
          networking.istio.io/service-type: ClusterIP
      listenerName: https
    tls:
      secretName: wandb-tls
```

This creates one managed Gateway `wandb-gateway` and individual HTTPRoutes per application:
- `wandb-app` HTTPRoute: `wandb.example.com/` → `wandb-app:8080`
- `wandb-gorilla` HTTPRoute: `wandb.example.com/api/` → `wandb-gorilla:8080`
- etc.

### Gateway API — external Gateway
```yaml
apiVersion: apps.wandb.com/v2
kind: WeightsAndBiases
metadata:
  name: wandb
spec:
  size: small
  wandb:
    hostname: https://wandb.example.com
    version: "0.76.1"
  networking:
    mode: GatewayAPI
    gatewayAPI:
      gateway:
        gatewayRef:
          name: shared-gateway
          namespace: gateway-system
      listenerName: https
    tls:
      secretName: wandb-tls
```

### Gateway API — managed Gateway with defaults
```yaml
# Minimal config: operator creates a Gateway with a default HTTPS listener
# derived from spec.wandb.hostname and spec.networking.tls.secretName
apiVersion: apps.wandb.com/v2
kind: WeightsAndBiases
metadata:
  name: wandb
spec:
  size: small
  wandb:
    hostname: https://wandb.example.com
    version: "0.76.1"
  networking:
    mode: GatewayAPI
    gatewayAPI:
      gateway:
        managed: true
        gatewayClassName: istio
    tls:
      secretName: wandb-tls
```

---

## Backward compatibility

- An empty or unset `networking` field preserves current behavior (NodePort with optional nginx-proxy)
- The NodePort proxy hostname override in `reconcile_v2.go` is skipped only when `Networking.Mode != ""`
- No changes to existing CRD fields or defaults
- The existing `IngressTemplate` field on `ApplicationSpec` is left in place but remains unused for this feature — it could be used for future per-application Ingress overrides if needed

---

## Verification

1. `make generate` — ensure deepcopy generation succeeds
2. `make manifests` — ensure CRD and RBAC generation succeeds
3. `make lint` — no lint errors
4. `make test` — all tests pass including new ones
5. Manual: apply a CR with `networking.mode: Ingress` and verify a single Ingress with path rules for all apps is created
6. Manual: apply a CR with `networking.mode: GatewayAPI` + `gateway.managed: true` and verify a Gateway + per-app HTTPRoutes are created
7. Manual: apply a CR with `networking.mode: GatewayAPI` + `gateway.gatewayRef` and verify only per-app HTTPRoutes are created (no Gateway)
8. Manual: delete a CR with managed Gateway and verify the Gateway and Ingress are cleaned up
9. Manual: apply a CR with no `networking` field and verify existing NodePort behavior is preserved
