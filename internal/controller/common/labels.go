package common

import (
	apiv2 "github.com/wandb/operator/api/v2"
)

const (
	WandbNameLabel      = "weightsandbiases.apps.wandb.com/name"
	WandbNamespaceLabel = "weightsandbiases.apps.wandb.com/namespace"
	WandbComponentLabel = "weightsandbiases.apps.wandb.com/component"
)

// Standard Kubernetes "recommended" label keys. These are descriptive labels for
// ecosystem tooling (kube-state-metrics, dashboards, kubectl) and NetworkPolicy
// selectors. They are intentionally distinct from the operator/ownership family
// above, which backs immutable spec.selectors and retention selectors.
const (
	StandardNameLabel      = "app.kubernetes.io/name"
	StandardInstanceLabel  = "app.kubernetes.io/instance"
	StandardComponentLabel = "app.kubernetes.io/component"
	StandardPartOfLabel    = "app.kubernetes.io/part-of"
	StandardManagedByLabel = "app.kubernetes.io/managed-by"
	StandardVersionLabel   = "app.kubernetes.io/version"

	// PartOfValue is the value shared by every wandb-managed resource; it is the
	// anchor for namespace-wide NetworkPolicies and release-wide queries.
	PartOfValue = "wandb"
	// ManagedByValue identifies resources reconciled by this operator.
	ManagedByValue = "wandb-operator"
)

// Architectural component roles. See docs/pod-labeling-standards.md.
const (
	RoleServer        = "server"
	RoleWorker        = "worker"
	RoleProxy         = "proxy"
	RoleDatabase      = "database"
	RoleCache         = "cache"
	RoleAnalyticsDB   = "analytics-db"
	RoleQueue         = "queue"
	RoleObjectStorage = "object-storage"
	RoleMigration     = "migration"
)

// appComponentRoles maps known W&B application (manifest) names to their
// architectural role. Unknown apps default to RoleServer.
var appComponentRoles = map[string]string{
	"executor":                          RoleWorker,
	"parquet":                           RoleWorker,
	"weave-trace-worker":                RoleWorker,
	"weave-trace-evaluate-model-worker": RoleWorker,
	"flat-run-fields-updater":           RoleWorker,
	"metric-observer":                   RoleWorker,
	"nginx-proxy":                       RoleProxy,
}

// AppComponentRole returns the architectural role for a W&B application name,
// defaulting to RoleServer for request-serving apps.
func AppComponentRole(appName string) string {
	if role, ok := appComponentRoles[appName]; ok {
		return role
	}
	return RoleServer
}

// StandardLabels returns the descriptive app.kubernetes.io/* label set for a
// resource owned by the given CR. component and version are optional; empty
// values are omitted. instance is always the owning CR (release) name.
func StandardLabels(wandb *apiv2.WeightsAndBiases, name, component, version string) map[string]string {
	l := map[string]string{
		StandardNameLabel:      name,
		StandardInstanceLabel:  wandb.Name,
		StandardPartOfLabel:    PartOfValue,
		StandardManagedByLabel: ManagedByValue,
	}
	if component != "" {
		l[StandardComponentLabel] = component
	}
	if version != "" {
		l[StandardVersionLabel] = version
	}
	return l
}

// HasAllLabelKeys reports whether existing contains every key present in desired,
// regardless of value.
func HasAllLabelKeys(existing, desired map[string]string) bool {
	for k := range desired {
		if _, ok := existing[k]; !ok {
			return false
		}
	}
	return true
}

// BuildWandbLabels returns the standard wandb labels for resources managed
// on behalf of the given WeightsAndBiases CR.
func BuildWandbLabels(wandb *apiv2.WeightsAndBiases, componentName string) map[string]string {
	return map[string]string{
		WandbNameLabel:      wandb.Name,
		WandbNamespaceLabel: wandb.Namespace,
		WandbComponentLabel: componentName,
	}
}
