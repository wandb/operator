package utils

import (
	"k8s.io/apimachinery/pkg/runtime"
)

func IsRegistered(scheme *runtime.Scheme, obj runtime.Object) bool {
	gvks, _, err := scheme.ObjectKinds(obj)
	if err != nil || len(gvks) == 0 {
		return false
	}
	return true
}
