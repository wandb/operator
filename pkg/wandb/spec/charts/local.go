package charts

import (
	"context"
	"encoding/base64"
	"os"

	"github.com/go-playground/validator/v10"
	v1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/helm"
	"github.com/wandb/operator/pkg/wandb/spec"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LocalRelease release is used for reference a helm chart that is stored
// locally. This can be a tar file or a directory.
//
// It is commonly used for internal development for testing of the helm chart,
// or for air-gapped instance where it points to the chart bundle with the
// controller docker image.
type LocalRelease struct {
	Path string `validate:"required" json:"path"`
}

func (c LocalRelease) Validate() error {
	return validator.New().Struct(c)
}

func (r LocalRelease) Chart() (*chart.Chart, error) {
	file, err := os.ReadFile(r.Path)
	if err != nil {
		return nil, err
	}

	decoded, err := base64.StdEncoding.DecodeString(string(file))
	if err != nil {
		return loader.Load(r.Path)
	}

	tmpfile, err := os.CreateTemp("", "decoded-chart.tgz")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write(decoded); err != nil {
		return nil, err
	}
	if err := tmpfile.Close(); err != nil {
		return nil, err
	}

	return loader.Load(tmpfile.Name())
}

func (r *LocalRelease) getActionableChart(wandb *v1.WeightsAndBiases) (*helm.ActionableChart, error) {
	namespace := wandb.GetNamespace()
	releaseName := wandb.GetName()
	return helm.NewActionableChart(releaseName, namespace)
}

func (r LocalRelease) Apply(
	ctx context.Context,
	c client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
	values spec.Values,
) error {
	actionableChart, err := r.getActionableChart(wandb)
	if err != nil {
		return err
	}

	chart, err := r.Chart()
	if err != nil {
		return err
	}

	_, err = actionableChart.Apply(chart, values)
	return err
}

func (r LocalRelease) Prune(
	ctx context.Context,
	c client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
	_ spec.Values,
) error {
	actionableChart, err := r.getActionableChart(wandb)
	if err != nil {
		return err
	}

	_, err = actionableChart.Uninstall()
	return err
}
