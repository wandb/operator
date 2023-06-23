package config

import (
	"reflect"

	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/wandb/cdk8s/release"
)

// Config is the configuration for the cdk8s. It holds information such as the
// cdk8s version and config to apply.
type Config struct {
	Release release.Release
	Config  map[string]interface{}
}

func (s *Config) SetRelease(release release.Release) {
	s.Release = release
}

func (s *Config) SetConfig(config map[string]interface{}) {
	s.Config = config
}

func (s Config) Equals(config *Config) bool {
	if s.Release.Version() != config.Release.Version() {
		return false
	}
	return reflect.DeepEqual(s.Config, config.Config)
}

// Config passed in will be merged with current config. If any keys match, the
// config passed in will take presentient
func (s *Config) Merge(config *Config) {
	if config.Release != nil {
		s.SetRelease(config.Release)
	}

	if s.Config == nil {
		s.SetConfig(config.Config)
		return
	}

	if config.Config != nil {
		if cfg, err := utils.Merge(s.Config, config.Config); err != nil {
			if cfgMap, ok := cfg.(map[string]interface{}); ok {
				s.SetConfig(cfgMap)
			}
		}
	}
}
