package retention

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
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
		if gvk.Group != "" && gvk.Group != "rbac.authorization.k8s.io" && gvk.Group != "gateway.networking.k8s.io" {
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
	// Resolve vendor workload replacements before their existing children.
	slices.SortStableFunc(objects, func(a, b *unstructured.Unstructured) int {
		order := func(kind string) int {
			switch kind {
			case "Deployment", "StatefulSet", "ReplicaSet", "Pod":
				return 1 + workloadOrder(kind)
			default:
				return 0
			}
		}
		return order(a.GetKind()) - order(b.GetKind())
	})
	// Other owner chains may still be listed in any order.
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
			var parent Resource
			found := false
			for _, ref := range obj.GetOwnerReferences() {
				if p, ok := owners[ref.UID]; ok {
					if found && InstanceID(parent.Component, parent.Instance) != InstanceID(p.Component, p.Instance) {
						return fmt.Errorf("descendant %s is shared by multiple infrastructure instances", candidate.Key())
					}
					parent = p
					found = true
				}
			}
			if existing, ok := s.Ledger.Resources[candidate.Key()]; ok {
				if found && !existing.Supplied && InstanceID(existing.Component, existing.Instance) != InstanceID(parent.Component, parent.Instance) {
					return fmt.Errorf("descendant %s has an owner in another infrastructure instance", candidate.Key())
				}
				// A vendor's in-flight reconcile can recreate an explicitly purged
				// claim after its workload/root disappeared. A reused name alone is
				// insufficient: require our persisted delete intent, full installation
				// association, and independent owner or StatefulSet-template proof.
				if existing.Stage == 3 && existing.UID != obj.GetUID() && existing.DeleteRequested && s.Ledger.Deleting && string(existing.Policy) == "purge" && associated(obj, s.parent, existing) {
					if !found {
						parent, found = s.claimParent(obj, objects, owners)
					}
					if found && parent.Component == existing.Component && parent.Instance == existing.Instance {
						existing.UID = obj.GetUID()
						existing.DeleteRequested = false
						if err := s.reconcileObject(ctx, existing, obj); err != nil {
							return err
						}
						changed = true
					}
				}
				if existing.UID != obj.GetUID() && (existing.Stage == 1 || existing.Stage == 2) && obj.GetDeletionTimestamp().IsZero() {
					if err := s.reconcileObject(ctx, existing, obj); err != nil {
						return err
					}
					changed = true
				}
				continue
			}
			if !found && obj.GetKind() == "PersistentVolumeClaim" {
				parent, found = s.claimParent(obj, objects, owners)
			}
			if !found {
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
			if s.Ledger.Deleting && string(candidate.Policy) == "purge" && !obj.GetDeletionTimestamp().IsZero() {
				if err := s.save(ctx); err != nil {
					return err
				}
				changed = true
				continue
			}
			if err := s.reconcileObject(ctx, candidate, obj); err != nil {
				return err
			}
			changed = true
		}
	}
	// Claims can precede their StatefulSet in an API list. Reject unknown
	// provenance only after the entire owner graph reaches a fixed point.
	for _, obj := range objects {
		candidate := Resource{APIVersion: obj.GetAPIVersion(), Kind: obj.GetKind(), Namespace: obj.GetNamespace(), Name: obj.GetName()}
		if _, ok := s.Ledger.Resources[candidate.Key()]; ok {
			continue
		}
		group := obj.GroupVersionKind().Group
		primary := group != "" && group != "apps" && group != "apps.wandb.com" && group != "rbac.authorization.k8s.io" && group != "policy"
		if ownedBy(obj, s.parent.UID) && (primary || isInfraComponent(obj.GetLabels()[common.WandbComponentLabel])) {
			return fmt.Errorf("unmapped legacy infrastructure %s; restore its instance configuration before deletion", candidate.Key())
		}
		if obj.GetKind() == "PersistentVolumeClaim" && obj.GetLabels()[common.WandbNameLabel] == s.parent.Name && obj.GetLabels()[common.WandbNamespaceLabel] == s.parent.Namespace && isInfraComponent(obj.GetLabels()[common.WandbComponentLabel]) {
			return fmt.Errorf("cannot verify instance provenance for claim %s", candidate.Key())
		}
	}
	return s.discoverRoutes(ctx, namespaces)
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
	workloads := map[string]Resource{}
	for _, r := range s.Ledger.Resources {
		if r.Kind == "StatefulSet" && r.UID != "" && !r.Supplied && r.Namespace == claim.GetNamespace() {
			workloads[r.Name] = r
		}
	}
	for _, obj := range objects {
		if obj.GetKind() != "StatefulSet" || obj.GetNamespace() != claim.GetNamespace() {
			continue
		}
		parent, ok := owners[obj.GetUID()]
		if !ok {
			continue
		}
		parent.ClaimTemplates = claimTemplates(obj)
		workloads[obj.GetName()] = parent
	}
	for workloadName, parent := range workloads {
		for _, template := range parent.ClaimTemplates {
			name := template.Name
			ordinal, ok := strings.CutPrefix(claim.GetName(), name+"-"+workloadName+"-")
			if !ok || name == "" {
				continue
			}
			n, err := strconv.Atoi(ordinal)
			if err != nil || n < 0 || strconv.Itoa(n) != ordinal {
				continue
			}
			labels := template.Labels
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

func claimTemplates(sts *unstructured.Unstructured) []ClaimTemplate {
	entries, _, _ := unstructured.NestedSlice(sts.Object, "spec", "volumeClaimTemplates")
	selector, _, _ := unstructured.NestedStringMap(sts.Object, "spec", "selector", "matchLabels")
	var result []ClaimTemplate
	for _, entry := range entries {
		template, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		name, _, _ := unstructured.NestedString(template, "metadata", "name")
		labels, _, _ := unstructured.NestedStringMap(template, "metadata", "labels")
		if labels == nil {
			labels = map[string]string{}
		}
		// Kubernetes stamps the StatefulSet selector onto each generated claim,
		// even when a vendor supplies an otherwise unlabeled claim template.
		maps.Copy(labels, selector)
		result = append(result, ClaimTemplate{Name: name, Labels: labels})
	}
	return result
}
