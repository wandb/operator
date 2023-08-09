package state

import (
	"errors"

	"github.com/wandb/operator/pkg/wandb/spec"
)

var (
	ErrNotFound = errors.New("version not found")
)

type State interface {
	Set(string, string, *spec.Spec) error
	Get(string, string) (*spec.Spec, error)
}
