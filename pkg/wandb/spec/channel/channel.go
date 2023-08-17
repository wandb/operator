package channel

import "github.com/wandb/operator/pkg/wandb/spec"

type Channel interface {
	Get() (*spec.Spec, error)
}
