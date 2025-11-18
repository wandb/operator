package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildClickHouseConfig will create a new WBClickHouseSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func BuildClickHouseConfig(actual apiv2.WBClickHouseSpec, defaultConfig model.ClickHouseConfig) (model.ClickHouseConfig, error) {
	clickhouseConfig := TranslateClickHouseSpec(actual)

	clickhouseConfig.StorageSize = utils.CoalesceQuantity(clickhouseConfig.StorageSize, defaultConfig.StorageSize)
	clickhouseConfig.Namespace = utils.Coalesce(clickhouseConfig.Namespace, defaultConfig.Namespace)
	clickhouseConfig.Version = utils.Coalesce(clickhouseConfig.Version, defaultConfig.Version)
	clickhouseConfig.Resources = utils.Resources(clickhouseConfig.Resources, defaultConfig.Resources)

	clickhouseConfig.Enabled = actual.Enabled
	clickhouseConfig.Replicas = actual.Replicas

	return clickhouseConfig, nil
}

func TranslateClickHouseSpec(spec apiv2.WBClickHouseSpec) model.ClickHouseConfig {
	config := model.ClickHouseConfig{
		Enabled:     spec.Enabled,
		Namespace:   spec.Namespace,
		StorageSize: spec.StorageSize,
		Replicas:    spec.Replicas,
		Version:     spec.Version,
	}
	if spec.Config != nil {
		config.Resources = spec.Config.Resources
	}

	return config
}

func ExtractClickHouseStatus(ctx context.Context, results *model.Results) apiv2.WBClickHouseStatus {
	return TranslateClickHouseStatus(
		ctx,
		model.ExtractClickHouseStatus(ctx, results),
	)
}

func TranslateClickHouseStatus(ctx context.Context, m model.ClickHouseStatus) apiv2.WBClickHouseStatus {
	var result apiv2.WBClickHouseStatus
	var details []apiv2.WBStatusDetail

	for _, err := range m.Errors {
		details = append(details, apiv2.WBStatusDetail{
			State:   apiv2.WBStateError,
			Code:    err.Code(),
			Message: err.Reason(),
		})
	}

	for _, detail := range m.Details {
		state := translateClickHouseStatusCode(detail.Code())
		details = append(details, apiv2.WBStatusDetail{
			State:   state,
			Code:    detail.Code(),
			Message: detail.Message(),
		})
	}

	result.Connection = apiv2.WBClickHouseConnection{
		ClickHouseHost: m.Connection.Host,
		ClickHousePort: m.Connection.Port,
		ClickHouseUser: m.Connection.User,
	}

	result.Ready = m.Ready
	result.Details = details
	result.State = computeOverallState(details, m.Ready)
	result.LastReconciled = metav1.Now()

	return result
}

func translateClickHouseStatusCode(code string) apiv2.WBStateType {
	switch code {
	case string(model.ClickHouseCreatedCode):
		return apiv2.WBStateUpdating
	case string(model.ClickHouseUpdatedCode):
		return apiv2.WBStateUpdating
	case string(model.ClickHouseDeletedCode):
		return apiv2.WBStateDeleting
	case string(model.ClickHouseConnectionCode):
		return apiv2.WBStateReady
	default:
		return apiv2.WBStateUnknown
	}
}

func (i *InfraConfigBuilder) AddClickHouseConfig(actual apiv2.WBClickHouseSpec) *InfraConfigBuilder {
	var err error
	var size model.Size
	var defaultConfig model.ClickHouseConfig
	var mergedConfig model.ClickHouseConfig

	size, err = ToModelSize(i.size)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	defaultConfig, err = model.BuildClickHouseDefaults(size, i.ownerNamespace)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}

	mergedConfig, err = BuildClickHouseConfig(actual, defaultConfig)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedClickHouse = mergedConfig
	return i
}
