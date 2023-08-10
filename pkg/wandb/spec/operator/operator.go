package operator

import (
	"os"

	v1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/release"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// Defaults sets properties on the spec that are specific to the operator. Such
// as owner reference and namespace.
func Defaults(wandb *v1.WeightsAndBiases, scheme *runtime.Scheme) *spec.Spec {
	opNs := os.Getenv("OPERATOR_NAMESPACE")
	if opNs == "" {
		opNs = "wandb"
	}
	gvk, _ := apiutil.GVKForObject(wandb, scheme)
	return &spec.Spec{
		Release: nil,
		Config: map[string]interface{}{
			"operator": map[string]interface{}{
				"namespace": os.Getenv("OPERATOR_NAMESPACE"),
			},
			"customResource": map[string]interface{}{
				"name":       wandb.GetName(),
				"namespace":  wandb.GetNamespace(),
				"apiVersion": wandb.APIVersion,
			},
			"global": map[string]interface{}{
				"metadata": map[string]interface{}{
					"ownerReferences": []map[string]interface{}{
						{
							"apiVersion":         gvk.GroupVersion().String(),
							"blockOwnerDeletion": true,
							"controller":         true,
							"kind":               gvk.Kind,
							"name":               wandb.GetName(),
							"uid":                wandb.GetUID(),
						},
					},
				},
			},
		},
	}
}

func Spec(wandb *v1.WeightsAndBiases) *spec.Spec {
	return &spec.Spec{
		Release: release.Get(wandb.Spec.Release.Object),
		Config:  wandb.Spec.Config.Object,
	}
}
