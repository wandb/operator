package retention

import (
	"maps"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	trackingIDAnnotation     = "argocd.argoproj.io/tracking-id"
	installationIDAnnotation = "argocd.argoproj.io/installation-id"
	syncOptionsAnnotation    = "argocd.argoproj.io/sync-options"
)

func protectArgo(obj client.Object, w *apiv2.WeightsAndBiases) {
	annotations := maps.Clone(obj.GetAnnotations())
	if annotations == nil {
		annotations = map[string]string{}
	}
	for _, key := range []string{trackingIDAnnotation, installationIDAnnotation} {
		if value := w.Annotations[key]; value != "" {
			annotations[key] = value
		} else {
			delete(annotations, key)
		}
	}
	options := []string{}
	for _, option := range strings.Split(annotations[syncOptionsAnnotation], ",") {
		option = strings.TrimSpace(option)
		key, _, _ := strings.Cut(option, "=")
		if option != "" && key != "Prune" && key != "Delete" {
			options = append(options, option)
		}
	}
	// Non-self tracking affects sync/prune, but does not independently protect
	// against deletion of the Argo Application. Purge belongs to this operator.
	options = append(options, "Prune=false", "Delete=false")
	annotations[syncOptionsAnnotation] = strings.Join(options, ",")
	obj.SetAnnotations(annotations)
}

func argoProtected(obj client.Object, w *apiv2.WeightsAndBiases) bool {
	expected := obj.DeepCopyObject().(client.Object)
	protectArgo(expected, w)
	for _, key := range []string{trackingIDAnnotation, installationIDAnnotation, syncOptionsAnnotation} {
		if obj.GetAnnotations()[key] != expected.GetAnnotations()[key] {
			return false
		}
	}
	return true
}
