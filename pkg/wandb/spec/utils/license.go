package utils

import (
	"context"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/wandb/operator/pkg/wandb/spec"
)

func GetLicense(ctx context.Context, k8sClient client.Client, wandb v1.Object, specs ...*spec.Spec) string {
	log := ctrllog.FromContext(ctx).WithName("GetLicense")

	// First check if license is directly provided in values
	for _, s := range specs {
		if s == nil || s.Values == nil {
			continue
		}

		license := s.Values.GetString("global.license")
		if license != "" {
			log.Info("License retrieved from values.yaml")
			return license
		}
	}

	// Then try to get from secret if no direct license was found
	for _, s := range specs {
		if s == nil || s.Values == nil {
			continue
		}

		secretName := s.Values.GetString("global.licenseSecret.name")
		secretKey := s.Values.GetString("global.licenseSecret.key")
		secretNamespace := wandb.GetNamespace()

		if secretName != "" && secretKey != "" {
			license := getLicenseFromSecret(k8sClient, secretName, secretKey, secretNamespace)
			if license != "" {
				log.Info("License retrieved from Kubernetes secret")
				return license
			}
		}
	}
	return ""
}

func getLicenseFromSecret(k8sClient client.Client, secretName, secretKey, secretNamespace string) string {
	log := ctrllog.Log.WithName("getLicenseFromSecret")
	secret := &corev1.Secret{}

	err := k8sClient.Get(context.Background(), client.ObjectKey{Name: secretName, Namespace: secretNamespace}, secret)
	if err != nil {
		log.Error(err, "Error retrieving secret")
		return ""
	}

	if secret.Data == nil {
		log.Info("Secret has no data")
		return ""
	}
	license, exists := secret.Data[secretKey]
	if !exists {
		log.Info("Key not found in secret")
		return ""
	}

	if len(license) == 0 {
		log.Info("Empty license value in secret")
		return ""
	}

	log.Info("Successfully retrieved license from secret")
	return string(license)
}
