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

		license := s.Values.GetString("global.license")
		if license != "" {
			log.Info("License retrieved from values.yaml")
			return license
		}

		secretName := s.Values.GetString("global.licenseSecret.name")
		secretKey := s.Values.GetString("global.licenseSecret.key")

		if secretName != "" && secretKey != "" {
			license := getLicenseFromSecret(kubeClient, secretName, secretKey)
			if license != "" {
				log.Info("License retrieved from Kubernetes secret", "secretName", secretName)
				return license
			}
		}
	}
	return "" 
}

func createKubeClient() (client.Client, error) {
	kubeConfig := config.GetConfigOrDie()
	kubeClient, err := client.New(kubeConfig, client.Options{})
	if err != nil {
		return nil, err
	}
	return kubeClient, nil
}

func getLicenseFromSecret(kubeClient client.Client, secretName, secretKey string) string {
	log := ctrllog.Log.WithName("getLicenseFromSecret")
	secret := &corev1.Secret{}
	ctx := context.TODO()

	err := kubeClient.Get(ctx, client.ObjectKey{Name: secretName, Namespace: "default"}, secret)
	if err != nil {
		log.Error(err, "Error retrieving secret", "secretName", secretName)
		return ""
	}

	if secret.Data == nil {
		log.Info("Secret has no data", "secretName", secretName)
		return ""
	}
	license, exists := secret.Data[secretKey]
	if !exists {
		log.Info("Key not found in secret", "secretKey", secretKey, "secretName", secretName)
		return ""
	}
	log.Info("Successfully retrieved license from secret", "secretName", secretName)
	return string(license)
}
