package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/common"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/defaults"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildKafkaConfig will create a new common.KafkaConfig with defaultConfig applied if not
// present in actual. It should *never* be saved into the CR!
func BuildKafkaConfig(actual apiv2.WBKafkaSpec, defaultConfig common.KafkaConfig) (common.KafkaConfig, error) {
	kafkaConfig := TranslateKafkaSpec(actual)

	kafkaConfig.StorageSize = utils.CoalesceQuantity(kafkaConfig.StorageSize, defaultConfig.StorageSize)
	kafkaConfig.Namespace = utils.Coalesce(kafkaConfig.Namespace, defaultConfig.Namespace)
	kafkaConfig.Resources = utils.Resources(kafkaConfig.Resources, defaultConfig.Resources)

	kafkaConfig.Enabled = actual.Enabled

	return kafkaConfig, nil
}

func TranslateKafkaSpec(spec apiv2.WBKafkaSpec) common.KafkaConfig {
	config := common.KafkaConfig{
		Enabled:     spec.Enabled,
		Namespace:   spec.Namespace,
		StorageSize: spec.StorageSize,
	}
	if spec.Config != nil {
		config.Resources = spec.Config.Resources
	}

	return config
}

func ExtractKafkaStatus(ctx context.Context, results *common.Results) apiv2.WBKafkaStatus {
	return TranslateKafkaStatus(
		ctx,
		common.ExtractKafkaStatus(ctx, results),
	)
}

func TranslateKafkaStatus(ctx context.Context, m common.KafkaStatus) apiv2.WBKafkaStatus {
	var result apiv2.WBKafkaStatus
	var details []apiv2.WBStatusDetail

	for _, err := range m.Errors {
		details = append(details, apiv2.WBStatusDetail{
			State:   apiv2.WBStateError,
			Code:    err.Code(),
			Message: err.Reason(),
		})
	}

	for _, detail := range m.Details {
		state := translateKafkaStatusCode(detail.Code())
		details = append(details, apiv2.WBStatusDetail{
			State:   state,
			Code:    detail.Code(),
			Message: detail.Message(),
		})
	}

	result.Connection = apiv2.WBKafkaConnection{
		KafkaHost: m.Connection.Host,
		KafkaPort: m.Connection.Port,
	}

	result.Ready = m.Ready
	result.Details = details
	result.State = computeOverallState(details, m.Ready)
	result.LastReconciled = metav1.Now()

	return result
}

func translateKafkaStatusCode(code string) apiv2.WBStateType {
	switch code {
	case string(common.KafkaCreatedCode):
		return apiv2.WBStateUpdating
	case string(common.KafkaUpdatedCode):
		return apiv2.WBStateUpdating
	case string(common.KafkaDeletedCode):
		return apiv2.WBStateDeleting
	case string(common.KafkaNodePoolCreatedCode):
		return apiv2.WBStateUpdating
	case string(common.KafkaNodePoolUpdatedCode):
		return apiv2.WBStateUpdating
	case string(common.KafkaNodePoolDeletedCode):
		return apiv2.WBStateDeleting
	case string(common.KafkaConnectionCode):
		return apiv2.WBStateReady
	default:
		return apiv2.WBStateUnknown
	}
}

func (i *InfraConfigBuilder) AddKafkaConfig(actual apiv2.WBKafkaSpec) *InfraConfigBuilder {
	var err error
	var size common.Size
	var defaultConfig common.KafkaConfig
	var mergedConfig common.KafkaConfig

	size, err = ToModelSize(i.size)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	defaultConfig, err = defaults.BuildKafkaDefaults(size, i.ownerNamespace)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}

	mergedConfig, err = BuildKafkaConfig(actual, defaultConfig)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedKafka = mergedConfig
	return i
}
