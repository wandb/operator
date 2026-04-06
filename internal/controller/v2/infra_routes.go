package v2

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// infraRouteEntry holds the resolved routing info for a single infra component instance.
type infraRouteEntry struct {
	// name is the HTTPRoute resource name
	name        string
	namespace   string
	serviceName string
	servicePort gatewayv1.PortNumber
	ingress     *serverManifest.AppIngressSpec
}

const infraHTTPRouteComponent = "infra-route"

// resolveInfraRoutes returns one entry per infra instance that has an Ingress spec and
// whose component is enabled in the CR.
func resolveInfraRoutes(wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) ([]infraRouteEntry, error) {
	var entries []infraRouteEntry

	if objectStoreSpec := wandb.Spec.ObjectStore.ManagedObjectStore; objectStoreSpec != nil {
		for instanceName, cfg := range manifest.Bucket {
			if cfg.Ingress == nil {
				continue
			}
			svcName := fmt.Sprintf("%s-hl", objectStoreSpec.Name)
			port, err := resolveInfraServicePort(cfg.Ingress, 9000)
			if err != nil {
				return nil, fmt.Errorf("bucket instance %q: %w", instanceName, err)
			}
			entries = append(entries, infraRouteEntry{
				name:        fmt.Sprintf("%s-bucket-%s", wandb.Name, instanceName),
				namespace:   objectStoreSpec.Namespace,
				serviceName: svcName,
				servicePort: port,
				ingress:     cfg.Ingress,
			})
		}
	}

	if chSpec := wandb.Spec.ClickHouse.ManagedClickHouse; chSpec != nil {
		for instanceName, cfg := range manifest.Clickhouse {
			if cfg.Ingress == nil {
				continue
			}
			clusterName := clickhouseClusterName(chSpec.Name)
			svcName := fmt.Sprintf("clickhouse-%s", clusterName)
			port, err := resolveInfraServicePort(cfg.Ingress, 8123)
			if err != nil {
				return nil, fmt.Errorf("clickhouse instance %q: %w", instanceName, err)
			}
			entries = append(entries, infraRouteEntry{
				name:        fmt.Sprintf("%s-clickhouse-%s", wandb.Name, instanceName),
				namespace:   chSpec.Namespace,
				serviceName: svcName,
				servicePort: port,
				ingress:     cfg.Ingress,
			})
		}
	}

	if mysqlSpec := wandb.Spec.MySQL.ManagedMysql; mysqlSpec != nil {
		for instanceName, cfg := range manifest.Mysql {
			if cfg.Ingress == nil {
				continue
			}
			svcName := mysqlSpec.Name
			port, err := resolveInfraServicePort(cfg.Ingress, 3306)
			if err != nil {
				return nil, fmt.Errorf("mysql instance %q: %w", instanceName, err)
			}
			entries = append(entries, infraRouteEntry{
				name:        fmt.Sprintf("%s-mysql-%s", wandb.Name, instanceName),
				namespace:   mysqlSpec.Namespace,
				serviceName: svcName,
				servicePort: port,
				ingress:     cfg.Ingress,
			})
		}
	}

	if redisSpec := wandb.Spec.Redis.ManagedRedis; redisSpec != nil {
		for instanceName, cfg := range manifest.Redis {
			if cfg.Ingress == nil {
				continue
			}
			svcName := redisSpec.Name
			port, err := resolveInfraServicePort(cfg.Ingress, 6379)
			if err != nil {
				return nil, fmt.Errorf("redis instance %q: %w", instanceName, err)
			}
			entries = append(entries, infraRouteEntry{
				name:        fmt.Sprintf("%s-redis-%s", wandb.Name, instanceName),
				namespace:   redisSpec.Namespace,
				serviceName: svcName,
				servicePort: port,
				ingress:     cfg.Ingress,
			})
		}
	}

	return entries, nil
}

func resolveInfraServicePort(ingress *serverManifest.AppIngressSpec, defaultPort int32) (gatewayv1.PortNumber, error) {
	if ingress != nil && ingress.ServicePort != "" {
		parsed := intstr.Parse(ingress.ServicePort)
		if parsed.Type != intstr.Int {
			return 0, fmt.Errorf("servicePort %q must be a numeric port number", ingress.ServicePort)
		}
		return gatewayv1.PortNumber(parsed.IntVal), nil
	}
	return gatewayv1.PortNumber(defaultPort), nil
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
	if wandb.Spec.Networking.Mode != apiv2.NetworkingModeGatewayAPI {
		return nil
	}
	if wandb.Status.GatewayStatus == nil || wandb.Status.GatewayStatus.GatewayRef == nil {
		return nil
	}

	gwConfig := wandb.Spec.Networking.GatewayAPI
	ref := wandb.Status.GatewayStatus.GatewayRef

	parentRef := gatewayv1.ParentReference{
		Name: gatewayv1.ObjectName(ref.Name),
	}
	if ref.Namespace != "" && ref.Namespace != wandb.Namespace {
		ns := gatewayv1.Namespace(ref.Namespace)
		parentRef.Namespace = &ns
	}
	if gwConfig != nil && gwConfig.ListenerName != nil {
		sectionName := gatewayv1.SectionName(*gwConfig.ListenerName)
		parentRef.SectionName = &sectionName
	}

	hostname := parseHostname(wandb.Spec.Wandb.Hostname)
	hostnames := []gatewayv1.Hostname{gatewayv1.Hostname(hostname)}
	for _, h := range wandb.Spec.Wandb.AdditionalHostnames {
		hostnames = append(hostnames, gatewayv1.Hostname(h))
	}

	entries, err := resolveInfraRoutes(wandb, manifest)
	if err != nil {
		return fmt.Errorf("failed to resolve infra routes: %w", err)
	}

	desiredNames := make(map[string]bool, len(entries))
	for _, entry := range entries {
		desiredNames[infraHTTPRouteKey(entry.namespace, entry.name).String()] = true

		route := buildInfraHTTPRoute(wandb, parentRef, hostnames, entry)
		if err := setInfraHTTPRouteOwnership(wandb, route, c.Scheme()); err != nil {
			return fmt.Errorf("failed to set ownership on infra HTTPRoute %s: %w", entry.name, err)
		}

		current := &gatewayv1.HTTPRoute{}
		err := c.Get(ctx, types.NamespacedName{Name: entry.name, Namespace: entry.namespace}, current)
		if err != nil {
			if apiErrors.IsNotFound(err) {
				if err := c.Create(ctx, route); err != nil {
					return fmt.Errorf("failed to create infra HTTPRoute %s: %w", entry.name, err)
				}
			} else {
				return fmt.Errorf("failed to get infra HTTPRoute %s: %w", entry.name, err)
			}
		} else if !reflect.DeepEqual(current.Spec, route.Spec) ||
			!reflect.DeepEqual(current.Labels, route.Labels) {
			route.ResourceVersion = current.ResourceVersion
			if err := c.Update(ctx, route); err != nil {
				return fmt.Errorf("failed to update infra HTTPRoute %s: %w", entry.name, err)
			}
		}
	}

	if err := deleteStaleInfraHTTPRoutes(ctx, c, wandb, desiredNames); err != nil {
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
			}},
		},
	}
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

func infraHTTPRouteLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "wandb-operator",
		"app.kubernetes.io/instance":   wandb.Name,
		translator.WandbNameLabel:      wandb.Name,
		translator.WandbNamespaceLabel: wandb.Namespace,
		translator.WandbComponentLabel: infraHTTPRouteComponent,
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
