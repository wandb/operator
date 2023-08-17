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

type Release interface {
	Apply(
		context.Context,
		client.Client,
		*v1.WeightsAndBiases,
		*runtime.Scheme,
		Config,
	) error

	Prune(
		context.Context,
		client.Client,
		*v1.WeightsAndBiases,
		*runtime.Scheme,
		Config,
	) error
}

type HelmRelease interface {
	Release
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
	Metadata Metadata `json:"metadata"`

	// Release contains information about what version of the service to deploy.
	Release Release `json:"release"`

	// Config contains information about how to configure the release. This is
	// passed into the release via the Apply command to generate the manifests.
	Config Config `json:"config"`
}

func (s *Spec) Apply(
	ctx context.Context,
	client client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
) error {
	return s.Release.Apply(ctx, client, wandb, scheme, s.Config)
}

func (s *Spec) Prune(
	ctx context.Context,
	client client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
) error {
	return s.Release.Prune(ctx, client, wandb, scheme, s.Config)
}

func (s *Spec) SetRelease(release Release) {
	s.Release = release
}

func (s *Spec) SetConfig(config Config) {
	s.Config = config
}

func (s *Spec) IsEqual(spec *Spec) bool {
	isReleaseEqual := reflect.DeepEqual(s.Release, spec.Release)
	isValuesEqual := reflect.DeepEqual(s.Config, spec.Config)
	return isReleaseEqual && isValuesEqual
}

func (s *Spec) mergeConfig(config Config) (err error) {
	if s.Config == nil {
		s.Config = config
		return nil
	}
	if err := s.Config.Merge(config); err != nil {
		return err
	}
	return nil
}

func (s *Spec) Merge(spec *Spec) {
	if spec == nil {
		return
	}
	if spec.Release != nil {
		s.Release = spec.Release
	}
	if spec.Config != nil {
		s.mergeConfig(spec.Config)
	}
}
