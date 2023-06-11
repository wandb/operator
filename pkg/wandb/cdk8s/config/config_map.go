package config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wandb/operator/pkg/wandb/cdk8s/release"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func CreateWithConfigMap(
	ctx context.Context,
	client client.Client,
	owner metav1.Object,
	scheme *runtime.Scheme,

	configMapName string,
	configMapNamespace string,
	release release.Release,
	config interface{},
) (*Config, error) {
	configJson, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: configMapNamespace,
			Labels: map[string]string{
				"wandb.ai/release/version": release.Version(),
			},
		},
		Data: map[string]string{
			"release": release.Version(),
			"config":  string(configJson),
		},
	}
	if err := controllerutil.SetControllerReference(owner, configMap, scheme); err != nil {
		return nil, err
	}
	if err := client.Create(ctx, configMap); err != nil {
		return nil, err
	}
	return &Config{Release: release, Config: config}, nil
}

func GetFromConfigMap(
	ctx context.Context,
	c client.Client,
	configMapName string,
	configMapNamespace string,
) (*Config, error) {
	configMap := &corev1.ConfigMap{}
	objKey := client.ObjectKey{Name: configMapName, Namespace: configMapNamespace}

	err := c.Get(ctx, objKey, configMap)
	if err != nil {
		return nil, err
	}

	relV, ok := configMap.Data["release"]
	if !ok {
		return nil, fmt.Errorf(
			"config map %s/%s does not have a `release` key", configMapName, configMapNamespace)
	}
	rel := release.NewLocalRelease(relV)

	configString, ok := configMap.Data["config"]
	if !ok {
		return nil, fmt.Errorf(
			"config map %s/%s does not have a `config` key", configMapName, configMapNamespace)
	}

	var config interface{}
	json.Unmarshal([]byte(configString), &config)

	return &Config{Release: rel, Config: config}, nil
}

// WroteToConfigMap is a helper function to write a config to a config map
func WriteToConfigMap(
	ctx context.Context,
	client client.Client,
	owner metav1.Object,
	scheme *runtime.Scheme,
	configMapName string,
	configMapNamespace string,
	config *Config,
) error {
	_, err := CreateWithConfigMap(
		ctx,
		client,
		owner,
		scheme,
		configMapName,
		configMapName,
		config.Release,
		config.Config,
	)
	return err
}
