package common

import "strings"

// ApplyImageRegistry rewrites image so it is pulled from registry instead of its
// original (public) registry host.
// When registry is empty, image is returned unchanged.
//
//	ApplyImageRegistry("quay.io/opstree/redis:v7", "reg.corp:5000")        -> "reg.corp:5000/opstree/redis:v7"
//	ApplyImageRegistry("altinity/clickhouse-server:25.8", "reg.corp:5000") -> "reg.corp:5000/altinity/clickhouse-server:25.8"
//	ApplyImageRegistry("ghcr.io/cybozu-go/moco/mysql:8.4", "reg.corp:5000") -> "reg.corp:5000/cybozu-go/moco/mysql:8.4"
func ApplyImageRegistry(image, registry string) string {
	if registry == "" {
		return image
	}
	registry = strings.TrimRight(registry, "/")
	if i := strings.IndexByte(image, '/'); i > 0 {
		first := image[:i]
		if strings.ContainsAny(first, ".:") || first == "localhost" {
			return registry + "/" + image[i+1:]
		}
	}
	return registry + "/" + image
}
