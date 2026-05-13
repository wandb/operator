package v2

import (
	"strconv"

	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
)

const (
	nginxProxyApplicationName = "nginx-proxy"
	bucketProxyPath           = "/bucket"
)

func objectStoreProxiedThroughNginx(manifest serverManifest.Manifest) bool {
	if !manifestFeaturesEnabled([]string{"proxy"}, manifest.Features) {
		return false
	}
	app, ok := manifest.Applications[nginxProxyApplicationName]
	return ok && app.Service != nil
}

func applyProxyIngress(app serverManifest.Application, manifest serverManifest.Manifest) serverManifest.Application {
	if app.Name != nginxProxyApplicationName || !objectStoreProxiedThroughNginx(manifest) {
		return app
	}

	ingress := copyAppIngress(app.Ingress)
	if ingress == nil {
		ingress = &serverManifest.AppIngressSpec{}
	}
	if !containsString(ingress.Paths, bucketProxyPath) {
		ingress.Paths = append(ingress.Paths, bucketProxyPath)
	}
	if ingress.PathType == "" {
		ingress.PathType = "Prefix"
	}
	if ingress.ServicePort == "" {
		ingress.ServicePort = firstServicePort(app.Service)
	}
	app.Ingress = ingress
	return app
}

func copyAppIngress(ingress *serverManifest.AppIngressSpec) *serverManifest.AppIngressSpec {
	if ingress == nil {
		return nil
	}
	return &serverManifest.AppIngressSpec{
		Paths:       append([]string(nil), ingress.Paths...),
		ServicePort: ingress.ServicePort,
		PathType:    ingress.PathType,
	}
}

func firstServicePort(service *serverManifest.ServiceSpec) string {
	if service == nil || len(service.Ports) == 0 {
		return ""
	}
	port := service.Ports[0].Port
	if port == 0 {
		return ""
	}
	return strconv.FormatInt(int64(port), 10)
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
