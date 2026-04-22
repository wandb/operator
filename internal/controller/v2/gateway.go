package v2

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	nginxGatewayv1alpha1 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/utils"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

func reconcileGateway(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases) error {
	ctx, logger := logx.WithSlog(ctx, "Gateway")
	logger.Info("Reconciling Gateway")
	gwConfig := wandb.Spec.Networking.GatewayAPI
	if gwConfig == nil {
		return nil
	}

	if !gwConfig.Gateway.Managed {
		if gwConfig.Gateway.GatewayRef == nil {
			return fmt.Errorf("gatewayAPI.gateway.gatewayRef is required when managed=false")
		}
		gw, err := validateExternalGatewayExists(ctx, c, wandb, gwConfig.Gateway.GatewayRef)
		if err != nil {
			return err
		}
		wandb.Status.GatewayStatus = summarizeGatewayStatus(gw)
		return nil
	}

	gatewayName := fmt.Sprintf("%s-gateway", wandb.Name)
	annotations := utils.MergeMapsStringString(make(map[string]string), gwConfig.Gateway.Annotations)

	if wandb.Spec.Networking.TLS != nil && wandb.Spec.Networking.TLS.CertManager != nil {
		cm := wandb.Spec.Networking.TLS.CertManager
		if cm.ClusterIssuer != "" {
			annotations["cert-manager.io/cluster-issuer"] = cm.ClusterIssuer
		}
		if cm.Issuer != "" {
			annotations["cert-manager.io/issuer"] = cm.Issuer
		}
	}

	desired := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gatewayName,
			Namespace: wandb.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/instance":   wandb.Name,
			},
			Annotations: annotations,
		},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: gatewayv1.ObjectName(*gwConfig.Gateway.GatewayClassName),
		},
	}

	if wandb.Spec.Networking.Annotations != nil {
		desired.Spec.Infrastructure.Annotations = map[gatewayv1.AnnotationKey]gatewayv1.AnnotationValue{}
		for k, v := range wandb.Spec.Networking.Annotations {
			desired.Spec.Infrastructure.Annotations[gatewayv1.AnnotationKey(k)] = gatewayv1.AnnotationValue(v)
		}
	}

	if len(gwConfig.Gateway.Listeners) > 0 {
		desired.Spec.Listeners = buildListenersFromConfig(gwConfig.Gateway.Listeners, wandb)
	} else {
		desired.Spec.Listeners = buildDefaultListeners(wandb)
	}

	if err := controllerutil.SetOwnerReference(wandb, desired, c.Scheme()); err != nil {
		return err
	}

	current := &gatewayv1.Gateway{}
	err := c.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: wandb.Namespace}, current)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			if err := c.Create(ctx, desired); err != nil {
				return err
			}
			wandb.Status.GatewayStatus = summarizeGatewayStatus(desired)
		} else {
			return err
		}
	} else {
		desired.ResourceVersion = current.ResourceVersion
		if err := c.Update(ctx, desired); err != nil {
			return err
		}
		wandb.Status.GatewayStatus = summarizeGatewayStatus(current)
	}

	if wandb.Status.GatewayStatus == nil {
		wandb.Status.GatewayStatus = summarizeGatewayStatus(desired)
	}

	maxSize := "0"
	timeoutStr := "2m"
	switch *wandb.Spec.Networking.GatewayAPI.Gateway.GatewayClassName {
	case "nginx":
		csp := &nginxGatewayv1alpha1.ClientSettingsPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      gatewayName,
				Namespace: wandb.Namespace,
			},
		}
		_, err = controllerruntime.CreateOrUpdate(ctx, c, csp, func() error {
			csp.Spec = nginxGatewayv1alpha1.ClientSettingsPolicySpec{
				Body: &nginxGatewayv1alpha1.ClientBody{
					MaxSize: (*nginxGatewayv1alpha1.Size)(&maxSize),
					Timeout: (*nginxGatewayv1alpha1.Duration)(&timeoutStr),
				},
				KeepAlive: nil,
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Name:  gatewayv1alpha2.ObjectName(gatewayName),
					Kind:  "Gateway",
					Group: "gateway.networking.k8s.io",
				},
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func deleteGateway(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases) error {
	gatewayName := fmt.Sprintf("%s-gateway", wandb.Name)
	gw := &gatewayv1.Gateway{}
	if !utils.IsRegistered(c.Scheme(), gw) {
		return nil
	}
	if err := c.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: wandb.Namespace}, gw); err != nil {
		if apiErrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return c.Delete(ctx, gw)
}

func validateExternalGatewayExists(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	ref *apiv2.GatewayReference,
) (*gatewayv1.Gateway, error) {
	ns := ref.Namespace
	if ns == "" {
		ns = wandb.Namespace
	}
	gw := &gatewayv1.Gateway{}
	if err := c.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ns}, gw); err != nil {
		if apiErrors.IsNotFound(err) {
			return nil, fmt.Errorf("external Gateway %s/%s not found", ns, ref.Name)
		}
		return nil, err
	}
	return gw, nil
}

func buildListenersFromConfig(listeners []apiv2.GatewayListener, wandb *apiv2.WeightsAndBiases) []gatewayv1.Listener {
	var result []gatewayv1.Listener
	for _, l := range listeners {
		listener := gatewayv1.Listener{
			Name:          gatewayv1.SectionName(l.Name),
			Port:          gatewayv1.PortNumber(l.Port),
			Protocol:      gatewayv1.ProtocolType(l.Protocol),
			AllowedRoutes: buildAllowedRoutes(wandb),
		}
		if l.Hostname != nil {
			h := gatewayv1.Hostname(*l.Hostname)
			listener.Hostname = &h
		}
		if l.TLS != nil {
			listener.TLS = buildListenerTLS(l.TLS, wandb)
		} else if gatewayv1.ProtocolType(l.Protocol) == gatewayv1.HTTPSProtocolType {
			listener.TLS = buildDefaultTLS(wandb)
		}
		result = append(result, listener)
	}
	return result
}

func buildDefaultListeners(wandb *apiv2.WeightsAndBiases) []gatewayv1.Listener {
	parsedURL, err := url.Parse(wandb.Spec.Wandb.Hostname)
	hostname := gatewayv1.Hostname(parsedURL.Hostname())
	port, _ := strconv.Atoi(parsedURL.Port())
	listenerPort := gatewayv1.PortNumber(port)
	if err != nil {
		return nil
	}
	listener := gatewayv1.Listener{
		Name:          gatewayv1.SectionName("http"),
		Hostname:      &hostname,
		Port:          listenerPort,
		AllowedRoutes: buildAllowedRoutes(wandb),
	}
	if parsedURL.Scheme == "https" {
		listener.Protocol = gatewayv1.HTTPSProtocolType
		listener.TLS = buildDefaultTLS(wandb)
		if parsedURL.Port() == "" {
			listener.Port = gatewayv1.PortNumber(443)
		}
	} else {
		listener.Protocol = gatewayv1.HTTPProtocolType
		if parsedURL.Port() == "" {
			listener.Port = gatewayv1.PortNumber(80)
		}
	}

	return []gatewayv1.Listener{listener}
}

func buildListenerTLS(tlsConfig *apiv2.ListenerTLSConfig, wandb *apiv2.WeightsAndBiases) *gatewayv1.ListenerTLSConfig {
	mode := gatewayv1.TLSModeTerminate
	if tlsConfig.Mode != nil {
		mode = gatewayv1.TLSModeType(*tlsConfig.Mode)
	}

	listenerTLS := &gatewayv1.ListenerTLSConfig{
		Mode: &mode,
	}

	if tlsConfig.CertificateRef != nil {
		ref := gatewayv1.SecretObjectReference{
			Name: gatewayv1.ObjectName(tlsConfig.CertificateRef.Name),
		}
		if tlsConfig.CertificateRef.Namespace != "" {
			ns := gatewayv1.Namespace(tlsConfig.CertificateRef.Namespace)
			ref.Namespace = &ns
		}
		listenerTLS.CertificateRefs = []gatewayv1.SecretObjectReference{ref}
	} else if wandb.Spec.Networking.TLS != nil && wandb.Spec.Networking.TLS.SecretName != "" {
		listenerTLS.CertificateRefs = []gatewayv1.SecretObjectReference{{
			Name: gatewayv1.ObjectName(wandb.Spec.Networking.TLS.SecretName),
		}}
	}

	return listenerTLS
}

func buildDefaultTLS(wandb *apiv2.WeightsAndBiases) *gatewayv1.ListenerTLSConfig {
	mode := gatewayv1.TLSModeTerminate
	listenerTLS := &gatewayv1.ListenerTLSConfig{
		Mode: &mode,
	}
	if wandb.Spec.Networking.TLS != nil && wandb.Spec.Networking.TLS.SecretName != "" {
		listenerTLS.CertificateRefs = []gatewayv1.SecretObjectReference{{
			Name: gatewayv1.ObjectName(wandb.Spec.Networking.TLS.SecretName),
		}}
	}
	return listenerTLS
}

func parseHostname(rawHostname string) string {
	parsed, err := url.Parse(rawHostname)
	if err != nil {
		return rawHostname
	}
	if parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return rawHostname
}

func buildAllowedRoutes(wandb *apiv2.WeightsAndBiases) *gatewayv1.AllowedRoutes {
	from := gatewayv1.NamespacesFromSame
	if requiresCrossNamespaceInfraRoutes(wandb) {
		from = gatewayv1.NamespacesFromAll
	}

	return &gatewayv1.AllowedRoutes{
		Namespaces: &gatewayv1.RouteNamespaces{
			From: &from,
		},
	}
}

func requiresCrossNamespaceInfraRoutes(wandb *apiv2.WeightsAndBiases) bool {
	namespaces := []string{}

	if spec := wandb.Spec.ObjectStore.ManagedObjectStore; spec != nil {
		namespaces = append(namespaces, spec.Namespace)
	}
	if spec := wandb.Spec.ClickHouse.ManagedClickHouse; spec != nil {
		namespaces = append(namespaces, spec.Namespace)
	}
	if spec := wandb.Spec.MySQL.ManagedMysql; spec != nil {
		namespaces = append(namespaces, spec.Namespace)
	}
	if spec := wandb.Spec.Redis.ManagedRedis; spec != nil {
		namespaces = append(namespaces, spec.Namespace)
	}

	for _, ns := range namespaces {
		if ns != "" && ns != wandb.Namespace {
			return true
		}
	}

	return false
}

func summarizeGatewayStatus(gw *gatewayv1.Gateway) *apiv2.GatewayStatusSummary {
	if gw == nil {
		return nil
	}

	summary := &apiv2.GatewayStatusSummary{
		Name:  gw.Name,
		Ready: isGatewayReady(gw.Status.Conditions),
		GatewayRef: &apiv2.GatewayReference{
			Name:      gw.Name,
			Namespace: gw.Namespace,
		},
	}

	for _, address := range gw.Status.Addresses {
		summary.Addresses = append(summary.Addresses, address.Value)
	}

	return summary
}

func isGatewayReady(conditions []metav1.Condition) bool {
	if apimeta.IsStatusConditionTrue(conditions, string(gatewayv1.GatewayConditionProgrammed)) {
		return true
	}
	return apimeta.IsStatusConditionTrue(conditions, string(gatewayv1.GatewayConditionAccepted))
}
