package spec

import (
	"context"

	v1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// type Spec interface {
// 	SetValues(map[string]interface{})
// 	GetValues() map[string]interface{}
// 	Apply(
// 		context.Context,
// 		client.Client,
// 		*v1.WeightsAndBiases,
// 		*runtime.Scheme,
// 	) error
// 	Merge(spec Spec) Spec
// }

type Release interface {
	Apply(
		context.Context,
		client.Client,
		*v1.WeightsAndBiases,
		*runtime.Scheme,
		map[string]interface{},
	) error
}

type Spec struct {
	Release Release
	Config  map[string]interface{}
}

func (s *Spec) Apply(
	ctx context.Context,
	client client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
) error {
	return s.Release.Apply(ctx, client, wandb, scheme, s.Config)
}

func (s *Spec) SetRelease(release Release) {
	s.Release = release
}

func (s *Spec) SetConfig(config map[string]interface{}) {
	s.Config = config
}

func (s *Spec) MergeConfig(config map[string]interface{}) (err error) {
	if s.Config == nil {
		s.Config = config
		return nil
	}
	s.Config, err = utils.MergeMapString(s.Config, config)
	return err
}

func (s *Spec) Merge(spec *Spec) {
	if spec.Release != nil {
		s.Release = spec.Release
	}
	if spec.Config != nil {
		s.MergeConfig(spec.Config)
	}
}

func Merge(specs ...*Spec) *Spec {
	root := &Spec{}
	for _, spec := range specs {
		root.Merge(spec)
	}
	return root
}
