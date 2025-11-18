package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildMySQLConfig will create a new model.MySQLConfig with defaultConfig applied if not
// present in actual. It should *never* be saved into the CR!
func BuildMySQLConfig(actual apiv2.WBMySQLSpec, defaultConfig model.MySQLConfig) (model.MySQLConfig, error) {
	mysqlConfig := TranslateMySQLSpec(actual)

	mysqlConfig.StorageSize = utils.CoalesceQuantity(mysqlConfig.StorageSize, defaultConfig.StorageSize)
	mysqlConfig.Namespace = utils.Coalesce(mysqlConfig.Namespace, defaultConfig.Namespace)
	mysqlConfig.Resources = utils.Resources(mysqlConfig.Resources, defaultConfig.Resources)

	mysqlConfig.Enabled = actual.Enabled

	return mysqlConfig, nil
}

func TranslateMySQLSpec(spec apiv2.WBMySQLSpec) model.MySQLConfig {
	config := model.MySQLConfig{
		Enabled:     spec.Enabled,
		Namespace:   spec.Namespace,
		StorageSize: spec.StorageSize,
	}
	if spec.Config != nil {
		config.Resources = spec.Config.Resources
	}

	return config
}

func ExtractMySQLStatus(ctx context.Context, results *model.Results) apiv2.WBMySQLStatus {
	return TranslateMySQLStatus(
		ctx,
		model.ExtractMySQLStatus(ctx, results),
	)
}

func TranslateMySQLStatus(ctx context.Context, m model.MySQLStatus) apiv2.WBMySQLStatus {
	var result apiv2.WBMySQLStatus
	var details []apiv2.WBStatusDetail

	for _, err := range m.Errors {
		details = append(details, apiv2.WBStatusDetail{
			State:   apiv2.WBStateError,
			Code:    err.Code(),
			Message: err.Reason(),
		})
	}

	for _, detail := range m.Details {
		state := translateMySQLStatusCode(detail.Code())
		details = append(details, apiv2.WBStatusDetail{
			State:   state,
			Code:    detail.Code(),
			Message: detail.Message(),
		})
	}

	result.Connection = apiv2.WBMySQLConnection{
		MySQLHost: m.Connection.Host,
		MySQLPort: m.Connection.Port,
		MySQLUser: m.Connection.User,
	}

	result.Ready = m.Ready
	result.Details = details
	result.State = computeOverallState(details, m.Ready)
	result.LastReconciled = metav1.Now()

	return result
}

func translateMySQLStatusCode(code string) apiv2.WBStateType {
	switch code {
	case string(model.MySQLCreatedCode):
		return apiv2.WBStateUpdating
	case string(model.MySQLUpdatedCode):
		return apiv2.WBStateUpdating
	case string(model.MySQLDeletedCode):
		return apiv2.WBStateDeleting
	case string(model.MySQLConnectionCode):
		return apiv2.WBStateReady
	default:
		return apiv2.WBStateUnknown
	}
}

func (i *InfraConfigBuilder) AddMySQLConfig(actual apiv2.WBMySQLSpec) *InfraConfigBuilder {
	var err error
	var size model.Size
	var defaultConfig model.MySQLConfig
	var mergedConfig model.MySQLConfig

	size, err = ToModelSize(i.size)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	defaultConfig, err = model.BuildMySQLDefaults(size, i.ownerNamespace)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}

	mergedConfig, err = BuildMySQLConfig(actual, defaultConfig)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedMySQL = mergedConfig
	return i
}
