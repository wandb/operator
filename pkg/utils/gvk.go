package utils

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
)

var serverResources = map[string]bool{}

func IsRegistered(scheme *runtime.Scheme, obj runtime.Object) bool {
	gvks, _, err := scheme.ObjectKinds(obj)
	if err != nil || len(gvks) == 0 {
		return false
	}
	return IsServerResource(fmt.Sprintf("%s.%s/%s", gvks[0].Kind, gvks[0].Group, gvks[0].Kind))
}

func AddServerResource(resource string) {
	serverResources[resource] = true
}

func IsServerResource(resource string) bool {
	if value, exists := serverResources[resource]; exists && value {
		return true
	}
	return false
}
