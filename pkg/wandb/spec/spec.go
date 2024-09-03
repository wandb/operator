package spec

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"
	"reflect"

	v1 "github.com/wandb/operator/api/v1"
	"helm.sh/helm/v3/pkg/chart"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Metadata map[string]string

//counterfeiter:generate . Chart
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

// Merge merges the given spec into the current spec. The currently spec will
// take precedence. This means, if the current spec has a value for a key, the
// new spec will not override it. Likewise, charts & metadata cannot be merged,
// so if a chart/metadata value is set, the one passed in will be ignored.
func (s *Spec) Merge(spec *Spec) {
	if spec == nil {
		return
	}

	if s.Metadata == nil {
		s.Metadata = spec.Metadata
	}
	if s.Chart == nil {
		s.Chart = spec.Chart
	}
	if spec.Values != nil {
		s.mergeConfig(spec.Values)
	}
}

func (s *Spec) SensitiveValuesMasked() *Spec {
	if s.Values != nil {
		return &Spec{
			Metadata: s.Metadata,
			Chart:    s.Chart,
			Values:   maskValues(s.Values),
		}
	}
	return s
}

var sensitiveKeys = map[string]bool{
	"secret":    true,
	"accessKey": true,
}

func maskValues(values map[string]interface{}) map[string]interface{} {
	newValues := make(map[string]interface{})
	for key, value := range values {
		switch v := value.(type) {
		case map[string]interface{}:
			newValues[key] = maskValues(v)
		case string:
			if sensitiveKeys[key] {
				newValues[key] = "***"
			} else {
				newValues[key] = value
			}
		default:
			newValues[key] = value
		}
	}
	return newValues
}

func (s *Spec) DiffValues(other *Spec) map[string]interface{} {
	return diffMaps(s.Values, other.Values)
}

func diffMaps(a, b map[string]interface{}) map[string]interface{} {
	diff := make(map[string]interface{})
	for key, aValue := range a {
		if bValue, ok := b[key]; ok {
			if !reflect.DeepEqual(aValue, bValue) {
				switch aValue.(type) {
				case map[string]interface{}:
					if bMap, ok := bValue.(map[string]interface{}); ok {
						nestedDiff := diffMaps(aValue.(map[string]interface{}), bMap)
						if len(nestedDiff) > 0 {
							diff[key] = nestedDiff
						}
					} else {
						diff[key] = aValue
					}
				default:
					diff[key] = aValue
				}
			}
		} else {
			diff[key] = aValue
		}
	}
	for key, bValue := range b {
		if _, ok := a[key]; !ok {
			diff[key] = bValue
		}
	}
	return diff
}
