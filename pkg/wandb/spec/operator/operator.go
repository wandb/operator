package operator

import (
	"os"

	v1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/charts"
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
		Chart: nil,
		Values: map[string]interface{}{
			"global": map[string]interface{}{
				"operator": map[string]interface{}{
					"apiVersion": gvk.GroupVersion().String(),
					"namespace":  opNs,
				},
			},
		},
	}
}

// Spec returns the spec for the given CRD.
func Spec(wandb *v1.WeightsAndBiases) *spec.Spec {
	return &spec.Spec{
		Chart:  charts.Get(wandb.Spec.Chart.Object),
		Values: wandb.Spec.Values.Object,
	}
}
