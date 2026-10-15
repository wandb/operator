package reconciler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	oputils "github.com/wandb/operator/pkg/utils"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func generateSecretValue(gs serverManifest.GeneratedSecret) ([]byte, error) {
	switch gs.CharacterType {
	case "hex":
		if gs.Length <= 0 || gs.Length%2 != 0 {
			return nil, fmt.Errorf("generated secret %q requires a positive even hex length", gs.Name)
		}
		value := make([]byte, gs.Length/2)
		if _, err := rand.Read(value); err != nil {
			return nil, err
		}
		return []byte(hex.EncodeToString(value)), nil
	case "", "password":
		length := gs.Length
		if length <= 0 {
			length = 32
		}
		value, err := oputils.GenerateRandomPassword(length)
		return []byte(value), err
	default:
		return nil, fmt.Errorf("generated secret %q has unsupported type %q", gs.Name, gs.CharacterType)
	}
}

func generateSecrets(ctx context.Context, client ctrlclient.Client, wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) (ctrl.Result, error) {
	statusBefore := wandb.DeepCopy().Status
	if wandb.Status.GeneratedSecrets == nil {
		wandb.Status.GeneratedSecrets = map[string]corev1.SecretKeySelector{}
	}
	for _, gs := range manifest.GeneratedSecrets {
		if len(gs.Features) > 0 && !manifest.FeaturesEnabled(gs.Features) {
			delete(wandb.Status.GeneratedSecrets, gs.Name)
			continue // Retain the Secret so re-enabling does not rotate credentials.
		}
		secretName := gs.Name
		if !gs.UseExactName {
			secretName = fmt.Sprintf("%s-%s", wandb.Name, gs.Name)
		}
		sec := &corev1.Secret{}
		err := client.Get(ctx, ctrlclient.ObjectKey{Name: secretName, Namespace: wandb.Namespace}, sec)
		if apierrors.IsNotFound(err) {
			value, err := generateSecretValue(gs)
			if err != nil {
				return ctrl.Result{}, err
			}
			sec = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: secretName, Namespace: wandb.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "wandb-operator",
						"app.kubernetes.io/instance":   wandb.Name,
						"app.kubernetes.io/part-of":    "wandb",
					},
				},
				Data: map[string][]byte{"key": value}, Type: corev1.SecretTypeOpaque,
			}
			if err := controllerutil.SetOwnerReference(wandb, sec, client.Scheme()); err != nil {
				return ctrl.Result{}, err
			}
			if err := client.Create(ctx, sec); err != nil {
				return ctrl.Result{}, err
			}
		} else if err != nil {
			return ctrl.Result{}, err
		} else if gs.CharacterType == "hex" {
			if !isOwnedBy(sec, wandb) {
				return ctrl.Result{}, fmt.Errorf("generated secret %q is not owned by this WeightsAndBiases CR", secretName)
			}
			value := sec.Data["key"]
			if gs.Length <= 0 || gs.Length%2 != 0 || len(value) != gs.Length {
				return ctrl.Result{}, fmt.Errorf("generated secret %q must contain a %d-character hex key; refusing to rotate it", secretName, gs.Length)
			}
			if _, err := hex.DecodeString(string(value)); err != nil {
				return ctrl.Result{}, fmt.Errorf("generated secret %q contains a non-hex key; refusing to rotate it", secretName)
			}
		} else if gs.CharacterType != "" && gs.CharacterType != "password" {
			return ctrl.Result{}, fmt.Errorf("generated secret %q has unsupported type %q", gs.Name, gs.CharacterType)
		} else if len(sec.Data["key"]) == 0 {
			value, err := generateSecretValue(gs)
			if err != nil {
				return ctrl.Result{}, err
			}
			if sec.Data == nil {
				sec.Data = map[string][]byte{}
			}
			sec.Data["key"] = value
			if err := client.Update(ctx, sec); err != nil {
				return ctrl.Result{}, err
			}
		}
		wandb.Status.GeneratedSecrets[gs.Name] = corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: "key",
		}
	}
	return ctrl.Result{}, updateWandbStatusIfChanged(ctx, client, wandb, statusBefore)
}

// Hash only generated credentials actually consumed by this workload. Infra
// credentials can rotate independently without requiring schema migrations.
func generatedSecretChecksum(ctx context.Context, client ctrlclient.Client, wandb *apiv2.WeightsAndBiases, envs []corev1.EnvVar) (string, error) {
	values := map[string][]byte{}
	for _, env := range envs {
		if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
			continue
		}
		ref := env.ValueFrom.SecretKeyRef
		for _, generated := range wandb.Status.GeneratedSecrets {
			if ref.Name != generated.Name || ref.Key != generated.Key {
				continue
			}
			var secret corev1.Secret
			if err := client.Get(ctx, ctrlclient.ObjectKey{Namespace: wandb.Namespace, Name: ref.Name}, &secret); err != nil {
				return "", err
			}
			value, ok := secret.Data[ref.Key]
			if !ok {
				return "", fmt.Errorf("generated secret %q is missing key %q", ref.Name, ref.Key)
			}
			values[ref.Name+"/"+ref.Key] = value
		}
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}
