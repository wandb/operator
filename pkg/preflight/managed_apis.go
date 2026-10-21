package preflight

import (
	"context"
	"fmt"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// APIRequirement is a kind the operator creates for a managed instance; its dependency operator must serve it.
type APIRequirement struct {
	GVK         schema.GroupVersionKind
	Remediation string
}

// GVKs are literals rather than the vendored types' GroupVersions: importing those packages would drag in the
// operator's mergo replace directive, which doesn't apply to importers like wsm. managed_apis_test.go pins them
// to the vendored GroupVersions so they can't drift from what the reconciler creates.
var (
	mocoMySQLClusterGVK      = schema.GroupVersionKind{Group: "moco.cybozu.com", Version: "v1beta2", Kind: "MySQLCluster"}
	opstreeRedisGVK          = schema.GroupVersionKind{Group: "redis.redis.opstreelabs.in", Version: "v1beta2", Kind: "Redis"}
	opstreeReplicationGVK    = schema.GroupVersionKind{Group: "redis.redis.opstreelabs.in", Version: "v1beta2", Kind: "RedisReplication"}
	opstreeSentinelGVK       = schema.GroupVersionKind{Group: "redis.redis.opstreelabs.in", Version: "v1beta2", Kind: "RedisSentinel"}
	seaweedGVK               = schema.GroupVersionKind{Group: "seaweed.seaweedfs.com", Version: "v1", Kind: "Seaweed"}
	altinityInstallationGVK  = schema.GroupVersionKind{Group: "clickhouse.altinity.com", Version: "v1", Kind: "ClickHouseInstallation"}
	altinityKeeperInstallGVK = schema.GroupVersionKind{Group: "clickhouse-keeper.altinity.com", Version: "v1", Kind: "ClickHouseKeeperInstallation"}
)

// managedAPIsCheck builds a check that a managed instance's dependency operator is installed.
func managedAPIsCheck[T any](name, crField string, requirements func(*T) []APIRequirement) Check {
	return typedCheck(name, crField, TypeCluster, []ResourceReq{ReqReadOnly},
		func(_ context.Context, spec *T, env Env) Result {
			if env.Mapper == nil {
				return Result{Check: name, Outcome: OutcomeUnknown, Message: "no cluster API discovery available"}
			}
			return checkAPIsServed(name, env.Mapper, requirements(spec))
		})
}

func checkAPIsServed(name string, mapper meta.RESTMapper, reqs []APIRequirement) Result {
	var missing []string
	for _, r := range reqs {
		_, err := mapper.RESTMapping(r.GVK.GroupKind(), r.GVK.Version)
		switch {
		case meta.IsNoMatchError(err):
			missing = append(missing, fmt.Sprintf("%s %s (%s)", r.GVK.GroupVersion(), r.GVK.Kind, r.Remediation))
		case err != nil:
			// Failed or forbidden discovery isn't evidence that the API is absent.
			return Result{Check: name, Outcome: OutcomeUnknown, Message: fmt.Sprintf("discover %s: %v",
				r.GVK.GroupVersion(), err)}
		}
	}
	if len(missing) > 0 {
		return Result{Check: name, Outcome: OutcomeFail, Message: "API not served: " + strings.Join(missing, "; ")}
	}
	return Result{Check: name, Outcome: OutcomePass}
}

func managedMysqlAPIs(*apiv2.ManagedMysqlSpec) []APIRequirement {
	return []APIRequirement{{GVK: mocoMySQLClusterGVK, Remediation: "install the MOCO operator"}}
}

// managedRedisAPIs mirrors opstree's split: sentinel mode creates RedisReplication + RedisSentinel instead of Redis.
func managedRedisAPIs(spec *apiv2.ManagedRedisSpec) []APIRequirement {
	const fix = "install the OpsTree redis-operator"
	if spec.Sentinel.Enabled {
		return []APIRequirement{
			{GVK: opstreeReplicationGVK, Remediation: fix},
			{GVK: opstreeSentinelGVK, Remediation: fix},
		}
	}
	return []APIRequirement{{GVK: opstreeRedisGVK, Remediation: fix}}
}

func managedObjectStoreAPIs(*apiv2.ManagedObjectStoreSpec) []APIRequirement {
	return []APIRequirement{{GVK: seaweedGVK, Remediation: "install the SeaweedFS operator"}}
}

func managedClickHouseAPIs(*apiv2.ManagedClickHouseSpec) []APIRequirement {
	const fix = "install the Altinity clickhouse-operator"
	return []APIRequirement{
		{GVK: altinityInstallationGVK, Remediation: fix},
		{GVK: altinityKeeperInstallGVK, Remediation: fix},
	}
}
