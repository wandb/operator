package common

import "testing"

func TestApplyImageRegistry(t *testing.T) {
	cases := []struct {
		name     string
		image    string
		registry string
		want     string
	}{
		{"empty registry is no-op", "altinity/clickhouse-server:25.8", "", "altinity/clickhouse-server:25.8"},
		{"docker.io implied (no host) is prefixed", "altinity/clickhouse-server:25.8", "reg.corp:5000", "reg.corp:5000/altinity/clickhouse-server:25.8"},
		{"quay host replaced", "quay.io/opstree/redis:v7.0.15", "reg.corp:5000", "reg.corp:5000/opstree/redis:v7.0.15"},
		{"ghcr nested path host replaced", "ghcr.io/cybozu-go/moco/mysql:8.4.8", "reg.corp:5000", "reg.corp:5000/cybozu-go/moco/mysql:8.4.8"},
		{"single-segment image (library) prefixed", "chrislusf/seaweedfs:latest", "reg.corp:5000", "reg.corp:5000/chrislusf/seaweedfs:latest"},
		{"trailing slash on registry trimmed", "quay.io/opstree/redis:v7", "reg.corp:5000/", "reg.corp:5000/opstree/redis:v7"},
		{"localhost host replaced", "localhost:5000/foo/bar:1", "reg.corp:5000", "reg.corp:5000/foo/bar:1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ApplyImageRegistry(c.image, c.registry); got != c.want {
				t.Fatalf("ApplyImageRegistry(%q, %q) = %q, want %q", c.image, c.registry, got, c.want)
			}
		})
	}
}
