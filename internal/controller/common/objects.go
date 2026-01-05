package common

import (
	"github.com/wandb/operator/internal/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetControllerOwner(obj metav1.Object) (metav1.OwnerReference, bool) {
	ctrlRefs := utils.FilterFunc(obj.GetOwnerReferences(), func(ref metav1.OwnerReference) bool {
		resultPtr := ref.Controller
		return resultPtr != nil && *resultPtr
	})
	if len(ctrlRefs) == 0 {
		return metav1.OwnerReference{}, false
	}
	// note: kubernetes enforces no more than one ctrlRefs
	return ctrlRefs[0], true
}
