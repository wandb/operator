package config

import (
	"reflect"

	"github.com/wandb/operator/pkg/wandb/cdk8s/release"
)

// Config is the configuration for the cdk8s. It holds information such as the
// cdk8s version and config to apply.
type Config struct {
	Release release.Release
	Config  interface{}
}

func (s *Config) SetRelease(release release.Release) {
	s.Release = release
}

func (s *Config) SetConfig(config interface{}) {
	s.Config = config
}

func (s Config) Equals(config *Config) bool {
	if s.Release.Version() != config.Release.Version() {
		return false
	}
	return reflect.DeepEqual(s.Config, config.Config)
}
