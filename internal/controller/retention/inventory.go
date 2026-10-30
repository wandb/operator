package retention

import (
	"fmt"
	"slices"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/managed/clickhouse/altinity"
	"github.com/wandb/operator/internal/controller/infra/managed/kafka/bufstream"
	"github.com/wandb/operator/internal/controller/infra/managed/mysql/moco"
	"github.com/wandb/operator/internal/controller/infra/managed/objectstore/seaweedfs"
	"github.com/wandb/operator/internal/controller/infra/managed/redis/opstree"
)

// Inventory includes all direct resources, including inactive Redis roots and
// optional legacy resources. Absence is resolved against the API, not assumed.
func Inventory(w *apiv2.WeightsAndBiases) ([]Resource, error) {
	var resources []Resource
	var firstErr error
	add := func(component, key, ns, name string, spec apiv2.ManagedInfraSpec, api, kind string, stage int) {
		policy, err := Policy(w, spec)
		if err != nil {
			firstErr = err
			return
		}
		if ns == "" {
			ns = w.Namespace
		}
		resources = append(resources, Resource{APIVersion: api, Kind: kind, Namespace: ns, Name: name, Component: component, Instance: key, Policy: policy, Stage: stage, OptionalAPI: kind == "Tenant"})
	}
	external := func(component, key string) {
		name := "wandb-" + component + "-connection"
		if key != "" && key != apiv2.DefaultInstanceName {
			name += "-" + key
		}
		add(component, key, w.Namespace, name, apiv2.ManagedInfraSpec{}, "v1", "Secret", 2)
	}
	for key, cfg := range w.Spec.MySQL {
		if s := cfg.ManagedMysql; s != nil {
			n := s.Name
			if n == "" {
				n = moco.DefaultSpecName(w.Name, key)
			}
			add("mysql", key, s.Namespace, n, s.ManagedInfraSpec, "moco.cybozu.com/v1beta2", "MySQLCluster", 0)
			add("mysql", key, s.Namespace, moco.MyCnfConfigMapName(n), s.ManagedInfraSpec, "v1", "ConfigMap", 2)
			for _, suffix := range []string{"-connection", "-db-password"} {
				add("mysql", key, s.Namespace, n+suffix, s.ManagedInfraSpec, "v1", "Secret", 2)
			}
		} else if cfg.ExternalMysql != nil {
			external("mysql", key)
		}
	}
	for key, cfg := range w.Spec.Redis {
		if s := cfg.ManagedRedis; s != nil {
			n := s.Name
			if n == "" {
				n = opstree.DefaultSpecName(w.Name, key)
			}
			for _, kind := range []string{"Redis", "RedisSentinel", "RedisReplication"} {
				name := n
				if kind == "RedisReplication" {
					name += "-replica"
				}
				add("redis", key, s.Namespace, name, s.ManagedInfraSpec, "redis.redis.opstreelabs.in/v1beta2", kind, 0)
			}
			add("redis", key, s.Namespace, n+"-connection", s.ManagedInfraSpec, "v1", "Secret", 2)
		} else if cfg.ExternalRedis != nil {
			external("redis", key)
		}
	}
	for key, cfg := range w.Spec.ObjectStore {
		if s := cfg.ManagedObjectStore; s != nil {
			n := s.Name
			if n == "" {
				n = seaweedfs.DefaultSpecName(w.Name, key)
			}
			add("objectstore", key, s.Namespace, n, s.ManagedInfraSpec, "seaweed.seaweedfs.com/v1", "Seaweed", 0)
			add("objectstore", key, s.Namespace, seaweedfs.ConfigName(n), s.ManagedInfraSpec, "v1", "Secret", 2)
			add("objectstore", key, s.Namespace, strings.TrimSuffix(n, "-seaweedfs")+"-objectstore-connection", s.ManagedInfraSpec, "v1", "Secret", 2)
			if key == apiv2.DefaultInstanceName {
				add("objectstore", key, w.Namespace, w.Name+"-minio", s.ManagedInfraSpec, "minio.min.io/v2", "Tenant", 0)
				add("objectstore", key, w.Namespace, w.Name+"-objectstore-connection", s.ManagedInfraSpec, "v1", "Secret", 2)
			}
		} else if cfg.ExternalObjectStore != nil {
			external("objectstore", key)
		}
	}
	for key, cfg := range w.Spec.ClickHouse {
		if s := cfg.ManagedClickHouse; s != nil {
			n := s.Name
			if n == "" {
				n = altinity.DefaultSpecName(w.Name, key)
			}
			add("clickhouse", key, s.Namespace, n, s.ManagedInfraSpec, "clickhouse.altinity.com/v1", "ClickHouseInstallation", 0)
			add("clickhouse", key, s.Namespace, strings.TrimSuffix(n, "-chi")+"-chk", s.ManagedInfraSpec, "clickhouse-keeper.altinity.com/v1", "ClickHouseKeeperInstallation", 0)
			add("clickhouse", key, s.Namespace, n+"-connection", s.ManagedInfraSpec, "v1", "Secret", 2)
			if s.ServiceAccount.Create == nil || *s.ServiceAccount.Create {
				name := s.ServiceAccount.ServiceAccountName
				if name == "" {
					name = n
				}
				add("clickhouse", key, s.Namespace, name, s.ManagedInfraSpec, "v1", "ServiceAccount", 2)
			}
		} else if cfg.ExternalClickHouse != nil {
			external("clickhouse", key)
		}
	}
	if s := w.Spec.Kafka.ManagedKafka; s != nil {
		n := s.Name
		if n == "" {
			n = bufstream.DefaultSpecName(w.Name, apiv2.DefaultInstanceName)
		}
		addKafka := func(name, api, kind string, stage int) {
			add("kafka", apiv2.DefaultInstanceName, s.Namespace, name, s.ManagedInfraSpec, api, kind, stage)
		}
		addKafka(n, "apps.wandb.com/v2", "Application", 0)
		addKafka(n+"-etcd", "apps.wandb.com/v2", "Application", 0)
		addKafka(n+"-config", "v1", "ConfigMap", 2)
		addKafka(n+"-storage-credentials", "v1", "Secret", 2)
		addKafka(n+"-connection", "v1", "Secret", 2)
		if s.ServiceAccount.Create == nil || *s.ServiceAccount.Create {
			name := s.ServiceAccount.ServiceAccountName
			if name == "" {
				name = n
			}
			addKafka(name, "v1", "ServiceAccount", 2)
		}
		addKafka(n+"-scc-nonroot-v2", "rbac.authorization.k8s.io/v1", "RoleBinding", 2)
	}
	if firstErr != nil {
		return nil, firstErr
	}
	unique := map[string]Resource{}
	for _, r := range resources {
		if old, ok := unique[r.Key()]; ok && (old.Component != r.Component || old.Instance != r.Instance || old.Policy != r.Policy) {
			return nil, fmt.Errorf("resource %s is shared by %s and %s; configure distinct resource names", r.Key(), InstanceID(old.Component, old.Instance), InstanceID(r.Component, r.Instance))
		}
		unique[r.Key()] = r
	}
	resources = resources[:0]
	for _, r := range unique {
		resources = append(resources, r)
	}
	slices.SortFunc(resources, func(a, b Resource) int { return strings.Compare(a.Key(), b.Key()) })
	return resources, nil
}
