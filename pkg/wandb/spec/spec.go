package spec

import (
	"context"
	"reflect"

	v1 "github.com/wandb/operator/api/v1"
	"helm.sh/helm/v3/pkg/chart"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Metadata map[string]string

type Chart interface {
	Apply(
		context.Context,
		client.Client,
		*v1.WeightsAndBiases,
		*runtime.Scheme,
		Values,
	) error

	Prune(
		context.Context,
		client.Client,
		*v1.WeightsAndBiases,
		*runtime.Scheme,
		Values,
	) error

	Chart() (*chart.Chart, error)
}

type Validatable interface {
	Validate() error
}

// Spec is the top level object that contains all the information needed to
// deploy an instances.
type Spec struct {
	// Used for tracking extra information. This maybe useful in channels for
	// determining information such as the last time the service was pinged and
	// contain information of which version the spec is on.
	Metadata *Metadata `json:"metadata"`

	// Chart contains information about what version of the service to deploy.
	Chart Chart `json:"chart"`

	// Values contains information about how to configure the chart. This is
	// passed into the chart via the Apply command to generate the manifests.
	Values Values `json:"values"`
}

func (s *Spec) Apply(
	ctx context.Context,
	client client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
) error {
	return s.Chart.Apply(ctx, client, wandb, scheme, s.Values)
}

func (s *Spec) Prune(
	ctx context.Context,
	client client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
) error {
	return s.Chart.Prune(ctx, client, wandb, scheme, s.Values)
}

func (s *Spec) SetChart(chart Chart) {
	s.Chart = chart
}

func (s *Spec) SetValues(values Values) {
	s.Values = values
}

func (s *Spec) IsEqual(spec *Spec) bool {
	isMetadataEqual := reflect.DeepEqual(&s.Metadata, &spec.Metadata)
	isReleaseEqual := reflect.DeepEqual(s.Chart, spec.Chart)
	isValuesEqual := reflect.DeepEqual(s.Values, spec.Values)
	return isReleaseEqual && isValuesEqual && isMetadataEqual
}

func (s *Spec) mergeConfig(values Values) (err error) {
	if s.Values == nil {
		s.Values = values
		return nil
	}
	mergedValues, err := s.Values.Merge(values)
	if err != nil {
		return err
	}
	s.Values = mergedValues
	return nil
}

func (s *Spec) Merge(spec *Spec) {
	if spec == nil {
		return
	}

	if spec.Metadata != nil {
		s.Metadata = spec.Metadata
	}

	if spec.Chart != nil {
		s.Chart = spec.Chart
	}
	if spec.Values != nil {
		s.mergeConfig(spec.Values)
	}
}
