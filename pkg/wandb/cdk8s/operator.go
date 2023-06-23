package cdk8s

import (
	"github.com/Masterminds/semver"
	v1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/cdk8s/config"
	"github.com/wandb/operator/pkg/wandb/cdk8s/release"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type operatorChannel struct {
	wandb  *v1.WeightsAndBiases
	scheme *runtime.Scheme
}

func (c operatorChannel) Recommend() *config.Config {
	gvk, _ := apiutil.GVKForObject(c.wandb, c.scheme)
	return &config.Config{
		Config: map[string]interface{}{
			"global": map[string]interface{}{
				"metadata": map[string]interface{}{
					"ownerReferences": []map[string]interface{}{
						{
							"apiVersion":         gvk.GroupVersion().String(),
							"blockOwnerDeletion": true,
							"controller":         true,
							"kind":               gvk.Kind,
							"name":               c.wandb.GetName(),
							"uid":                c.wandb.GetUID(),
						},
					},
				},
			},
		},
	}
}

func (c operatorChannel) Override() *config.Config {
	version := c.wandb.Spec.Cdk8sVersion
	// Spec doesn't have a version, so we don't need to override anything
	if version == "" {
		return &config.Config{}
	}

	if v, err := semver.NewVersion(version); err == nil {
		return &config.Config{
			Release: &release.GithubRelease{
				Repo: "wandb/cdk8s",
				Tag:  v.String(),
			},
		}
	}

	return &config.Config{
		Release: release.NewLocalRelease(version),
	}
}

func Operator(wandb *v1.WeightsAndBiases, scheme *runtime.Scheme) config.Modifier {
	return &operatorChannel{wandb, scheme}
}
