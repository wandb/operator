package v2

import "strings"

const (
	NginxIngressProxyBodySizeAnnotation    = "nginx.ingress.kubernetes.io/proxy-body-size"
	NginxIngressProxyReadTimeoutAnnotation = "nginx.ingress.kubernetes.io/proxy-read-timeout"
	NginxIngressProxySendTimeoutAnnotation = "nginx.ingress.kubernetes.io/proxy-send-timeout"
)

func DefaultNginxIngressAnnotations() map[string]string {
	return map[string]string{
		NginxIngressProxyBodySizeAnnotation:    "0",
		NginxIngressProxyReadTimeoutAnnotation: "3600",
		NginxIngressProxySendTimeoutAnnotation: "3600",
	}
}

func UsesNginxIngressClass(config *IngressConfig) bool {
	if config == nil || config.IngressClassName == nil || *config.IngressClassName == "" {
		return true
	}
	return strings.Contains(strings.ToLower(*config.IngressClassName), "nginx")
}
