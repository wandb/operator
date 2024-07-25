package state

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"errors"

	"github.com/wandb/operator/pkg/wandb/spec"
)

var (
	ErrNotFound = errors.New("version not found")
)

//counterfeiter:generate . State
type State interface {
	Set(string, string, *spec.Spec) error
	Get(string, string) (*spec.Spec, error)
}
