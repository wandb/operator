package v2

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/logx"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
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
		return validateExternalGatewayExists(ctx, c, wandb, gwConfig.Gateway.GatewayRef)
	}

	gatewayName := fmt.Sprintf("%s-gateway", wandb.Name)

	desired := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gatewayName,
			Namespace: wandb.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/instance":   wandb.Name,
			},
			Annotations: gwConfig.Gateway.Annotations,
		},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: gatewayv1.ObjectName(*gwConfig.Gateway.GatewayClassName),
		},
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
	if err := c.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ns}, gw); err != nil {
		if apiErrors.IsNotFound(err) {
			return fmt.Errorf("external Gateway %s/%s not found", ns, ref.Name)
		}
		return err
	}
	return nil
}

func buildListenersFromConfig(listeners []apiv2.GatewayListener, wandb *apiv2.WeightsAndBiases) []gatewayv1.Listener {
	var result []gatewayv1.Listener
	for _, l := range listeners {
		listener := gatewayv1.Listener{
			Name:     gatewayv1.SectionName(l.Name),
			Port:     gatewayv1.PortNumber(l.Port),
			Protocol: gatewayv1.ProtocolType(l.Protocol),
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
		Name:     gatewayv1.SectionName("http"),
		Hostname: &hostname,
		Port:     listenerPort,
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
