package reconciler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	gkeGatewayApiNetworkingv1 "github.com/GoogleCloudPlatform/gke-gateway-api/apis/networking/v1"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/utils"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	v1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/pkg/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// infraRouteEntry holds the resolved routing info for a single infra component instance.
type infraRouteEntry struct {
	// name is the HTTPRoute resource name
	name            string
	namespace       string
	serviceName     string
	servicePort     gatewayv1.PortNumber
	healthCheckPath string
	healthCheckPort int32
	ingress         *serverManifest.AppIngressSpec
}

const infraHTTPRouteComponent = "infra-route"

// resolveInfraRoutes returns one entry per infra instance that has an Ingress spec and
// whose component is enabled in the CR.
func resolveInfraRoutes(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) ([]infraRouteEntry, error) {
	var entries []infraRouteEntry

	if objectStoreSpec := wandb.Spec.ObjectStore.ManagedObjectStore; objectStoreSpec != nil {
		for _, instanceName := range sortedInfraConfigNames(manifest.Bucket) {
			cfg := manifest.Bucket[instanceName]
			if cfg.Ingress == nil {
				continue
			}
			svcName := "minio"
			port, err := resolveInfraServicePort(
				ctx,
				c,
				types.NamespacedName{Name: svcName, Namespace: wandb.Spec.ObjectStore.ManagedObjectStore.Namespace},
				cfg.Ingress,
				80,
			)
			if err != nil {
				return nil, fmt.Errorf("bucket instance %q: %w", instanceName, err)
			}
			entries = append(entries, infraRouteEntry{
				name:            fmt.Sprintf("%s-bucket-%s", wandb.Name, instanceName),
				namespace:       objectStoreSpec.Namespace,
				serviceName:     svcName,
				servicePort:     port,
				ingress:         cfg.Ingress,
				healthCheckPath: "/ready",
				healthCheckPort: 4444,
			})
		}
	}

	if chSpec := wandb.Spec.ClickHouse.ManagedClickHouse; chSpec != nil {
		for _, instanceName := range sortedInfraConfigNames(manifest.Clickhouse) {
			cfg := manifest.Clickhouse[instanceName]
			if cfg.Ingress == nil {
				continue
			}
			clusterName := clickhouseClusterName(chSpec.Name)
			svcName := fmt.Sprintf("clickhouse-%s", clusterName)
			port, err := resolveInfraServicePort(
				ctx,
				c,
				types.NamespacedName{Name: svcName, Namespace: wandb.Spec.ClickHouse.ManagedClickHouse.Namespace},
				cfg.Ingress,
				8123,
			)
			if err != nil {
				return nil, fmt.Errorf("clickhouse instance %q: %w", instanceName, err)
			}
			entries = append(entries, infraRouteEntry{
				name:            fmt.Sprintf("%s-clickhouse-%s", wandb.Name, instanceName),
				namespace:       chSpec.Namespace,
				serviceName:     svcName,
				servicePort:     port,
				ingress:         cfg.Ingress,
				healthCheckPath: "/ready",
				healthCheckPort: port,
			})
		}
	}

	return entries, nil
}

func resolveInfraServicePort(ctx context.Context, c ctrlClient.Client, serviceRef types.NamespacedName, ingress *serverManifest.AppIngressSpec, defaultPort int32) (gatewayv1.PortNumber, error) {
	if ingress != nil && ingress.ServicePort != "" {
		parsed := intstr.Parse(ingress.ServicePort)
		if parsed.Type != intstr.Int {
			service := &v1.Service{}
			err := c.Get(ctx, serviceRef, service)
			if err != nil {
				return 0, err
			}
			for _, port := range service.Spec.Ports {
				if port.Name == parsed.StrVal {
					return port.Port, nil
				}
			}
			return 0, errors.New(fmt.Sprintf("Port %s, not found in service %s", parsed.StrVal, serviceRef.Name))
		}
		return parsed.IntVal, nil
	}
	return defaultPort, nil
}

// clickhouseClusterName mirrors the logic in the altinity NsNameBuilder.
func clickhouseClusterName(specName string) string {
	name := specName
	if len(name) > 15 {
		name = name[:15]
	}
	name = strings.TrimRight(name, "-")
	return name
}

func reconcileInfraHTTPRoutes(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	manifest serverManifest.Manifest,
) error {
	logger := logx.GetSlog(ctx)
	if wandb.Spec.Networking.Mode != apiv2.NetworkingModeGatewayAPI {
		return nil
	}
	if wandb.Status.GatewayStatus == nil || wandb.Status.GatewayStatus.GatewayRef == nil {
		return nil
	}

	gwConfig := wandb.Spec.Networking.GatewayAPI
	ref := wandb.Status.GatewayStatus.GatewayRef

	hostname := parseHostname(wandb.Spec.Wandb.Hostname)
	hostnames := []gatewayv1.Hostname{gatewayv1.Hostname(hostname)}
	for _, h := range wandb.Spec.Wandb.AdditionalHostnames {
		hostnames = append(hostnames, gatewayv1.Hostname(h))
	}

	entries, err := resolveInfraRoutes(ctx, c, wandb, manifest)
	if err != nil {
		return fmt.Errorf("failed to resolve infra routes: %w", err)
	}

	desiredNames := make(map[string]bool, len(entries))
	for _, entry := range entries {
		desiredNames[infraHTTPRouteKey(entry.namespace, entry.name).String()] = true

		parentRef := buildInfraGatewayParentRef(ref, gwConfig, entry.namespace)
		route := buildInfraHTTPRoute(wandb, parentRef, hostnames, entry)

		httpRoute := &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      entry.name,
				Namespace: entry.namespace,
			},
		}
		op, err := controllerruntime.CreateOrUpdate(ctx, c, httpRoute, func() error {
			httpRoute.Labels = utils.MergeMapsStringString(httpRoute.Labels, route.Labels)
			httpRoute.Annotations = utils.MergeMapsStringString(httpRoute.Annotations, route.Annotations)
			httpRoute.Spec.ParentRefs = route.Spec.ParentRefs
			httpRoute.Spec.Hostnames = route.Spec.Hostnames
			httpRoute.Spec.Rules = route.Spec.Rules
			if err := setInfraHTTPRouteOwnership(wandb, route, c.Scheme()); err != nil {
				return fmt.Errorf("failed to set ownership on infra HTTPRoute %s: %w", entry.name, err)
			}
			return nil
		})
		if err != nil {
			return err
		}
		logger.Info(fmt.Sprintf("Successfully %s HTTPRoute", op), "HTTPRoute", httpRoute.Name)

		if len(httpRoute.Status.Parents) > 0 && httpRoute.Status.Parents[0].ControllerName == "networking.gke.io/gateway" {
			healthCheckPolicy := &gkeGatewayApiNetworkingv1.HealthCheckPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      entry.name,
					Namespace: entry.namespace,
				},
			}
			op, err = controllerruntime.CreateOrUpdate(ctx, c, healthCheckPolicy, func() error {
				healthCheckPolicy.Labels = utils.MergeMapsStringString(healthCheckPolicy.Labels, infraHealthCheckPolicyLabels(wandb))
				healthCheckPolicy.Spec.Default = &gkeGatewayApiNetworkingv1.HealthCheckPolicyConfig{
					CheckIntervalSec:   ptr.Int64(5),
					TimeoutSec:         ptr.Int64(5),
					UnhealthyThreshold: ptr.Int64(3),
					HealthyThreshold:   ptr.Int64(1),
					Config: &gkeGatewayApiNetworkingv1.HealthCheck{
						Type: gkeGatewayApiNetworkingv1.HTTP,
						HTTP: &gkeGatewayApiNetworkingv1.HTTPHealthCheck{
							CommonHealthCheck: gkeGatewayApiNetworkingv1.CommonHealthCheck{
								Port: ptr.Int64(int64(entry.healthCheckPort)),
							},
							CommonHTTPHealthCheck: gkeGatewayApiNetworkingv1.CommonHTTPHealthCheck{
								RequestPath: ptr.String(entry.healthCheckPath),
							},
						},
					},
				}
				healthCheckPolicy.Spec.TargetRef = gatewayv1alpha2.NamespacedPolicyTargetReference{
					Kind: "Service",
					Name: gatewayv1alpha2.ObjectName(entry.serviceName),
				}
				if err != nil {
					return err
				}
				if err := controllerutil.SetControllerReference(wandb, healthCheckPolicy, c.Scheme()); err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				logger.Error("Failed to create or update health check policy", "HealthCheckPolicy", healthCheckPolicy.Name, logx.ErrAttr(err))
				return err
			}
			logger.Info(fmt.Sprintf("Successfully %s HealthCheckPolicy", op), "HealthCheckPolicy", healthCheckPolicy.Name)
		}
	}

	if err := deleteStaleInfraHTTPRoutes(ctx, c, wandb, desiredNames); err != nil {
		return err
	}

	if err := deleteStaleInfraHealthCheckPolicies(ctx, c, wandb, desiredNames); err != nil {
		return err
	}

	return nil
}

func buildInfraHTTPRoute(
	wandb *apiv2.WeightsAndBiases,
	parentRef gatewayv1.ParentReference,
	hostnames []gatewayv1.Hostname,
	entry infraRouteEntry,
) *gatewayv1.HTTPRoute {
	paths := []string{"/"}
	if entry.ingress != nil && len(entry.ingress.Paths) > 0 {
		paths = entry.ingress.Paths
	}

	var matches []gatewayv1.HTTPRouteMatch
	for _, p := range paths {
		p := p
		matchType := gatewayv1.PathMatchPathPrefix
		if entry.ingress != nil && entry.ingress.PathType == "Exact" {
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
				Name: gatewayv1.ObjectName(entry.serviceName),
				Port: &entry.servicePort,
			},
		},
	}

	hostnameOverride := fmt.Sprintf("%s.%s.svc.cluster.local", entry.serviceName, entry.namespace)

	return &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      entry.name,
			Namespace: entry.namespace,
			Labels:    infraHTTPRouteLabels(wandb),
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{parentRef},
			},
			Hostnames: hostnames,
			Rules: []gatewayv1.HTTPRouteRule{{
				Matches:     matches,
				BackendRefs: []gatewayv1.HTTPBackendRef{backendRef},
				Filters: []gatewayv1.HTTPRouteFilter{
					{
						Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
						RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
							Remove: []string{"X-Forwarded-Host", "X-Forwarded-Port"},
						},
					}, {
						Type: gatewayv1.HTTPRouteFilterURLRewrite,
						URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
							Hostname: (*gatewayv1.PreciseHostname)(&hostnameOverride),
						},
					},
				},
			}},
		},
	}
}

func buildInfraGatewayParentRef(
	ref *apiv2.GatewayReference,
	gwConfig *apiv2.GatewayAPIConfig,
	routeNamespace string,
) gatewayv1.ParentReference {
	parentRef := gatewayv1.ParentReference{
		Name: gatewayv1.ObjectName(ref.Name),
	}
	if ref.Namespace != "" && ref.Namespace != routeNamespace {
		ns := gatewayv1.Namespace(ref.Namespace)
		parentRef.Namespace = &ns
	}
	if gwConfig != nil && gwConfig.ListenerName != nil {
		sectionName := gatewayv1.SectionName(*gwConfig.ListenerName)
		parentRef.SectionName = &sectionName
	}
	return parentRef
}

func deleteInfraHTTPRoutes(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	return deleteStaleInfraHTTPRoutes(ctx, c, wandb, map[string]bool{})
}

// isInfraHTTPRouteName returns true for HTTPRoute names that follow the infra route naming convention.
func isInfraHTTPRouteName(routeName, wandbName string) bool {
	infraTypes := []string{"bucket", "clickhouse", "mysql", "redis"}
	for _, t := range infraTypes {
		prefix := fmt.Sprintf("%s-%s-", wandbName, t)
		if strings.HasPrefix(routeName, prefix) {
			return true
		}
	}
	return false
}

func deleteStaleInfraHTTPRoutes(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	desiredRoutes map[string]bool,
) error {
	routeList := &gatewayv1.HTTPRouteList{}
	if !utils.IsRegistered(c.Scheme(), routeList) {
		return nil
	}
	if err := c.List(ctx, routeList, ctrlClient.MatchingLabels(infraHTTPRouteLabels(wandb))); err != nil {
		return fmt.Errorf("failed to list managed infra HTTPRoutes: %w", err)
	}
	for i := range routeList.Items {
		route := &routeList.Items[i]
		key := infraHTTPRouteKey(route.Namespace, route.Name).String()
		if desiredRoutes[key] {
			continue
		}
		if err := c.Delete(ctx, route); err != nil && !apiErrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete stale infra HTTPRoute %s/%s: %w", route.Namespace, route.Name, err)
		}
	}

	legacyRouteList := &gatewayv1.HTTPRouteList{}
	if err := c.List(ctx, legacyRouteList, ctrlClient.InNamespace(wandb.Namespace)); err != nil {
		return fmt.Errorf("failed to list legacy infra HTTPRoutes: %w", err)
	}
	for i := range legacyRouteList.Items {
		route := &legacyRouteList.Items[i]
		if !isOwnedBy(route, wandb) || !isInfraHTTPRouteName(route.Name, wandb.Name) {
			continue
		}
		key := infraHTTPRouteKey(route.Namespace, route.Name).String()
		if desiredRoutes[key] {
			continue
		}
		if err := c.Delete(ctx, route); err != nil && !apiErrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete legacy infra HTTPRoute %s/%s: %w", route.Namespace, route.Name, err)
		}
	}

	return nil
}

func deleteStaleInfraHealthCheckPolicies(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	desiredPolicies map[string]bool,
) error {
	policyList := &gkeGatewayApiNetworkingv1.HealthCheckPolicyList{}
	if !utils.IsRegistered(c.Scheme(), policyList) {
		return nil
	}
	if err := c.List(ctx, policyList, ctrlClient.MatchingLabels(infraHealthCheckPolicyLabels(wandb))); err != nil {
		return fmt.Errorf("failed to list managed infra HealthCheckPolicies: %w", err)
	}
	for i := range policyList.Items {
		policy := &policyList.Items[i]
		key := infraHTTPRouteKey(policy.Namespace, policy.Name).String()
		if desiredPolicies[key] {
			continue
		}
		if err := c.Delete(ctx, policy); err != nil && !apiErrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete stale infra HealthCheckPolicy %s/%s: %w", policy.Namespace, policy.Name, err)
		}
	}

	return nil
}

func infraHealthCheckPolicyLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	labels := infraHTTPRouteLabels(wandb)
	labels[common.WandbComponentLabel] = "infra-healthcheck-policy"
	return labels
}

func infraHTTPRouteLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "wandb-operator",
		"app.kubernetes.io/instance":   wandb.Name,
		common.WandbNameLabel:          wandb.Name,
		common.WandbNamespaceLabel:     wandb.Namespace,
		common.WandbComponentLabel:     infraHTTPRouteComponent,
	}
}

func infraHTTPRouteKey(namespace, name string) types.NamespacedName {
	return types.NamespacedName{Namespace: namespace, Name: name}
}

func setInfraHTTPRouteOwnership(
	wandb *apiv2.WeightsAndBiases,
	route *gatewayv1.HTTPRoute,
	scheme *runtime.Scheme,
) error {
	if route.Namespace != wandb.Namespace {
		route.OwnerReferences = nil
		return nil
	}
	return controllerutil.SetOwnerReference(wandb, route, scheme)
}
