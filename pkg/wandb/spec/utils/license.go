package utils

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/wandb/operator/pkg/wandb/spec"
)

func GetLicense(specs ...*spec.Spec) string {
	log := ctrllog.Log.WithName("GetLicense")

	kubeClient, err := createKubeClient()
	if err != nil {
		log.Error(err, "Error creating Kubernetes client")
		return ""
	}

	for _, s := range specs {
		if s == nil || s.Values == nil {
			continue
		}

		secretName := s.Values.GetString("global.licenseSecret.name")
		secretKey := s.Values.GetString("global.licenseSecret.key")
		secretNamespace := s.Values.GetString("global.licenseSecret.namespace")
		if secretNamespace == "" {
			secretNamespace = "default"
		}

		if secretName != "" && secretKey != "" {
			license := getLicenseFromSecret(kubeClient, secretName, secretKey, secretNamespace)
			if license != "" {
				log.Info("License retrieved from Kubernetes secret", "secretName", secretName, "namespace", secretNamespace)
				return license
			}
		}

		license := s.Values.GetString("global.license")
		if license != "" {
			log.Info("License retrieved from values.yaml")
			return license
		}
	}
	return ""
}

func createKubeClient() (client.Client, error) {
	kubeConfig, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	kubeClient, err := client.New(kubeConfig, client.Options{})
	if err != nil {
		return nil, err
	}
	return kubeClient, nil
}

func getLicenseFromSecret(kubeClient client.Client, secretName, secretKey, secretNamespace string) string {
	log := ctrllog.Log.WithName("getLicenseFromSecret")
	secret := &corev1.Secret{}
	ctx := context.TODO()

	err := kubeClient.Get(ctx, client.ObjectKey{Name: secretName, Namespace: secretNamespace}, secret)
	if err != nil {
		log.Error(err, "Error retrieving secret", "secretName", secretName, "namespace", secretNamespace)
		return ""
	}

	if secret.Data == nil {
		log.Info("Secret has no data", "secretName", secretName, "namespace", secretNamespace)
		return ""
	}
	license, exists := secret.Data[secretKey]
	if !exists {
		log.Info("Key not found in secret", "secretKey", secretKey, "secretName", secretName, "namespace", secretNamespace)
		return ""
	}
	log.Info("Successfully retrieved license from secret", "secretName", secretName, "namespace", secretNamespace)
	return string(license)
}
