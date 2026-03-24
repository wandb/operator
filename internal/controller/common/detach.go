package common

import (
	"github.com/wandb/operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsDetached(obj client.Object, ownerUID types.UID) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == ownerUID {
			return false
		}
	}
	return true
}

func RemoveOwnerReference(obj client.Object, ownerUID types.UID) {
	newRefs := utils.FilterFunc(obj.GetOwnerReferences(), func(ref metav1.OwnerReference) bool {
		return ref.UID != ownerUID
	})
	obj.SetOwnerReferences(newRefs)
}
