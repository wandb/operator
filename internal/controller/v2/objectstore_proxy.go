package v2

import (
	"fmt"
	"strconv"
	"strings"

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

func applyObjectStoreProxyConfig(app serverManifest.Application, manifest serverManifest.Manifest) serverManifest.Application {
	if app.Name != nginxProxyApplicationName || !objectStoreProxiedThroughNginx(manifest) {
		return app
	}

	app.Env = upsertBucketEnvSource(app.Env, "UPSTREAM_BUCKET", "proxyUpstream")
	app.Env = upsertBucketEnvSource(app.Env, "BUCKET_PROXY_SCHEME", "proxyScheme")
	app.Env = upsertBucketEnvSource(app.Env, "BUCKET_PROXY_HOST_HEADER", "proxyHostHeader")
	app.Env = upsertBucketEnvSource(app.Env, "BUCKET_PROXY_SSL_NAME", "proxySSLName")

	for i := range app.Files {
		switch app.Files[i].FileName {
		case "envvar.conf.template":
			app.Files[i].Inline = patchProxyEnvTemplate(app.Files[i].Inline)
		case "nginx.conf":
			app.Files[i].Inline = patchProxyNginxConfig(app.Files[i].Inline)
		}
	}

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

func upsertBucketEnvSource(envs []serverManifest.EnvVar, name, field string) []serverManifest.EnvVar {
	source := []serverManifest.EnvSource{{
		Name:  "default",
		Type:  "bucket",
		Field: field,
	}}
	for i := range envs {
		if envs[i].Name == name {
			envs[i].Value = ""
			envs[i].ValueFrom = nil
			envs[i].Sources = source
			return envs
		}
	}
	return append(envs, serverManifest.EnvVar{Name: name, Sources: source})
}

func patchProxyEnvTemplate(inline string) string {
	if strings.Contains(inline, "$bucket_proxy_pass") {
		return inline
	}

	inline = strings.Replace(inline, "map $host $bucket{", "map $host $bucket {", 1)
	bucketMap := "map $host $bucket {\n            default $UPSTREAM_BUCKET;\n          }"
	proxyMaps := bucketMap + "\n\n          map $host $bucket_proxy_pass {\n            default $BUCKET_PROXY_SCHEME://bucket;\n          }\n\n          map $host $bucket_host_header {\n            default $BUCKET_PROXY_HOST_HEADER;\n          }\n\n          map $host $bucket_ssl_name {\n            default $BUCKET_PROXY_SSL_NAME;\n          }"
	if strings.Contains(inline, bucketMap) {
		return strings.Replace(inline, bucketMap, proxyMaps, 1)
	}

	return fmt.Sprintf("%s\n%s\n", strings.TrimRight(inline, "\n"), proxyMaps)
}

func patchProxyNginxConfig(inline string) string {
	inline = strings.ReplaceAll(inline, "proxy_set_header Host $bucket;", "proxy_set_header Host $bucket_host_header;")
	inline = strings.ReplaceAll(inline, "proxy_pass https://bucket;", "proxy_pass $bucket_proxy_pass;")
	inline = strings.ReplaceAll(inline, "proxy_pass http://bucket;", "proxy_pass $bucket_proxy_pass;")
	if strings.Contains(inline, "proxy_ssl_name $bucket_ssl_name;") {
		return inline
	}
	return strings.Replace(inline, "proxy_ssl_verify off;", "proxy_ssl_verify off;\n                proxy_ssl_name $bucket_ssl_name;\n                proxy_ssl_server_name on;", 1)
}
