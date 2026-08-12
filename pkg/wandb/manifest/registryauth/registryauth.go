// Package registryauth resolves credentials for pulling the server manifest from
// a private registry. It bridges Kubernetes dockerconfigjson pull secrets and the
// operator pod's ambient cloud identity to an ORAS credential, keeping the
// manifest package itself free of any k8s client dependency. It is shared by the
// reconciler and the v1→v2 conversion webhook.
package registryauth

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"

	"github.com/wandb/operator/pkg/wandb/manifest"
)

// Resolve builds a *manifest.RegistryAuth for pulling from a registry, matched
// per host the way kubelet resolves image pulls: dockerconfigjson pull secrets
// first, then — when allowAmbient is set — the operator pod's ambient cloud
// identity (ECR/GAR/ACR), then anonymous. It returns (nil, nil) when neither
// source applies, so the pull stays anonymous (the public default).
//
// allowAmbient is gated by the caller to the non-default (private) registries so
// the anonymous public default never pays the cost of a cloud credential probe.
// The reader may be a full client.Client (reconciler) or a bare Reader
// (mgr.GetAPIReader() in the conversion webhook). A referenced pull secret that
// is missing or not a dockerconfigjson is a hard error so misconfiguration is
// surfaced rather than silently falling back.
func Resolve(
	ctx context.Context,
	reader ctrlclient.Reader,
	namespace string,
	pullSecrets []corev1.LocalObjectReference,
	allowAmbient bool,
) (*manifest.RegistryAuth, error) {
	var funcs []auth.CredentialFunc

	pullSecretCred, err := pullSecretCredential(ctx, reader, namespace, pullSecrets)
	if err != nil {
		return nil, err
	}
	if pullSecretCred != nil {
		funcs = append(funcs, pullSecretCred)
	}
	if allowAmbient {
		funcs = append(funcs, ambientCredential())
	}

	if len(funcs) == 0 {
		return nil, nil
	}
	return &manifest.RegistryAuth{Credential: chainCredentials(funcs)}, nil
}

// pullSecretCredential builds an ORAS credential function from the referenced
// dockerconfigjson secrets, or (nil, nil) when none are referenced.
func pullSecretCredential(
	ctx context.Context,
	reader ctrlclient.Reader,
	namespace string,
	pullSecrets []corev1.LocalObjectReference,
) (auth.CredentialFunc, error) {
	stores := make([]credentials.Store, 0, len(pullSecrets))
	for _, ref := range pullSecrets {
		if ref.Name == "" {
			continue
		}
		var secret corev1.Secret
		if err := reader.Get(ctx, ctrlclient.ObjectKey{Namespace: namespace, Name: ref.Name}, &secret); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("image pull secret %s/%s not found", namespace, ref.Name)
			}
			return nil, fmt.Errorf("read image pull secret %s/%s: %w", namespace, ref.Name, err)
		}
		cfg, ok := secret.Data[corev1.DockerConfigJsonKey]
		if !ok || len(cfg) == 0 {
			return nil, fmt.Errorf("image pull secret %s/%s missing key %q", namespace, ref.Name, corev1.DockerConfigJsonKey)
		}
		store, err := credentials.NewMemoryStoreFromDockerConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("parse image pull secret %s/%s: %w", namespace, ref.Name, err)
		}
		stores = append(stores, store)
	}

	if len(stores) == 0 {
		return nil, nil
	}

	// Later secrets are consulted only for hosts the earlier ones don't cover.
	store := stores[0]
	if len(stores) > 1 {
		store = credentials.NewStoreWithFallbacks(stores[0], stores[1:]...)
	}
	return credentials.Credential(store), nil
}

// chainCredentials tries each credential function in order, returning the first
// non-empty credential for a host (kubelet-style resolution).
func chainCredentials(funcs []auth.CredentialFunc) auth.CredentialFunc {
	return func(ctx context.Context, host string) (auth.Credential, error) {
		for _, f := range funcs {
			cred, err := f(ctx, host)
			if err != nil {
				return auth.EmptyCredential, err
			}
			if cred != auth.EmptyCredential {
				return cred, nil
			}
		}
		return auth.EmptyCredential, nil
	}
}
