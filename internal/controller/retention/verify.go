package retention

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *Session) verifyOwnerPath(ctx context.Context, obj client.Object, retained bool, seen map[types.UID]bool) error {
	if seen[obj.GetUID()] {
		return nil
	}
	seen[obj.GetUID()] = true
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == s.parent.UID {
			return fmt.Errorf("%s/%s still has an ownership path to WandB", obj.GetNamespace(), obj.GetName())
		}
		if retained {
			for _, r := range s.Ledger.Resources {
				if r.UID == ref.UID && r.Policy == apiv2.PurgeOnDelete && !r.Supplied {
					return fmt.Errorf("retained object %s/%s is owned by purged resource %s", obj.GetNamespace(), obj.GetName(), r.Key())
				}
			}
		}
		owner := &unstructured.Unstructured{}
		owner.SetAPIVersion(ref.APIVersion)
		owner.SetKind(ref.Kind)
		if err := s.reader.Get(ctx, client.ObjectKey{Namespace: obj.GetNamespace(), Name: ref.Name}, owner); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		if owner.GetUID() != ref.UID {
			return fmt.Errorf("owner of %s/%s was replaced", obj.GetNamespace(), obj.GetName())
		}
		if err := s.verifyOwnerPath(ctx, owner, retained, seen); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) verifyReferences(ctx context.Context, obj *unstructured.Unstructured, r Resource) error {
	if r.Policy != apiv2.DetachOnDelete {
		return nil
	}
	refs := map[string]string{}
	var walk func(any)
	walk = func(value any) {
		switch node := value.(type) {
		case []any:
			for _, v := range node {
				walk(v)
			}
		case map[string]any:
			for key, v := range node {
				kind := ""
				field := "name"
				switch key {
				case "secretKeyRef", "secretRef", "secret":
					kind = "Secret"
					if key == "secret" {
						field = "secretName"
					}
				case "configMapKeyRef", "configMapRef", "configMap":
					kind = "ConfigMap"
				}
				if kind != "" {
					if child, ok := v.(map[string]any); ok {
						if name, ok := child[field].(string); ok && name != "" {
							refs[kind+"/"+name] = kind
						}
					}
				}
				if key == "serviceAccountName" {
					if name, ok := v.(string); ok && name != "" {
						refs["ServiceAccount/"+name] = "ServiceAccount"
					}
				}
				if key == "imagePullSecrets" {
					if entries, ok := v.([]any); ok {
						for _, entry := range entries {
							if child, ok := entry.(map[string]any); ok {
								if name, ok := child["name"].(string); ok {
									refs["Secret/"+name] = "Secret"
								}
							}
						}
					}
				}
				walk(v)
			}
		}
	}
	walk(obj.Object["spec"])
	for id, kind := range refs {
		name := id[len(kind)+1:]
		dependency := &unstructured.Unstructured{}
		dependency.SetAPIVersion("v1")
		dependency.SetKind(kind)
		if err := s.reader.Get(ctx, client.ObjectKey{Namespace: r.Namespace, Name: name}, dependency); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		for _, candidate := range s.Ledger.Resources {
			if candidate.UID == dependency.GetUID() && candidate.Policy == apiv2.PurgeOnDelete && !candidate.Supplied {
				return fmt.Errorf("retained %s requires purged %s", r.Key(), candidate.Key())
			}
		}
		if err := s.verifyOwnerPath(ctx, dependency, true, map[types.UID]bool{}); err != nil {
			return err
		}
	}
	return nil
}
