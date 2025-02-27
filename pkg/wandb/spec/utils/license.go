package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/wandb/operator/pkg/wandb/spec"
)

func GetLicense(specs ...*spec.Spec) string {
	log := ctrllog.Log.WithName("GetLicense")

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
				log.Info("License retrieved from Kubernetes secret")
				return license
			}
		}
	}
	return ""
}

func createKubeClient() (client.Client, error) {
	kubeConfig, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	if kubeConfig.Host == "" {
		return nil, fmt.Errorf("invalid kubernetes configuration: empty host")
	}

	kubeClient, err := client.New(kubeConfig, client.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		return nil, err
	}
	return kubeClient, nil
}

func getLicenseFromSecret(kubeClient client.Client, secretName, secretKey, secretNamespace string) string {
	log := ctrllog.Log.WithName("getLicenseFromSecret")
	secret := &corev1.Secret{}

	err := kubeClient.Get(context.Background(), client.ObjectKey{Name: secretName, Namespace: secretNamespace}, secret)
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
