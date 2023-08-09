package cdk8s

import (
	"os"

	v1 "github.com/wandb/operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func OperatorConfig(wandb *v1.WeightsAndBiases, scheme *runtime.Scheme) map[string]interface{} {
	opNs := os.Getenv("OPERATOR_NAMESPACE")
	if opNs == "" {
		opNs = "wandb"
	}
	gvk, _ := apiutil.GVKForObject(wandb, scheme)
	return map[string]interface{}{
		"console": map[string]interface{}{
			"operator": map[string]interface{}{
				"namespace": os.Getenv("OPERATOR_NAMESPACE"),
			},
			"name":      wandb.GetName(),
			"namespace": wandb.GetNamespace(),
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
	}
}
