package cdk8s

import (
	"github.com/wandb/operator/pkg/wandb/cdk8s/config"
)

// Deployer returns the config suggested by deployer
func Deployer(license string) config.Modifier {
	return &deployerChannel{}
}

type deployerResponse struct {
	recommend *config.Config
	override  *config.Config
}

type deployerChannel struct {
	response deployerResponse
}

func (c deployerChannel) Recommend(_ *config.Config) *config.Config {
	return c.response.recommend
}

func (c deployerChannel) Override(_ *config.Config) *config.Config {
	return c.response.override
}
