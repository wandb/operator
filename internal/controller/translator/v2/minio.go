package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/common"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/defaults"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildMinioConfig will create a new common.MinioConfig with defaultConfig applied if not
// present in actual. It should *never* be saved into the CR!
func BuildMinioConfig(actual apiv2.WBMinioSpec, defaultConfig common.MinioConfig) (common.MinioConfig, error) {
	minioConfig := TranslateMinioSpec(actual)

	minioConfig.StorageSize = utils.CoalesceQuantity(minioConfig.StorageSize, defaultConfig.StorageSize)
	minioConfig.Namespace = utils.Coalesce(minioConfig.Namespace, defaultConfig.Namespace)
	minioConfig.Resources = utils.Resources(minioConfig.Resources, defaultConfig.Resources)

	minioConfig.Enabled = actual.Enabled

	return minioConfig, nil
}

func TranslateMinioSpec(spec apiv2.WBMinioSpec) common.MinioConfig {
	config := common.MinioConfig{
		Enabled:     spec.Enabled,
		Namespace:   spec.Namespace,
		StorageSize: spec.StorageSize,
	}
	if spec.Config != nil {
		config.Resources = spec.Config.Resources
	}

	return config
}

func ExtractMinioStatus(ctx context.Context, results *common.Results) apiv2.WBMinioStatus {
	return TranslateMinioStatus(
		ctx,
		common.ExtractMinioStatus(ctx, results),
	)
}

func TranslateMinioStatus(ctx context.Context, m common.MinioStatus) apiv2.WBMinioStatus {
	var result apiv2.WBMinioStatus
	var details []apiv2.WBStatusDetail

	for _, err := range m.Errors {
		details = append(details, apiv2.WBStatusDetail{
			State:   apiv2.WBStateError,
			Code:    err.Code(),
			Message: err.Reason(),
		})
	}

	for _, detail := range m.Details {
		state := translateMinioStatusCode(detail.Code())
		details = append(details, apiv2.WBStatusDetail{
			State:   state,
			Code:    detail.Code(),
			Message: detail.Message(),
		})
	}

	result.Connection = apiv2.WBMinioConnection{
		MinioHost:      m.Connection.Host,
		MinioPort:      m.Connection.Port,
		MinioAccessKey: m.Connection.AccessKey,
	}

	result.Ready = m.Ready
	result.Details = details
	result.State = computeOverallState(details, m.Ready)
	result.LastReconciled = metav1.Now()

	return result
}

func translateMinioStatusCode(code string) apiv2.WBStateType {
	switch code {
	case string(common.MinioCreatedCode):
		return apiv2.WBStateUpdating
	case string(common.MinioUpdatedCode):
		return apiv2.WBStateUpdating
	case string(common.MinioDeletedCode):
		return apiv2.WBStateDeleting
	case string(common.MinioConnectionCode):
		return apiv2.WBStateReady
	default:
		return apiv2.WBStateUnknown
	}
}

func (i *InfraConfigBuilder) AddMinioConfig(actual apiv2.WBMinioSpec) *InfraConfigBuilder {
	var err error
	var size common.Size
	var defaultConfig common.MinioConfig
	var mergedConfig common.MinioConfig

	size, err = ToModelSize(i.size)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	defaultConfig, err = defaults.BuildMinioDefaults(size, i.ownerNamespace)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}

	mergedConfig, err = BuildMinioConfig(actual, defaultConfig)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedMinio = mergedConfig
	return i
}
