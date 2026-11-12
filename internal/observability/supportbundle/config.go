package supportbundle

import (
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	sb "github.com/wandb/operator/pkg/observability/supportbundle"
	"k8s.io/utils/ptr"
)

const (
	Component               = "support-bundle"
	supportBundleNameSuffix = "-support-bundle"
	collectorCronJobSuffix  = "-support-bundle-collector"

	supportBundleCronJobNameBudget = 52

	// Still setting a valid schedule even though the job is suspended and idle
	supportBundleSchedule = "0 0 * * *"

	supportBundleContainerName = "collector"
)

// Enabled requires an explicit true, or else its false
func Enabled(wandb *apiv2.WeightsAndBiases) bool {
	return ptr.Deref(wandb.Spec.SupportBundle.Enabled, false)
}

func Version(wandb *apiv2.WeightsAndBiases) string {
	return sb.Version(wandb)
}

// Image retrieves the image with a given repo and version
func Image(wandb *apiv2.WeightsAndBiases) string {
	return sb.Image(wandb)
}

// Name creates the shared namespaced resource name
func Name(wandb *apiv2.WeightsAndBiases) string {
	return common.FitDefaultInfraName(wandb.Name, supportBundleNameSuffix, supportBundleCronJobNameBudget)
}

// CronJobName names the collection template, so other support-bundle jobs can sit beside it under their own names.
func CronJobName(wandb *apiv2.WeightsAndBiases) string {
	return common.FitDefaultInfraName(wandb.Name, collectorCronJobSuffix, supportBundleCronJobNameBudget)
}

// ClusterScopedName includes namespace plus installation name
func ClusterScopedName(wandb *apiv2.WeightsAndBiases) string {
	return common.FitDefaultInfraName(wandb.Namespace+"."+wandb.Name, supportBundleNameSuffix, 253)
}

// SelectorLabels identifies every support-bundle object for an installation
func SelectorLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	labels := common.BuildWandbLabels(wandb, Component)
	labels["app.kubernetes.io/part-of"] = "wandb"
	return labels
}

// objectLabels adds a label for everything created by Operator
func objectLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	labels := SelectorLabels(wandb)
	labels["app.kubernetes.io/managed-by"] = "wandb-operator"
	return labels
}
