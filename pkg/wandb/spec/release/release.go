package release

import (
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/release/cdk8s"
)

func Get(maybeRelease interface{}) spec.Release {
	if maybeRelease == nil {
		return nil
	}

	if s := cdk8s.GetCdk8sJobSpec(maybeRelease); s != nil {
		return s
	}

	if s := cdk8s.GetCdk8sGitSpec(maybeRelease); s != nil {
		return s
	}

	if s := cdk8s.GetCdk8sLocalSpec(maybeRelease); s != nil {
		return s
	}

	return nil
}
