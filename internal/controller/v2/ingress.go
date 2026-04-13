package v2

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func reconcileConsolidatedIngress(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) error {
	ingressName := fmt.Sprintf("%s-ingress", wandb.Name)
	hostname := parseHostname(wandb.Spec.Wandb.Hostname)

	var paths []networkingv1.HTTPIngressPath

	for _, app := range sortedManifestApplications(manifest) {
		if len(app.Features) > 0 && !manifestFeaturesEnabled(app.Features, manifest.Features) {
			continue
		}
		if app.Ingress == nil {
			continue
		}

		appPaths := []string{"/"}
		pathType := networkingv1.PathTypePrefix
		if len(app.Ingress.Paths) > 0 {
			appPaths = app.Ingress.Paths
		}
		switch app.Ingress.PathType {
		case "Exact":
			pathType = networkingv1.PathTypeExact
		case "ImplementationSpecific":
			pathType = networkingv1.PathTypeImplementationSpecific
		}

		serviceName := fmt.Sprintf("%s-%s", wandb.Name, app.Name)
		servicePort := resolveIngressServicePort(app)

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

	if len(paths) == 0 {
		return nil
	}

	rules := []networkingv1.IngressRule{{
		Host: hostname,
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{Paths: paths},
		},
	}}

	for _, additionalHost := range wandb.Spec.Wandb.AdditionalHostnames {
		rules = append(rules, networkingv1.IngressRule{
			Host: additionalHost,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{Paths: paths},
			},
		})
	}

	annotations := map[string]string{}
	for k, v := range wandb.Spec.Networking.Annotations {
		annotations[k] = v
	}
	if wandb.Spec.Networking.TLS != nil && wandb.Spec.Networking.TLS.CertManager != nil {
		cm := wandb.Spec.Networking.TLS.CertManager
		if cm.ClusterIssuer != "" {
			annotations["cert-manager.io/cluster-issuer"] = cm.ClusterIssuer
		}
		if cm.Issuer != "" {
			annotations["cert-manager.io/issuer"] = cm.Issuer
		}
	}

	desired := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName,
			Namespace: wandb.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/instance":   wandb.Name,
			},
			Annotations: annotations,
		},
		Spec: networkingv1.IngressSpec{
			Rules: rules,
		},
	}

	if wandb.Spec.Networking.Ingress != nil {
		desired.Spec.IngressClassName = wandb.Spec.Networking.Ingress.IngressClassName
	}

	if wandb.Spec.Networking.TLS != nil && wandb.Spec.Networking.TLS.SecretName != "" {
		allHosts := []string{hostname}
		allHosts = append(allHosts, wandb.Spec.Wandb.AdditionalHostnames...)
		desired.Spec.TLS = []networkingv1.IngressTLS{{
			Hosts:      allHosts,
			SecretName: wandb.Spec.Networking.TLS.SecretName,
		}}
	}

	if err := controllerutil.SetOwnerReference(wandb, desired, c.Scheme()); err != nil {
		return err
	}

	current := &networkingv1.Ingress{}
	err := c.Get(ctx, types.NamespacedName{Name: ingressName, Namespace: wandb.Namespace}, current)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			if err := c.Create(ctx, desired); err != nil {
				return err
			}
			wandb.Status.IngressStatus = summarizeIngressStatus(desired)
			return nil
		}
		return err
	}

	desired.ResourceVersion = current.ResourceVersion
	if err := c.Update(ctx, desired); err != nil {
		return err
	}
	wandb.Status.IngressStatus = summarizeIngressStatus(current)
	return nil
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

func resolveIngressServicePort(app serverManifest.Application) networkingv1.ServiceBackendPort {
	if app.Ingress != nil && app.Ingress.ServicePort != "" {
		port := intstr.Parse(app.Ingress.ServicePort)
		if port.Type == intstr.Int {
			return networkingv1.ServiceBackendPort{Number: port.IntVal}
		}
		return networkingv1.ServiceBackendPort{Name: port.StrVal}
	}
	if app.Service != nil && len(app.Service.Ports) > 0 {
		return networkingv1.ServiceBackendPort{Number: app.Service.Ports[0].Port}
	}
	return networkingv1.ServiceBackendPort{Number: 8080}
}

func summarizeIngressStatus(ingress *networkingv1.Ingress) *apiv2.IngressStatusSummary {
	if ingress == nil {
		return nil
	}

	summary := &apiv2.IngressStatusSummary{
		Name: ingress.Name,
	}
	for _, lb := range ingress.Status.LoadBalancer.Ingress {
		loadBalancerIngress := corev1.LoadBalancerIngress{
			IP:       lb.IP,
			Hostname: lb.Hostname,
		}
		for _, port := range lb.Ports {
			loadBalancerIngress.Ports = append(loadBalancerIngress.Ports, corev1.PortStatus{
				Port:     port.Port,
				Protocol: port.Protocol,
				Error:    port.Error,
			})
		}
		summary.LoadBalancerIngress = append(summary.LoadBalancerIngress, loadBalancerIngress)
	}

	return summary
}
