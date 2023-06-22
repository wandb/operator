package cdk8s

import (
	"github.com/wandb/operator/pkg/wandb/cdk8s/config"
)

// Deployment returns the config suggested by deployer
func Deployment(license string) config.Modifier {
	return nil
}
