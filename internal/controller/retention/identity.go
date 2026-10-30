package retention

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	UIDLabel           = "weightsandbiases.apps.wandb.com/uid"
	ComponentLabel     = "weightsandbiases.apps.wandb.com/retention-component"
	InstanceLabel      = "weightsandbiases.apps.wandb.com/instance"
	InstanceAnnotation = "weightsandbiases.apps.wandb.com/instance-key"
	ReadyCondition     = "RetentionReady"
	Finalizer          = "wandb.apps.wandb.com/cleanup"
)

// Resource identifies one object, not a label-selected collection. UID is filled
// after creation; the creation intent and association close the crash window.
type Resource struct {
	APIVersion string               `json:"apiVersion"`
	Kind       string               `json:"kind"`
	Namespace  string               `json:"namespace"`
	Name       string               `json:"name"`
	UID        types.UID            `json:"uid,omitempty"`
	Component  string               `json:"component"`
	Instance   string               `json:"instance"`
	Policy     apiv2.OnDeletePolicy `json:"policy"`
	// Routes follow their backend policy unless their managed Gateway is removed.
	RouteFor          string `json:"routeFor,omitempty"`
	RemoveWithGateway string `json:"removeWithGateway,omitempty"`
	// Stage orders explicit deletion: workloads, then supporting objects, then storage.
	Stage    int  `json:"stage"`
	Supplied bool `json:"supplied,omitempty"`
	// Records an exact-UID purge intent before DELETE, including across retries.
	DeleteRequested bool `json:"deleteRequested,omitempty"`
	// OptionalAPI is used only for inventory from removed legacy CRDs.
	OptionalAPI    bool            `json:"optionalAPI,omitempty"`
	ClaimTemplates []ClaimTemplate `json:"claimTemplates,omitempty"`
}

// Keep claim provenance after foreground deletion removes the StatefulSet.
type ClaimTemplate struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
}

func (r Resource) GVK() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(r.APIVersion, r.Kind)
}

func (r Resource) Key() string {
	return r.GVK().GroupKind().String() + "/" + r.Namespace + "/" + r.Name
}

func InstanceID(component, key string) string { return component + "/" + key }

func instanceLabel(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])[:63]
}

func associate(obj client.Object, w *apiv2.WeightsAndBiases, r Resource) {
	labels := maps.Clone(obj.GetLabels())
	if labels == nil {
		labels = map[string]string{}
	}
	// Vendor selectors can include their own component label (notably Keeper).
	// Keep it intact and use a separate key for the logical retention instance.
	for key, value := range common.BuildWandbLabels(w, r.Component) {
		if key != common.WandbComponentLabel || labels[key] == "" {
			labels[key] = value
		}
	}
	labels[ComponentLabel] = r.Component
	labels[UIDLabel] = string(w.UID)
	labels[InstanceLabel] = instanceLabel(r.Instance)
	obj.SetLabels(labels)
	annotations := maps.Clone(obj.GetAnnotations())
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[InstanceAnnotation] = r.Instance
	obj.SetAnnotations(annotations)
	common.RemoveOwnerReference(obj, w.UID)
	protectArgo(obj, w)
}

func associated(obj client.Object, w *apiv2.WeightsAndBiases, r Resource) bool {
	return obj.GetLabels()[UIDLabel] == string(w.UID) &&
		obj.GetLabels()[ComponentLabel] == r.Component &&
		obj.GetLabels()[InstanceLabel] == instanceLabel(r.Instance) &&
		obj.GetAnnotations()[InstanceAnnotation] == r.Instance
}

func ownedBy(obj client.Object, uid types.UID) bool {
	if uid == "" {
		return false
	}
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == uid {
			return true
		}
	}
	return false
}

func Policy(w *apiv2.WeightsAndBiases, spec apiv2.ManagedInfraSpec) (apiv2.OnDeletePolicy, error) {
	p := w.GetRetentionPolicy(spec).OnDelete
	if p == "" {
		p = apiv2.DetachOnDelete
	}
	if p != apiv2.DetachOnDelete && p != apiv2.PurgeOnDelete {
		return "", fmt.Errorf("invalid retention policy %q: expected detach or purge", p)
	}
	return p, nil
}
