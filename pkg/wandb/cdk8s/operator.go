package cdk8s

import (
	"os"

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

func (c operatorChannel) Recommend(_ *config.Config) *config.Config {
	opNs := os.Getenv("OPERATOR_NAMESPACE")
	if opNs == "" {
		opNs = "wandb"
	}
	gvk, _ := apiutil.GVKForObject(c.wandb, c.scheme)
	return &config.Config{
		Config: map[string]interface{}{
			"console": map[string]interface{}{
				"operator": map[string]interface{}{
					"namespace": os.Getenv("OPERATOR_NAMESPACE"),
				},
				"name":      c.wandb.GetName(),
				"namespace": c.wandb.GetNamespace(),
			},
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

func (c operatorChannel) Override(_ *config.Config) *config.Config {
	cfg := &config.Config{}

	cfg.SetConfig(c.wandb.Spec.Config.Object)

	version := c.wandb.Spec.Version
	release, err := release.ReleaseFromString(version)
	if err == nil {
		cfg.SetRelease(release)
	}

	return cfg
}

func Operator(wandb *v1.WeightsAndBiases, scheme *runtime.Scheme) config.Modifier {
	return &operatorChannel{wandb, scheme}
}
