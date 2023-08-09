package cdk8s

import (
	"context"
	"encoding/json"

	"github.com/go-playground/validator/v10"
	v1 "github.com/wandb/operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetCdk8sLocalSpec(s interface{}) *Cdk8sLocal {
	release := &Cdk8sLocal{}
	specBytes, _ := json.Marshal(s)

	if err := json.Unmarshal(specBytes, release); err != nil {
		return nil
	}

	if err := release.Validate(); err != nil {
		return nil
	}

	return release
}

type Cdk8sLocal struct {
	Directory string `validate:"required,dir" json:"directory"`
}

func (c *Cdk8sLocal) Validate() error {
	return validator.New().Struct(c)
}

func (c Cdk8sLocal) Apply(
	ctx context.Context,
	client client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
	config map[string]interface{},
) error {
	if err := PnpmInstall(c.Directory); err != nil {
		return err
	}

	if err := PnpmGenerate(c.Directory, config); err != nil {
		return err
	}

	if err := KubectlApply(c.Directory, wandb.GetNamespace()); err != nil {
		return err
	}

	return nil
}
