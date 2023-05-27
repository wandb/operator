package v1

import (
	api "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/helm/values"
)

type Instance struct {
	source *api.WeightsAndBiases
	values values.Values
}
