package reconciler

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/pkg/utils"
	servermanifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const manifestSecretLabel = "weightsandbiases.apps.wandb.com/manifest-secret"

// Cleanup uses persisted identities rather than fetching a possibly unavailable
// manifest while deletion is waiting on our finalizer.
func finalizeManifestSecrets(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases) error {
	if wandb.Spec.RetentionPolicy.OnDelete == apiv2.PurgeOnDelete {
		return nil
	}
	legacyNames := map[string]bool{}
	for _, selector := range wandb.Status.GeneratedSecrets {
		legacyNames[selector.Name] = true
	}
	secrets := &corev1.SecretList{}
	if err := c.List(ctx, secrets, client.InNamespace(wandb.Namespace)); err != nil {
		return fmt.Errorf("list manifest Secrets for detach: %w", err)
	}
	for i := range secrets.Items {
		secret := &secrets.Items[i]
		if !hasOwnerReference(secret, wandb) || (secret.Labels[manifestSecretLabel] != "true" && !legacyNames[secret.Name]) {
			continue
		}
		common.RemoveOwnerReference(secret, wandb.UID)
		if err := c.Update(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("detach manifest Secret %s: %w", secret.Name, err)
		}
	}
	return nil
}

func manifestSecretName(wandb *apiv2.WeightsAndBiases, declaration servermanifest.Secret) string {
	if declaration.UseExactName {
		return declaration.Name
	}
	return wandb.Name + "-" + declaration.Name
}

func validateManifestSecrets(wandb *apiv2.WeightsAndBiases, declarations []servermanifest.Secret) error {
	names := map[string]bool{}
	resources := map[string]bool{}
	for _, declaration := range declarations {
		name := manifestSecretName(wandb, declaration)
		if declaration.Name == "" {
			return fmt.Errorf("manifest Secret requires a name")
		}
		if problems := validation.IsDNS1123Subdomain(name); len(problems) != 0 {
			return fmt.Errorf("invalid manifest Secret name %q: %v", name, problems)
		}
		if names[declaration.Name] || resources[name] {
			return fmt.Errorf("duplicate manifest Secret declaration %q resolves to %q", declaration.Name, name)
		}
		names[declaration.Name] = true
		resources[name] = true
	}
	return nil
}

func reconcileManifestSecrets(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, manifest servermanifest.Manifest) (ctrl.Result, error) {
	declarations := manifest.SecretDeclarations()
	if err := validateManifestSecrets(wandb, declarations); err != nil {
		return ctrl.Result{}, err
	}
	statusBefore := wandb.DeepCopy().Status
	if wandb.Status.GeneratedSecrets == nil {
		wandb.Status.GeneratedSecrets = map[string]corev1.SecretKeySelector{}
	}
	for _, declaration := range declarations {
		name := manifestSecretName(wandb, declaration)
		secret := &corev1.Secret{}
		err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: wandb.Namespace}, secret)
		missing := apierrors.IsNotFound(err)
		if err != nil && !missing {
			return ctrl.Result{}, err
		}
		changed := false
		if missing {
			secret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: wandb.Namespace, Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wandb-operator", "app.kubernetes.io/instance": wandb.Name, "app.kubernetes.io/part-of": "wandb",
			}}, Type: corev1.SecretTypeOpaque}
		}
		if !secret.DeletionTimestamp.IsZero() {
			return ctrl.Result{}, fmt.Errorf("manifest Secret %s is being deleted", name)
		}
		if len(secret.OwnerReferences) == 0 {
			if err := controllerutil.SetOwnerReference(wandb, secret, c.Scheme()); err != nil {
				return ctrl.Result{}, err
			}
			changed = true
		} else if !hasOwnerReference(secret, wandb) {
			return ctrl.Result{}, fmt.Errorf("refusing to adopt manifest Secret %s owned by another resource", name)
		}
		if secret.Labels[manifestSecretLabel] != "true" {
			if secret.Labels == nil {
				secret.Labels = map[string]string{}
			}
			secret.Labels[manifestSecretLabel] = "true"
			changed = true
		}
		if generation := declaration.Generated; generation != nil && secret.Data["key"] == nil && secret.StringData["key"] == "" {
			length := generation.Length
			if length <= 0 {
				length = 32
			}
			value, err := utils.GenerateRandomPassword(length)
			if err != nil {
				return ctrl.Result{}, err
			}
			if secret.Data == nil {
				secret.Data = map[string][]byte{}
			}
			secret.Data["key"] = []byte(value)
			changed = true
		}
		if missing {
			if err := c.Create(ctx, secret); err != nil {
				return ctrl.Result{}, err
			}
		} else if changed {
			if err := c.Update(ctx, secret); err != nil {
				return ctrl.Result{}, err
			}
		}
		if declaration.Generated != nil {
			// Preserve the existing status/env-reference contract for generated values.
			wandb.Status.GeneratedSecrets[declaration.Name] = corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: name}, Key: "key"}
		}
	}
	if err := updateWandbStatusIfChanged(ctx, c, wandb, statusBefore); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
