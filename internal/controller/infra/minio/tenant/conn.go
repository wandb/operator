package tenant

import (
	"fmt"
	"net/url"
)

const (
	MinioUrlScheme = "minio"
	MinioPort      = "443"
)

type minioConnInfo struct {
	RootUser     string
	RootPassword string
	Host         string
	Port         string
}

func buildMinioConnInfo(
	rootUser, rootPassword string, nsNameBldr *NsNameBuilder,
) *minioConnInfo {
	namespace := nsNameBldr.Namespace()
	serviceName := nsNameBldr.ServiceName()
	return &minioConnInfo{
		RootUser:     rootUser,
		RootPassword: rootPassword,
		Host:         fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace),
		Port:         MinioPort,
	}
}

func (m *minioConnInfo) toUrl() *url.URL {
	return &url.URL{
		Scheme: MinioUrlScheme,
		Host:   fmt.Sprintf("%s:%s", m.Host, m.Port),
		User:   url.UserPassword(m.RootUser, m.RootPassword),
	}
}
