package helm

import (
	"context"

	"github.com/go-playground/validator/v10"
	v1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/helm"
	"github.com/wandb/operator/pkg/wandb/spec"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LocalRelease struct {
	Path string `validate:"required" json:"path"`
}

func (c *LocalRelease) Validate() error {
	return validator.New().Struct(c)
}

func (r *LocalRelease) Chart() (*chart.Chart, error) {
	return loader.Load(r.Path)
}

func (r *LocalRelease) Apply(
	ctx context.Context,
	c client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
	config spec.Config,
) error {
	chart, err := loader.Load(r.Path)
	if err != nil {
		return err
	}

	actionableChart, err := getActionableChart(chart, wandb)
	if err != nil {
		return err
	}

	_, err = actionableChart.Apply(config)
	return err
}

func (r *LocalRelease) Prune(
	ctx context.Context,
	c client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
	_ spec.Config,
) error {
	chart, err := loader.Load(r.Path)
	if err != nil {
		return err
	}

	actionableChart, err := getActionableChart(chart, wandb)
	if err != nil {
		return err
	}

	_, err = actionableChart.Uninstall()
	return err
}

func getActionableChart(
	chart *chart.Chart,
	wandb *v1.WeightsAndBiases,
) (*helm.ActionableChart, error) {
	namespace := wandb.GetNamespace()
	releaseName := wandb.GetName()
	return helm.NewActionableChart(chart, releaseName, namespace)
}
