package retention

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"

	"github.com/wandb/operator/internal/controller/common"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *Session) patchLabels(ctx context.Context, obj client.Object, r Resource, data []byte) error {
	var patch map[string]json.RawMessage
	if err := json.Unmarshal(data, &patch); err != nil {
		return err
	}
	for key := range patch {
		if key != "metadata" {
			return fmt.Errorf("retention permits only metadata label patches")
		}
	}
	var metadata map[string]json.RawMessage
	if err := json.Unmarshal(patch["metadata"], &metadata); err != nil {
		return err
	}
	for key := range metadata {
		if key != "labels" && key != "resourceVersion" {
			return fmt.Errorf("retention permits only label patches, got %s", key)
		}
	}
	var labels map[string]string
	if err := json.Unmarshal(metadata["labels"], &labels); err != nil {
		return err
	}
	current := maps.Clone(obj.GetLabels())
	if current == nil {
		current = map[string]string{}
	}
	maps.Copy(current, labels)
	obj.SetLabels(current)
	associate(obj, s.parent, r)
	return s.Client.Update(ctx, obj)
}

func descendantKinds() []schema.GroupVersionKind {
	return []schema.GroupVersionKind{
		{Version: "v1", Kind: "Secret"}, {Version: "v1", Kind: "ConfigMap"}, {Version: "v1", Kind: "Service"}, {Version: "v1", Kind: "ServiceAccount"}, {Version: "v1", Kind: "PersistentVolumeClaim"}, {Version: "v1", Kind: "Pod"},
		{Group: "apps", Version: "v1", Kind: "StatefulSet"}, {Group: "apps", Version: "v1", Kind: "Deployment"}, {Group: "apps", Version: "v1", Kind: "ReplicaSet"}, {Group: "apps", Version: "v1", Kind: "ControllerRevision"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "RoleBinding"}, {Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "Role"},
		{Group: "policy", Version: "v1", Kind: "PodDisruptionBudget"},
	}
}

func (s *Session) discover(ctx context.Context) error {
	namespaces := map[string]bool{s.parent.Namespace: true}
	for _, r := range s.Ledger.Resources {
		namespaces[r.Namespace] = true
	}
	var objects []*unstructured.Unstructured
	kinds := descendantKinds()
	for _, gvk := range WatchedKinds() {
		if gvk.Group != "" && gvk.Group != "rbac.authorization.k8s.io" {
			if _, err := s.Scheme().New(gvk); err == nil {
				kinds = append(kinds, gvk)
			}
		}
	}
	for ns := range namespaces {
		for _, gvk := range kinds {
			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
			if err := s.reader.List(ctx, list, client.InNamespace(ns)); err != nil {
				if meta.IsNoMatchError(err) {
					continue
				}
				return err
			}
			for i := range list.Items {
				list.Items[i].SetGroupVersionKind(gvk)
				objects = append(objects, &list.Items[i])
			}
		}
	}
	// Owner chains may be listed in any order (e.g. Pods before StatefulSets).
	for changed := true; changed; {
		changed = false
		owners := map[types.UID]Resource{}
		for _, r := range s.Ledger.Resources {
			if r.UID != "" && !r.Supplied {
				owners[r.UID] = r
			}
		}
		for _, obj := range objects {
			candidate := Resource{APIVersion: obj.GetAPIVersion(), Kind: obj.GetKind(), Namespace: obj.GetNamespace(), Name: obj.GetName()}
			if _, ok := s.Ledger.Resources[candidate.Key()]; ok {
				continue
			}
			var parent Resource
			found := false
			for _, ref := range obj.GetOwnerReferences() {
				if p, ok := owners[ref.UID]; ok {
					parent = p
					found = true
					break
				}
			}
			if !found && obj.GetKind() == "PersistentVolumeClaim" {
				parent, found = s.claimParent(obj, objects, owners)
			}
			if !found {
				primary := obj.GroupVersionKind().Group != "" && obj.GroupVersionKind().Group != "apps" && obj.GroupVersionKind().Group != "apps.wandb.com" && obj.GroupVersionKind().Group != "rbac.authorization.k8s.io" && obj.GroupVersionKind().Group != "policy"
				if ownedBy(obj, s.parent.UID) && (primary || isInfraComponent(obj.GetLabels()[common.WandbComponentLabel])) {
					return fmt.Errorf("unmapped legacy infrastructure %s; restore its instance configuration before deletion", candidate.Key())
				}
				if obj.GetKind() == "PersistentVolumeClaim" && obj.GetLabels()[common.WandbNameLabel] == s.parent.Name && obj.GetLabels()[common.WandbNamespaceLabel] == s.parent.Namespace && isInfraComponent(obj.GetLabels()[common.WandbComponentLabel]) {
					return fmt.Errorf("cannot verify instance provenance for claim %s", candidate.Key())
				}
				continue
			}
			candidate.Component = parent.Component
			candidate.Instance = parent.Instance
			candidate.Policy = parent.Policy
			candidate.UID = obj.GetUID()
			candidate.Stage = 2
			switch obj.GetKind() {
			case "StatefulSet", "Deployment", "ReplicaSet", "Pod":
				candidate.Stage = 1
			case "PersistentVolumeClaim":
				candidate.Stage = 3
			}
			if obj.GetLabels()[UIDLabel] != "" && !associated(obj, s.parent, candidate) {
				adopt, err := s.canAdopt(ctx, obj, candidate)
				if err != nil {
					return err
				}
				if !adopt {
					return fmt.Errorf("descendant %s belongs to another instance", candidate.Key())
				}
			}
			s.Ledger.Resources[candidate.Key()] = candidate
			if err := s.reconcileObject(ctx, candidate, obj); err != nil {
				return err
			}
			changed = true
		}
	}
	return nil
}

func isInfraComponent(value string) bool {
	switch value {
	case "mysql", "redis", "objectstore", "clickhouse", "kafka", "clickhouse-keeper":
		return true
	}
	return false
}

// StatefulSet claims deliberately have no owner when retention is Retain. Use
// the verified StatefulSet's actual template and matching template labels, not
// a component-wide selector or an arbitrary name prefix.
func (s *Session) claimParent(claim *unstructured.Unstructured, objects []*unstructured.Unstructured, owners map[types.UID]Resource) (Resource, bool) {
	if len(claim.GetOwnerReferences()) != 0 {
		return Resource{}, false
	}
	for _, obj := range objects {
		if obj.GetKind() != "StatefulSet" || obj.GetNamespace() != claim.GetNamespace() {
			continue
		}
		parent, ok := owners[obj.GetUID()]
		if !ok {
			continue
		}
		templates, _, _ := unstructured.NestedSlice(obj.Object, "spec", "volumeClaimTemplates")
		for _, entry := range templates {
			template, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			name, _, _ := unstructured.NestedString(template, "metadata", "name")
			ordinal, ok := strings.CutPrefix(claim.GetName(), name+"-"+obj.GetName()+"-")
			if !ok || name == "" {
				continue
			}
			n, err := strconv.Atoi(ordinal)
			if err != nil || n < 0 || strconv.Itoa(n) != ordinal {
				continue
			}
			labels, _, _ := unstructured.NestedStringMap(template, "metadata", "labels")
			if len(labels) == 0 {
				continue
			}
			matches := true
			for key, value := range labels {
				if claim.GetLabels()[key] != value {
					matches = false
					break
				}
			}
			if matches {
				return parent, true
			}
		}
	}
	return Resource{}, false
}
