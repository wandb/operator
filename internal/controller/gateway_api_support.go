package controller

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func isAPISupported(mapper apimeta.RESTMapper, groupKind schema.GroupKind, versions ...string) (bool, error) {
	if mapper == nil {
		return false, nil
	}

	_, err := mapper.RESTMapping(groupKind, versions...)
	if err == nil {
		return true, nil
	}
	if apimeta.IsNoMatchError(err) {
		return false, nil
	}
	return false, err
}
