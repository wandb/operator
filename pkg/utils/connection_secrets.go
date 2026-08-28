package utils

import (
	"context"
	"fmt"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// connSecretResolver resolves SecretKeySelectors from an ObjectStoreConnection,
// caching each referenced secret so a connection spanning multiple secrets is
// fetched once per secret.
type ConnSecretResolver struct {
	Client    client.Client
	Namespace string
	Cache     map[string]*v1.Secret
}

// value returns the trimmed value the selector points at, or "" if the selector
// is unset or the key is absent.
func (r *ConnSecretResolver) Value(ctx context.Context, sel v1.SecretKeySelector) (string, error) {
	if sel.Name == "" || sel.Key == "" {
		return "", nil
	}
	secret, ok := r.Cache[sel.Name]
	if !ok {
		secret = &v1.Secret{}
		key := types.NamespacedName{Namespace: r.Namespace, Name: sel.Name}
		if err := r.Client.Get(ctx, key, secret); err != nil {
			return "", fmt.Errorf("read object store connection secret %q: %w", key, err)
		}
		r.Cache[sel.Name] = secret
	}
	return strings.TrimSpace(string(secret.Data[sel.Key])), nil
}

// ValueOrSecret returns a ValueOrSecret's literal value, or the trimmed value the
// secret arm points at, or "" when unset.
func (r *ConnSecretResolver) ValueOrSecret(ctx context.Context, v apiv2.ValueOrSecret) (string, error) {
	if v.Value != "" {
		return strings.TrimSpace(v.Value), nil
	}
	if ref := v.SecretKeyRef(); ref != nil {
		return r.Value(ctx, *ref)
	}
	return "", nil
}
